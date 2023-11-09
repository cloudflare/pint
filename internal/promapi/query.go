package promapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prymitive/current"
)

type QueryResult struct {
	URI       string
	PublicURI string
	Series    []Sample
	Stats     QueryStats
}

type instantQuery struct {
	prom      *Prometheus
	ctx       context.Context
	expr      string
	timestamp time.Time
}

func (q instantQuery) Run() queryResult {
	slog.Debug(
		"Running prometheus query",
		slog.String("uri", q.prom.safeURI),
		slog.String("query", q.expr),
	)

	ctx, cancel := q.prom.requestContext(q.ctx)
	defer cancel()

	var qr queryResult

	args := url.Values{}
	args.Set("query", q.expr)
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

	qr.value, qr.stats, qr.err = streamSamples(resp.Body)
	return qr
}

func (q instantQuery) Endpoint() string {
	return "/api/v1/query"
}

func (q instantQuery) String() string {
	return q.expr
}

func (q instantQuery) CacheKey() uint64 {
	return hash(q.prom.unsafeURI, q.Endpoint(), q.expr)
}

func (q instantQuery) CacheTTL() time.Duration {
	return time.Minute * 5
}

func (p *Prometheus) Query(ctx context.Context, expr string) (*QueryResult, error) {
	slog.Debug("Scheduling prometheus query", slog.String("uri", p.safeURI), slog.String("query", expr))

	key := fmt.Sprintf("/api/v1/query/%s", expr)
	p.locker.lock(key)
	defer p.locker.unlock(key)

	resultChan := make(chan queryResult)
	p.queries <- queryRequest{
		query:  instantQuery{prom: p, ctx: ctx, expr: expr, timestamp: time.Now()},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	qr := QueryResult{
		URI:       p.safeURI,
		PublicURI: p.publicURI,
		Series:    result.value.([]Sample),
		Stats:     result.stats,
	}
	slog.Debug("Parsed response", slog.String("uri", p.safeURI), slog.String("query", expr), slog.Int("series", len(qr.Series)))

	return &qr, nil
}

type Sample struct {
	Labels labels.Labels
	Value  float64
}

func streamSamples(r io.Reader) (samples []Sample, stats QueryStats, err error) {
	defer dummyReadAll(r)

	var status, resultType, errType, errText string
	samples = []Sample{}
	var sample model.Sample
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
					samples = append(samples, Sample{
						Labels: MetricToLabels(sample.Metric),
						Value:  float64(sample.Value),
					})
					sample.Metric = model.Metric{}
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

	if resultType != "vector" {
		return nil, stats, APIError{Status: status, ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("invalid result type, expected vector, got %s", resultType)}
	}

	return samples, stats, nil
}
