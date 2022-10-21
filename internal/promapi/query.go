package promapi

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-json-experiment/json"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

type QueryResponse struct {
	Status    string `json:"status"`
	Error     string `json:"error"`
	ErrorType string `json:"errorType"`
	Data      struct {
		ResultType string         `json:"resultType"`
		Result     []model.Sample `json:"result"`
	} `json:"data"`
}

type QueryResult struct {
	URI    string
	Series []model.Sample
}

type instantQuery struct {
	prom      *Prometheus
	ctx       context.Context
	expr      string
	timestamp time.Time
}

func (q instantQuery) Run() queryResult {
	log.Debug().
		Str("uri", q.prom.uri).
		Str("query", q.expr).
		Msg("Running prometheus query")

	ctx, cancel := context.WithTimeout(q.ctx, q.prom.timeout)
	defer cancel()

	qr := queryResult{expires: q.timestamp.Add(cacheExpiry * 2)}

	args := url.Values{}
	args.Set("query", q.expr)
	args.Set("timeout", q.prom.timeout.String())
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

	var decoded QueryResponse
	err = json.UnmarshalFull(resp.Body, &decoded)
	if err != nil {
		qr.err = APIError{Status: decoded.Status, ErrorType: decodeErrorType(decoded.ErrorType), Err: decoded.Error}
		return qr
	}

	if decoded.Status != promAPIStatusSuccess {
		qr.err = APIError{Status: decoded.Status, ErrorType: decodeErrorType(decoded.ErrorType), Err: decoded.Error}
		return qr
	}

	if decoded.Data.ResultType != "vector" {
		qr.err = APIError{Status: decoded.Status, ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("invalid result type, expected vector, got %s", decoded.Data.ResultType)}
		return qr
	}

	qr.value = decoded.Data.Result
	return qr
}

func (q instantQuery) Endpoint() string {
	return "/api/v1/query"
}

func (q instantQuery) String() string {
	return q.expr
}

func (q instantQuery) CacheKey() string {
	h := sha1.New()
	_, _ = io.WriteString(h, q.Endpoint())
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, q.expr)
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, q.timestamp.Round(cacheExpiry).Format(time.RFC3339))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Prometheus) Query(ctx context.Context, expr string) (*QueryResult, error) {
	log.Debug().Str("uri", p.uri).Str("query", expr).Msg("Scheduling prometheus query")

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
		URI:    p.uri,
		Series: result.value.([]model.Sample),
	}
	log.Debug().Str("uri", p.uri).Str("query", expr).Int("series", len(qr.Series)).Msg("Parsed response")

	return &qr, nil
}
