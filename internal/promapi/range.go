package promapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/go-json-experiment/json"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/cloudflare/pint/internal/output"
)

const (
	APIPathQueryRange = "/api/v1/query_range"
)

type RangeQueryResult struct {
	URI    string
	Series SeriesTimeRanges
	Stats  QueryStats
}

type PrometheusRangeQuerySample struct {
	Labels map[string]string      `json:"metric"`
	Values []SampleTimestampValue `json:"values"`
}

type PrometheusRangeQueryResponse struct {
	PrometheusResponse
	Data struct {
		ResultType string                       `json:"resultType"`
		Result     []PrometheusRangeQuerySample `json:"result"`
		Stats      QueryStats                   `json:"stats"`
	} `json:"data"`
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

	var qr queryResult

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
	ranges, qr.stats, qr.err = parseMatrixSamples(resp.Body, q.r.Step)
	ExpandRangesEnd(ranges, q.r.Step)
	qr.value = ranges
	return qr
}

func (q rangeQuery) Endpoint() string {
	return APIPathQueryRange
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

func (prom *Prometheus) RangeQuery(ctx context.Context, expr string, params RangeQueryTimes) (*RangeQueryResult, error) {
	start := params.Start()
	end := params.End()
	lookback := params.Dur()
	step := params.Step()

	var timeSlices []TimeRange
	queryStep := (time.Hour * 2).Round(step)
	if queryStep > lookback {
		queryStep = lookback
		timeSlices = append(timeSlices, TimeRange{Start: start, End: end})
	} else {
		timeSlices = sliceRange(start, end, step, queryStep)
	}

	slog.LogAttrs(ctx, slog.LevelDebug, "Scheduling prometheus range query",
		slog.String("uri", prom.safeURI),
		slog.String("query", expr),
		slog.String("lookback", output.HumanizeDuration(lookback)),
		slog.String("step", output.HumanizeDuration(step)),
		slog.String("slice", output.HumanizeDuration(queryStep)),
		slog.Int("slices", len(timeSlices)),
	)

	key := APIPathQueryRange + "\n" + expr + "\n" + params.String()
	prom.locker.lock(key)
	defer prom.locker.unlock(key)

	var wg sync.WaitGroup
	var lastErr error

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan queryResult, len(timeSlices))
	for _, s := range timeSlices {
		query := queryRequest{ // nolint: exhaustruct
			query: rangeQuery{
				prom: prom,
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
			prom.queries <- query
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

	merged := RangeQueryResult{ // nolint: exhaustruct
		URI: prom.publicURI,
		Series: SeriesTimeRanges{ // nolint: exhaustruct
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

	slices.SortStableFunc(merged.Series.Ranges, CompareMetricTimeRanges)

	slog.LogAttrs(ctx, slog.LevelDebug,
		"Parsed range response",
		slog.String("uri", prom.safeURI),
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

	for rstart.Before(end) {
		s := TimeRange{Start: rstart, End: rstart.Add(sliceSize)}
		if s.End.After(end) {
			s.End = end
		}
		slices = append(slices, s)

		rstart = rstart.Add(sliceSize)
	}

	for i := range slices {
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
	return output.HumanizeDuration(rr.lookback) + "/" + output.HumanizeDuration(rr.step)
}

func parseMatrixSamples(r io.Reader, step time.Duration) (dst MetricTimeRanges, _ QueryStats, err error) {
	defer dummyReadAll(r)

	var data PrometheusRangeQueryResponse
	if err = json.UnmarshalRead(r, &data); err != nil {
		return dst, data.Data.Stats, APIError{Status: data.Status, ErrorType: v1.ErrBadResponse, Err: "JSON parse error: " + err.Error()}
	}

	if data.Status != "success" {
		if data.Error == "" {
			data.Error = "empty response object"
		}
		return dst, data.Data.Stats, APIError{Status: data.Status, ErrorType: decodeErrorType(data.ErrorType), Err: data.Error}
	}

	if data.Data.ResultType != "matrix" {
		return nil, data.Data.Stats, APIError{Status: data.Status, ErrorType: v1.ErrBadResponse, Err: "invalid result type, expected matrix, got " + data.Data.ResultType}
	}

	var sp model.SamplePair
	for _, s := range data.Data.Result {
		lset := labels.FromMap(s.Labels)
		values := make([]model.SamplePair, 0, len(s.Values))
		for _, val := range s.Values {
			sp.Timestamp = val.Timestamp
			sp.Value = model.SampleValue(val.Value)
			values = append(values, sp)
		}
		dst = AppendSampleToRanges(dst, lset, values, step)
	}

	return dst, data.Data.Stats, nil
}
