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
	"github.com/rs/zerolog/log"
)

type FlagsResponse struct {
	Status    string         `json:"status"`
	Error     string         `json:"error"`
	ErrorType string         `json:"errorType"`
	Data      v1.FlagsResult `json:"data"`
}

type FlagsResult struct {
	URI   string
	Flags v1.FlagsResult
}

type flagsQuery struct {
	prom      *Prometheus
	ctx       context.Context
	timestamp time.Time
}

func (q flagsQuery) Run() queryResult {
	log.Debug().
		Str("uri", q.prom.uri).
		Msg("Getting prometheus flags")

	ctx, cancel := context.WithTimeout(q.ctx, q.prom.timeout)
	defer cancel()

	qr := queryResult{expires: q.timestamp.Add(cacheExpiry * 2)}

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

	var decoded FlagsResponse
	err = json.UnmarshalFull(resp.Body, &decoded)
	if err != nil {
		qr.err = APIError{Status: decoded.Status, ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("JSON parse error: %s", err)}
		return qr
	}

	if decoded.Status != promAPIStatusSuccess {
		qr.err = APIError{Status: decoded.Status, ErrorType: decodeErrorType(decoded.ErrorType), Err: decoded.Error}
		return qr
	}

	qr.value = decoded.Data
	return qr
}

func (q flagsQuery) Endpoint() string {
	return "/api/v1/status/flags"
}

func (q flagsQuery) String() string {
	return "/api/v1/status/flags"
}

func (q flagsQuery) CacheKey() string {
	h := sha1.New()
	_, _ = io.WriteString(h, q.Endpoint())
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, q.timestamp.Round(cacheExpiry).Format(time.RFC3339))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Prometheus) Flags(ctx context.Context) (*FlagsResult, error) {
	log.Debug().Str("uri", p.uri).Msg("Scheduling Prometheus flags query")

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

	r := FlagsResult{URI: p.uri, Flags: result.value.(v1.FlagsResult)}

	return &r, nil
}
