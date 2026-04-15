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
	APIPathBuildInfo = "/api/v1/status/buildinfo"
)

type PrometheusBuildInfoResponse struct {
	Data v1.BuildinfoResult `json:"data"`
	PrometheusResponse
}

type BuildInfoResult struct {
	URI     string
	Version string
}

type buildInfoQuery struct {
	prom      *Prometheus
	ctx       context.Context
	timestamp time.Time
}

func (q buildInfoQuery) Run() queryResult {
	slog.LogAttrs(q.ctx, slog.LevelDebug, "Getting prometheus build info", slog.String("uri", q.prom.safeURI))

	ctx, cancel := q.prom.requestContext(q.ctx)
	defer cancel()

	var qr queryResult

	args := url.Values{}
	resp, err := q.prom.doRequest(ctx, http.MethodGet, q.Endpoint(), args)
	if err != nil {
		qr.err = fmt.Errorf("failed to query Prometheus build info: %w", err)
		return qr
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		qr.err = tryDecodingAPIError(resp)
		return qr
	}

	bi, err := parseBuildInfo(resp.Body)
	qr.value, qr.err = bi, err
	return qr
}

func (q buildInfoQuery) Endpoint() string {
	return APIPathBuildInfo
}

func (q buildInfoQuery) String() string {
	return APIPathBuildInfo
}

func (q buildInfoQuery) CacheKey() uint64 {
	return hash(q.prom.unsafeURI, q.Endpoint())
}

func (q buildInfoQuery) CacheTTL() time.Duration {
	return time.Minute * 10
}

func (prom *Prometheus) BuildInfo(ctx context.Context) (*BuildInfoResult, error) {
	slog.LogAttrs(ctx, slog.LevelDebug, "Scheduling Prometheus build info query", slog.String("uri", prom.safeURI))

	prom.locker.lock(APIPathBuildInfo)
	defer prom.locker.unlock(APIPathBuildInfo)

	resultChan := make(chan queryResult)
	prom.queries <- queryRequest{
		query:  buildInfoQuery{prom: prom, ctx: ctx, timestamp: time.Now()},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	bi := result.value.(v1.BuildinfoResult)
	r := BuildInfoResult{
		URI:     prom.publicURI,
		Version: bi.Version,
	}

	return &r, nil
}

func parseBuildInfo(r io.Reader) (_ v1.BuildinfoResult, err error) {
	defer dummyReadAll(r)

	var data PrometheusBuildInfoResponse
	if err = json.NewDecoder(r).Decode(&data); err != nil {
		return data.Data, APIError{
			Status:    data.Status,
			ErrorType: v1.ErrBadResponse,
			Err:       "JSON parse error: " + err.Error(),
		}
	}

	if data.Status != "success" {
		if data.Error == "" {
			data.Error = "empty response object"
		}
		return data.Data, APIError{
			Status:    data.Status,
			ErrorType: decodeErrorType(data.ErrorType),
			Err:       data.Error,
		}
	}

	return data.Data, nil
}
