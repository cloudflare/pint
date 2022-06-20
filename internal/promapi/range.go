package promapi

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
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
	/*
		Too noisy
			log.Debug().
				Str("uri", q.prom.uri).
				Str("query", q.expr).
				Str("start", q.r.Start.Format(time.RFC3339)).
				Str("end", q.r.End.Format(time.RFC3339)).
				Str("step", output.HumanizeDuration(q.r.Step)).
				Msg("Running prometheus range query slice")
	*/

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

type RangeQueryTimes interface {
	Start() time.Time
	End() time.Time
	Dur() time.Duration
	Step() time.Duration
	String() string
}

func (p *Prometheus) RangeQuery(ctx context.Context, expr string, params RangeQueryTimes) (*RangeQueryResult, error) {
	start := params.Start()
	end := params.End()
	lookback := params.Dur()
	step := params.Step()

	queryStep := (time.Hour * 2).Round(step)
	if queryStep > lookback {
		queryStep = lookback
	}

	log.Debug().
		Str("uri", p.uri).
		Str("query", expr).
		Str("lookback", output.HumanizeDuration(lookback)).
		Str("step", output.HumanizeDuration(step)).
		Str("slice", output.HumanizeDuration(queryStep)).
		Msg("Scheduling prometheus range query")

	h := sha1.New()
	_, _ = io.WriteString(h, expr)
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, params.String())
	cacheKey := fmt.Sprintf("%x", h.Sum(nil))

	if cached, ok := p.cache.Get(cacheKey); ok {
		prometheusCacheHitsTotal.WithLabelValues(p.name, "/api/v1/query/range").Inc()
		log.Debug().
			Str("uri", p.uri).
			Str("query", expr).
			Msg("Cache hit")
		res := cached.(RangeQueryResult)
		return &res, nil
	}

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

	p.cache.Add(cacheKey, res)

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

func NewRelativeRange(lookback, step time.Duration) RelativeRange {
	return RelativeRange{lookback: lookback, step: step}
}

type RelativeRange struct {
	lookback time.Duration
	step     time.Duration
}

func (rr RelativeRange) Start() time.Time {
	return time.Now().Add(rr.lookback * -1)
}

func (rr RelativeRange) End() time.Time {
	return time.Now()
}

func (rr RelativeRange) Dur() time.Duration {
	return rr.lookback
}

func (rr RelativeRange) Step() time.Duration {
	return rr.step
}

func (rr RelativeRange) String() string {
	return fmt.Sprintf("%s/%s", output.HumanizeDuration(rr.lookback), output.HumanizeDuration(rr.step))
}

func NewAbsoluteRange(start, end time.Time, step time.Duration) AbsoluteRange {
	return AbsoluteRange{start: start, end: end, step: step}
}

type AbsoluteRange struct {
	start time.Time
	end   time.Time
	step  time.Duration
}

func (ar AbsoluteRange) Start() time.Time {
	return ar.start
}

func (ar AbsoluteRange) End() time.Time {
	return ar.end
}

func (ar AbsoluteRange) Dur() time.Duration {
	return ar.end.Sub(ar.start)
}

func (ar AbsoluteRange) Step() time.Duration {
	return ar.step
}

func (ar AbsoluteRange) String() string {
	return fmt.Sprintf(
		"%s-%s/%s",
		ar.start.Format(time.RFC3339),
		ar.end.Format(time.RFC3339),
		output.HumanizeDuration(ar.step))
}
