package promapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

const (
	APIPathFlags = "/api/v1/status/flags"
)

type PrometheusFlagsResponse struct {
	Data v1.FlagsResult `json:"data"`
	PrometheusResponse
}

type FlagsResult struct {
	Flags v1.FlagsResult
	URI   string
}

type flagsQuery struct {
	prom      *Prometheus
	ctx       context.Context
	timestamp time.Time
}

func (q flagsQuery) Run() queryResult {
	slog.LogAttrs(q.ctx, slog.LevelDebug, "Getting prometheus flags", slog.String("uri", q.prom.safeURI))

	ctx, cancel := q.prom.requestContext(q.ctx)
	defer cancel()

	var qr queryResult

	args := url.Values{}
	resp, err := q.prom.doRequest(ctx, http.MethodGet, q.Endpoint(), args)
	if err != nil {
		qr.err = fmt.Errorf("failed to query Prometheus flags: %w", err)
		return qr
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		qr.err = tryDecodingAPIError(resp)
		return qr
	}

	flags, err := parseFlags(resp.Body)
	qr.value, qr.err = flags, err
	return qr
}

func (q flagsQuery) Endpoint() string {
	return APIPathFlags
}

func (q flagsQuery) String() string {
	return APIPathFlags
}

func (q flagsQuery) CacheKey() uint64 {
	return hash(q.prom.unsafeURI, q.Endpoint())
}

func (q flagsQuery) CacheTTL() time.Duration {
	return time.Minute * 10
}

func (prom *Prometheus) Flags(ctx context.Context) (*FlagsResult, error) {
	slog.LogAttrs(ctx, slog.LevelDebug, "Scheduling Prometheus flags query", slog.String("uri", prom.safeURI))

	prom.locker.lock(APIPathFlags)
	defer prom.locker.unlock(APIPathFlags)

	resultChan := make(chan queryResult)
	prom.queries <- queryRequest{
		query:  flagsQuery{prom: prom, ctx: ctx, timestamp: time.Now()},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	r := FlagsResult{
		URI:   prom.publicURI,
		Flags: result.value.(v1.FlagsResult),
	}

	return &r, nil
}

func parseFlags(r io.Reader) (_ v1.FlagsResult, err error) {
	defer dummyReadAll(r)

	var data PrometheusFlagsResponse
	if err = json.NewDecoder(r).Decode(&data); err != nil {
		return data.Data, APIError{Status: data.Status, ErrorType: v1.ErrBadResponse, Err: "JSON parse error: " + err.Error()}
	}

	if data.Status != "success" {
		if data.Error == "" {
			data.Error = "empty response object"
		}
		return data.Data, APIError{Status: data.Status, ErrorType: decodeErrorType(data.ErrorType), Err: data.Error}
	}

	return data.Data, nil
}
