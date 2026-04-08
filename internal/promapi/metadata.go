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
	APIPathMetadata = "/api/v1/metadata"
)

type PrometheusMetadataResponse struct {
	Data map[string][]v1.Metadata `json:"data"`
	PrometheusResponse
}

type MetadataResult struct {
	URI      string
	Metadata []v1.Metadata
}

type metadataQuery struct {
	timestamp time.Time
	ctx       context.Context
	prom      *Prometheus
	metric    string
}

func (q metadataQuery) Run() queryResult {
	slog.LogAttrs(q.ctx, slog.LevelDebug,
		"Getting prometheus metrics metadata",
		slog.String("uri", q.prom.safeURI),
		slog.String("metric", q.metric),
	)

	ctx, cancel := q.prom.requestContext(q.ctx)
	defer cancel()

	var qr queryResult

	args := url.Values{}
	args.Set("metric", q.metric)
	resp, err := q.prom.doRequest(ctx, http.MethodGet, q.Endpoint(), args)
	if err != nil {
		qr.err = fmt.Errorf("failed to query Prometheus metrics metadata: %w", err)
		return qr
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		qr.err = tryDecodingAPIError(resp)
		return qr
	}

	meta, err := parseMetadata(resp.Body)
	qr.value, qr.err = meta, err
	return qr
}

func (q metadataQuery) Endpoint() string {
	return APIPathMetadata
}

func (q metadataQuery) String() string {
	return q.metric
}

func (q metadataQuery) CacheKey() uint64 {
	return hash(q.prom.unsafeURI, q.Endpoint(), q.metric)
}

func (q metadataQuery) CacheTTL() time.Duration {
	return time.Minute * 10
}

func (prom *Prometheus) Metadata(ctx context.Context, metric string) (*MetadataResult, error) {
	slog.LogAttrs(ctx, slog.LevelDebug, "Scheduling Prometheus metrics metadata query", slog.String("uri", prom.safeURI), slog.String("metric", metric))

	key := APIPathMetadata + metric
	prom.locker.lock(key)
	defer prom.locker.unlock(key)

	resultChan := make(chan queryResult)
	prom.queries <- queryRequest{
		query:  metadataQuery{prom: prom, ctx: ctx, metric: metric, timestamp: time.Now()},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	metadata := MetadataResult{
		URI:      prom.publicURI,
		Metadata: result.value.(map[string][]v1.Metadata)[metric],
	}

	return &metadata, nil
}

func parseMetadata(r io.Reader) (meta map[string][]v1.Metadata, err error) {
	defer dummyReadAll(r)

	var data PrometheusMetadataResponse
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
