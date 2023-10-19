package promapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/prymitive/current"

	"github.com/cloudflare/pint/internal/output"
)

type RangeQueryResult struct {
	URI    string
	Series SeriesTimeRanges
	Stats  QueryStats
}

type rangeQuery struct {
	prom *Prometheus
	ctx  context.Context
	expr string
	r    v1.Range
	ttl  time.Duration
}

func (q rangeQuery) Run() queryResult {
	ctx, cancel := q.prom.requestContext(q.ctx)
	defer cancel()

	qr := queryResult{}

	args := url.Values{}
	args.Set("query", q.expr)
	args.Set("start", formatTime(q.r.Start))
	args.Set("end", formatTime(q.r.End))
	args.Set("step", strconv.FormatFloat(q.r.Step.Seconds(), 'f', -1, 64))
	args.Set("timeout", q.prom.timeout.String())
	args.Set("stats", "1")
	resp, err := q.prom.doRequest(ctx, http.MethodPost, q.Endpoint(), args)
	if err != nil {
		qr.err = err
		return qr
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		qr.err = tryDecodingAPIError(resp)
		return qr
	}

	var ranges MetricTimeRanges
	ranges, qr.stats, qr.err = streamSampleStream(resp.Body, q.r.Step)
	ExpandRangesEnd(ranges, q.r.Step)
	qr.value = ranges
	return qr
}

func (q rangeQuery) Endpoint() string {
	return "/api/v1/query_range"
}

func (q rangeQuery) String() string {
	return q.expr
}

func (q rangeQuery) CacheKey() uint64 {
	return hash(q.prom.unsafeURI, q.Endpoint(), q.expr, q.r.Start.Format(time.RFC3339), q.r.End.Round(q.r.Step).Format(time.RFC3339), output.HumanizeDuration(q.r.Step))
}

func (q rangeQuery) CacheTTL() time.Duration {
	return q.ttl
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

	var slices []TimeRange
	queryStep := (time.Hour * 2).Round(step)
	if queryStep > lookback {
		queryStep = lookback
		slices = append(slices, TimeRange{Start: start, End: end})
	} else {
		slices = sliceRange(start, end, step, queryStep)
	}

	slog.Debug("Scheduling prometheus range query",
		slog.String("uri", p.safeURI),
		slog.String("query", expr),
		slog.String("lookback", output.HumanizeDuration(lookback)),
		slog.String("step", output.HumanizeDuration(step)),
		slog.String("slice", output.HumanizeDuration(queryStep)),
		slog.Int("slices", len(slices)),
	)

	key := fmt.Sprintf("/api/v1/query_range/%s/%s", expr, params.String())
	p.locker.lock(key)
	defer p.locker.unlock(key)

	var wg sync.WaitGroup
	var lastErr error

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan queryResult, len(slices))
	for _, s := range slices {
		query := queryRequest{
			query: rangeQuery{
				prom: p,
				ctx:  ctx,
				expr: expr,
				r: v1.Range{
					Start: s.Start,
					End:   s.End,
					Step:  step,
				},
				ttl: s.End.Sub(start) + time.Minute*10,
			},
		}

		wg.Add(1)
		go func() {
			var result queryResult
			query.result = make(chan queryResult)
			p.queries <- query
			result = <-query.result
			results <- result

			if result.err != nil {
				cancel()
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	merged := RangeQueryResult{
		URI: p.safeURI,
		Series: SeriesTimeRanges{
			From:  start,
			Until: end,
			Step:  step,
		},
	}

	for result := range results {
		if result.err != nil {
			if !errors.Is(result.err, context.Canceled) {
				lastErr = result.err
			}
			wg.Done()
			continue
		}
		merged.Series.Ranges = append(merged.Series.Ranges, result.value.(MetricTimeRanges)...)
		merged.Stats.Samples.PeakSamples += result.stats.Samples.PeakSamples
		merged.Stats.Samples.TotalQueryableSamples += result.stats.Samples.TotalQueryableSamples
		merged.Stats.Timings.EvalTotalTime += result.stats.Timings.EvalTotalTime
		merged.Stats.Timings.ExecQueueTime += result.stats.Timings.ExecQueueTime
		merged.Stats.Timings.ExecTotalTime += result.stats.Timings.ExecTotalTime
		merged.Stats.Timings.InnerEvalTime += result.stats.Timings.InnerEvalTime
		merged.Stats.Timings.QueryPreparationTime += result.stats.Timings.QueryPreparationTime
		merged.Stats.Timings.ResultSortTime += result.stats.Timings.ResultSortTime
		wg.Done()
	}
	if len(merged.Series.Ranges) > 1 {
		merged.Series.Ranges, _ = MergeRanges(merged.Series.Ranges, step)
	}

	if lastErr != nil {
		return nil, QueryError{err: lastErr, msg: decodeError(lastErr)}
	}

	sort.Stable(merged.Series.Ranges)

	slog.Debug(
		"Parsed range response",
		slog.String("uri", p.safeURI),
		slog.String("query", expr),
		slog.Int("samples", len(merged.Series.Ranges)),
	)

	return &merged, nil
}

func sliceRange(start, end time.Time, resolution, sliceSize time.Duration) (slices []TimeRange) {
	if end.Sub(start) <= resolution {
		return []TimeRange{{Start: start, End: end}}
	}

	rstart := start.Round(sliceSize)

	if rstart.After(start) {
		s := TimeRange{Start: rstart.Add(sliceSize * -1), End: rstart}
		if s.End.After(end) {
			s.End = end
		}
		slices = append(slices, s)
	}

	for {
		if !rstart.Before(end) {
			break
		}

		s := TimeRange{Start: rstart, End: rstart.Add(sliceSize)}
		if s.End.After(end) {
			s.End = end
		}
		slices = append(slices, s)

		rstart = rstart.Add(sliceSize)
	}

	for i := 0; i < len(slices); i++ {
		if i < len(slices)-1 {
			slices[i].End = slices[i].End.Add(time.Second * -1)
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

func streamSampleStream(r io.Reader, step time.Duration) (dst MetricTimeRanges, stats QueryStats, err error) {
	defer dummyReadAll(r)

	var status, errType, errText, resultType string
	var sample model.SampleStream
	decoder := current.Object(
		current.Key("status", current.Value(func(s string, isNil bool) {
			status = s
		})),
		current.Key("error", current.Value(func(s string, isNil bool) {
			errText = s
		})),
		current.Key("errorType", current.Value(func(s string, isNil bool) {
			errType = s
		})),
		current.Key("data", current.Object(
			current.Key("resultType", current.Value(func(s string, isNil bool) {
				resultType = s
			})),
			current.Key("result", current.Array(
				&sample,
				func() {
					lset := MetricToLabels(sample.Metric)
					dst = AppendSampleToRanges(dst, lset, sample.Values, step)
					sample.Metric = model.Metric{}
					sample.Values = make([]model.SamplePair, 0, len(sample.Values))
				},
			)),
			current.Key("stats", current.Object(
				current.Key("timings", current.Object(
					current.Key("evalTotalTime", current.Value(func(v float64, isNil bool) {
						stats.Timings.EvalTotalTime = v
					})),
					current.Key("resultSortTime", current.Value(func(v float64, isNil bool) {
						stats.Timings.ResultSortTime = v
					})),
					current.Key("queryPreparationTime", current.Value(func(v float64, isNil bool) {
						stats.Timings.QueryPreparationTime = v
					})),
					current.Key("innerEvalTime", current.Value(func(v float64, isNil bool) {
						stats.Timings.InnerEvalTime = v
					})),
					current.Key("execQueueTime", current.Value(func(v float64, isNil bool) {
						stats.Timings.ExecQueueTime = v
					})),
					current.Key("execTotalTime", current.Value(func(v float64, isNil bool) {
						stats.Timings.ExecTotalTime = v
					})),
				)),
				current.Key("samples", current.Object(
					current.Key("totalQueryableSamples", current.Value(func(v float64, isNil bool) {
						stats.Samples.TotalQueryableSamples = int(math.Round(v))
					})),
					current.Key("peakSamples", current.Value(func(v float64, isNil bool) {
						stats.Samples.PeakSamples = int(math.Round(v))
					})),
				)),
			)),
		)),
	)

	dec := json.NewDecoder(r)
	if err = decoder.Stream(dec); err != nil {
		return nil, stats, APIError{Status: status, ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("JSON parse error: %s", err)}
	}

	if status != "success" {
		return nil, stats, APIError{Status: status, ErrorType: decodeErrorType(errType), Err: errText}
	}

	if resultType != "matrix" {
		return nil, stats, APIError{Status: status, ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("invalid result type, expected matrix, got %s", resultType)}
	}

	return dst, stats, nil
}
