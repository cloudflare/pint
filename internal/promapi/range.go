package promapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"

	"github.com/cloudflare/pint/internal/output"
)

type RangeQueryResult struct {
	URI             string
	Samples         []*model.SampleStream
	Start           time.Time
	End             time.Time
	DurationSeconds float64
}

type rangeQuery struct {
	prom *Prometheus
	ctx  context.Context
	expr string
	r    v1.Range
}

func (q rangeQuery) Run() (any, error) {
	ctx, cancel := context.WithTimeout(q.ctx, q.prom.timeout)
	defer cancel()

	v, _, err := q.prom.api.QueryRange(ctx, q.expr, q.r)
	return v, err
}

func (q rangeQuery) Endpoint() string {
	return "/api/v1/query/range"
}

func (q rangeQuery) String() string {
	return q.expr
}

func (q rangeQuery) CacheKey() string {
	return ""
}

func (p *Prometheus) RangeQuery(ctx context.Context, expr string, start, end time.Time, step time.Duration) (*RangeQueryResult, error) {
	lookback := end.Sub(start)

	queryStep := (time.Hour * 2).Round(step)
	if queryStep > lookback {
		queryStep = lookback
	}

	log.Debug().
		Str("uri", p.uri).
		Str("query", expr).
		Str("start", start.Format(time.RFC3339)).
		Str("end", end.Format(time.RFC3339)).
		Str("step", output.HumanizeDuration(step)).
		Str("slice", output.HumanizeDuration(queryStep)).
		Msg("Scheduling prometheus range query")

	var wg sync.WaitGroup
	var mu sync.Mutex
	var lastErr error

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	res := RangeQueryResult{URI: p.uri, Start: start, End: end}
	for _, s := range sliceRange(start, end, step, queryStep) {
		query := queryRequest{
			query: rangeQuery{
				prom: p,
				ctx:  ctx,
				expr: expr,
				r: v1.Range{
					Start: s.start,
					End:   s.end,
					Step:  step,
				},
			},
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			var result queryResult
			query.result = make(chan queryResult)
			p.queries <- query
			result = <-query.result

			mu.Lock()
			defer mu.Unlock()

			if result.err != nil {
				if !errors.Is(result.err, context.Canceled) {
					lastErr = result.err
				}
				cancel()
				return
			}

			switch result.value.(model.Value).Type() {
			case model.ValMatrix:
				for _, sample := range result.value.(model.Matrix) {
					var found bool
					for i, rs := range res.Samples {
						if sample.Metric.Equal(rs.Metric) {
							found = true
							res.Samples[i].Values = append(res.Samples[i].Values, sample.Values...)
							break
						}
					}
					if !found {
						res.Samples = append(res.Samples, sample)
					}
				}
			default:
				log.Error().Str("uri", p.uri).Str("query", expr).Msgf("Range query returned unknown result type: %v", result.value.(model.Value).Type())
				lastErr = fmt.Errorf("unknown result type: %v", result.value.(model.Value).Type())
				return
			}
		}()
	}
	wg.Wait()

	if lastErr != nil {
		return nil, QueryError{err: lastErr, msg: decodeError(lastErr)}
	}

	for k := range res.Samples {
		sort.SliceStable(res.Samples[k].Values, func(i, j int) bool {
			return res.Samples[k].Values[i].Timestamp.Before(res.Samples[k].Values[j].Timestamp)
		})
	}

	log.Debug().Str("uri", p.uri).Str("query", expr).Int("samples", len(res.Samples)).Msg("Parsed range response")

	return &res, nil
}

type timeRange struct {
	start time.Time
	end   time.Time
}

func sliceRange(start, end time.Time, resolution, sliceSize time.Duration) []timeRange {
	diff := end.Sub(start)
	if diff <= sliceSize {
		return []timeRange{{start: start, end: end}}
	}

	var slices []timeRange
	for {
		s := timeRange{start: start, end: start.Add(sliceSize)}
		if s.end.After(end) {
			s.end = end
		}
		slices = append(slices, s)
		start = start.Add(sliceSize)
		if !start.Before(end) {
			break
		}
	}

	for i := 0; i < len(slices); i++ {
		if i < len(slices)-1 {
			slices[i].end = slices[i].end.Add(resolution * -1)
		}
	}

	return slices
}
