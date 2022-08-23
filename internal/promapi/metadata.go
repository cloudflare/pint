package promapi

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prymitive/current"
	"github.com/rs/zerolog/log"
)

type MetadataResult struct {
	URI      string
	Metadata []v1.Metadata
}

type metadataQuery struct {
	prom      *Prometheus
	ctx       context.Context
	metric    string
	timestamp time.Time
}

func (q metadataQuery) Run() queryResult {
	log.Debug().
		Str("uri", q.prom.uri).
		Str("metric", q.metric).
		Msg("Getting prometheus metrics metadata")

	ctx, cancel := context.WithTimeout(q.ctx, q.prom.timeout)
	defer cancel()

	qr := queryResult{expires: q.timestamp.Add(cacheExpiry * 2)}

	args := url.Values{}
	args.Set("metric", q.metric)
	resp, err := q.prom.doRequest(ctx, http.MethodPost, "/api/v1/metadata", args)
	if err != nil {
		qr.err = fmt.Errorf("failed to query Prometheus metrics metadata: %w", err)
		return qr
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		qr.err = tryDecodingAPIError(resp)
		return qr
	}

	qr.value, qr.err = streamMetadata(resp.Body)
	return qr
}

func (q metadataQuery) Endpoint() string {
	return "/api/v1/metadata"
}

func (q metadataQuery) String() string {
	return q.metric
}

func (q metadataQuery) CacheKey() string {
	h := sha1.New()
	_, _ = io.WriteString(h, q.Endpoint())
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, q.metric)
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, q.timestamp.Round(cacheExpiry).Format(time.RFC3339))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Prometheus) Metadata(ctx context.Context, metric string) (*MetadataResult, error) {
	log.Debug().Str("uri", p.uri).Str("metric", metric).Msg("Scheduling Prometheus metrics metadata query")

	key := fmt.Sprintf("/api/v1/metadata/%s", metric)
	p.locker.lock(key)
	defer p.locker.unlock(key)

	resultChan := make(chan queryResult)
	p.queries <- queryRequest{
		query:  metadataQuery{prom: p, ctx: ctx, metric: metric, timestamp: time.Now()},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	metadata := MetadataResult{URI: p.uri, Metadata: result.value.(map[string][]v1.Metadata)[metric]}

	return &metadata, nil
}

func streamMetadata(r io.Reader) (meta map[string][]v1.Metadata, err error) {
	defer dummyReadAll(r)

	var status, errType, errText string
	meta = map[string][]v1.Metadata{}
	decoder := current.Object(
		func() {},
		current.Key("status", current.Text(func(s string) {
			status = s
		})),
		current.Key("error", current.Text(func(s string) {
			errText = s
		})),
		current.Key("errorType", current.Text(func(s string) {
			errType = s
		})),
		current.Key("data", current.Map(func(k string, v []v1.Metadata) {
			meta[k] = v
		})),
	)

	dec := json.NewDecoder(r)
	if err = current.Stream(dec, decoder); err != nil {
		return nil, APIError{Status: status, ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("JSON parse error: %s", err)}
	}

	if status != "success" {
		return nil, APIError{Status: status, ErrorType: decodeErrorType(errType), Err: errText}
	}

	return meta, nil
}
