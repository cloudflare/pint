package promapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

const (
	APIPathQuery = "/api/v1/query"
)

type QueryResult struct {
	URI    string
	Series []Sample
	Stats  QueryStats
}

type SampleLabels labels.Labels

func (s *SampleLabels) UnmarshalJSONFrom(dec *jsontext.Decoder) (err error) {
	var (
		tok jsontext.Token
		k   jsontext.Kind
	)

	if k = dec.PeekKind(); k != '{' {
		return &json.SemanticError{JSONKind: k} // nolint: exhaustruct
	}
	if _, err = dec.ReadToken(); err != nil {
		return err
	}

	var parts []string
	for dec.PeekKind() != '}' {
		if tok, err = dec.ReadToken(); err != nil {
			return err
		}
		parts = append(parts, tok.String())
	}
	*s = SampleLabels(labels.FromStrings(parts...))

	if _, err = dec.ReadToken(); err != nil {
		return err
	}
	return nil
}

type SampleTimestampValue struct {
	Timestamp model.Time
	Value     model.SampleValue
}

func (s *SampleTimestampValue) UnmarshalJSONFrom(dec *jsontext.Decoder) (err error) {
	var (
		tok jsontext.Token
		k   jsontext.Kind
		f   float64
	)

	if k = dec.PeekKind(); k != '[' {
		return &json.SemanticError{JSONKind: k} // nolint: exhaustruct
	}
	if _, err = dec.ReadToken(); err != nil {
		return err
	}

	tok, err = dec.ReadToken()
	if err != nil {
		return err
	}
	s.Timestamp = model.Time(tok.Int() * 1000)

	tok, err = dec.ReadToken()
	if err != nil {
		return err
	}
	f, err = strconv.ParseFloat(tok.String(), 64)
	if err == nil {
		s.Value = model.SampleValue(f)
	}

	if k = dec.PeekKind(); k != ']' {
		return &json.SemanticError{JSONKind: k} // nolint: exhaustruct
	}
	if _, err = dec.ReadToken(); err != nil {
		return err
	}
	return nil
}

type PrometheusQuerySample struct {
	Labels SampleLabels         `json:"metric"`
	Value  SampleTimestampValue `json:"value"`
}

type PrometheusQueryResponse struct {
	PrometheusResponse
	Data struct {
		ResultType string                  `json:"resultType"`
		Result     []PrometheusQuerySample `json:"result"`
		Stats      QueryStats              `json:"stats"`
	} `json:"data"`
}

type instantQuery struct {
	timestamp time.Time
	ctx       context.Context
	prom      *Prometheus
	expr      string
}

func (q instantQuery) Run() queryResult {
	slog.LogAttrs(q.ctx, slog.LevelDebug,
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

	qr.value, qr.stats, qr.err = parseVectorSamples(resp.Body)
	return qr
}

func (q instantQuery) Endpoint() string {
	return APIPathQuery
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

func (prom *Prometheus) Query(ctx context.Context, expr string) (*QueryResult, error) {
	slog.LogAttrs(ctx, slog.LevelDebug, "Scheduling prometheus query", slog.String("uri", prom.safeURI), slog.String("query", expr))

	key := APIPathQuery + expr
	prom.locker.lock(key)
	defer prom.locker.unlock(key)

	resultChan := make(chan queryResult)
	prom.queries <- queryRequest{
		query:  instantQuery{prom: prom, ctx: ctx, expr: expr, timestamp: time.Now()},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	qr := QueryResult{
		URI:    prom.publicURI,
		Series: result.value.([]Sample),
		Stats:  result.stats,
	}
	slog.LogAttrs(ctx, slog.LevelDebug, "Parsed response", slog.String("uri", prom.safeURI), slog.String("query", expr), slog.Int("series", len(qr.Series)))

	return &qr, nil
}

type Sample struct {
	Labels labels.Labels
	Value  float64
}

func parseVectorSamples(r io.Reader) (samples []Sample, _ QueryStats, err error) {
	defer dummyReadAll(r)

	var data PrometheusQueryResponse
	if err = json.UnmarshalRead(r, &data); err != nil {
		return samples, data.Data.Stats, APIError{Status: data.Status, ErrorType: v1.ErrBadResponse, Err: "JSON parse error: " + err.Error()}
	}

	if data.Status != "success" {
		if data.Error == "" {
			data.Error = "empty response object"
		}
		return samples, data.Data.Stats, APIError{Status: data.Status, ErrorType: decodeErrorType(data.ErrorType), Err: data.Error}
	}

	if data.Data.ResultType != "vector" {
		return nil, data.Data.Stats, APIError{Status: data.Status, ErrorType: v1.ErrBadResponse, Err: "invalid result type, expected vector, got " + data.Data.ResultType}
	}

	samples = make([]Sample, 0, len(data.Data.Result))
	for _, s := range data.Data.Result {
		samples = append(samples, Sample{
			Labels: labels.Labels(s.Labels),
			Value:  float64(s.Value.Value),
		})
	}

	return samples, data.Data.Stats, nil
}
