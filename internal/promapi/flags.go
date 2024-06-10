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
	"github.com/prymitive/current"
)

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
	slog.Debug("Getting prometheus flags", slog.String("uri", q.prom.safeURI))

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

	flags, err := streamFlags(resp.Body)
	qr.value, qr.err = flags, err
	return qr
}

func (q flagsQuery) Endpoint() string {
	return "/api/v1/status/flags"
}

func (q flagsQuery) String() string {
	return "/api/v1/status/flags"
}

func (q flagsQuery) CacheKey() uint64 {
	return hash(q.prom.unsafeURI, q.Endpoint())
}

func (q flagsQuery) CacheTTL() time.Duration {
	return time.Minute * 10
}

func (p *Prometheus) Flags(ctx context.Context) (*FlagsResult, error) {
	slog.Debug("Scheduling Prometheus flags query", slog.String("uri", p.safeURI))

	key := "/api/v1/status/flags"
	p.locker.lock(key)
	defer p.locker.unlock(key)

	resultChan := make(chan queryResult)
	p.queries <- queryRequest{
		query:  flagsQuery{prom: p, ctx: ctx, timestamp: time.Now()},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	r := FlagsResult{
		URI:   p.publicURI,
		Flags: result.value.(v1.FlagsResult),
	}

	return &r, nil
}

func streamFlags(r io.Reader) (flags v1.FlagsResult, err error) {
	defer dummyReadAll(r)

	var status, errType, errText string
	flags = v1.FlagsResult{}
	decoder := current.Object(
		current.Key("status", current.Value(func(s string, _ bool) {
			status = s
		})),
		current.Key("error", current.Value(func(s string, _ bool) {
			errText = s
		})),
		current.Key("errorType", current.Value(func(s string, _ bool) {
			errType = s
		})),
		current.Key("data", current.Map(func(k, v string) {
			flags[k] = v
		})),
	)

	dec := json.NewDecoder(r)
	if err = decoder.Stream(dec); err != nil {
		return nil, APIError{Status: status, ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("JSON parse error: %s", err)}
	}

	if status != "success" {
		return nil, APIError{Status: status, ErrorType: decodeErrorType(errType), Err: errText}
	}

	return flags, nil
}
