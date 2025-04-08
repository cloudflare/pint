package promapi

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/klauspost/compress/gzhttp"
	"go.uber.org/ratelimit"
)

var ErrUnsupported = errors.New("unsupported API")

type PrometheusContextKey string

const (
	AllPrometheusServers = PrometheusContextKey("allServers")
)

type QueryError struct {
	err error
	msg string
}

func (qe QueryError) Error() string {
	return qe.msg
}

func (qe QueryError) Unwrap() error {
	return qe.err
}

type querier interface {
	Endpoint() string
	String() string
	CacheKey() uint64
	CacheTTL() time.Duration
	Run() queryResult
}

type queryRequest struct {
	query  querier
	result chan queryResult
}

type queryResult struct {
	value any
	err   error
	stats QueryStats
}

type QueryTimings struct {
	EvalTotalTime        float64 `json:"evalTotalTime"`
	ResultSortTime       float64 `json:"resultSortTime"`
	QueryPreparationTime float64 `json:"queryPreparationTime"`
	InnerEvalTime        float64 `json:"innerEvalTime"`
	ExecQueueTime        float64 `json:"execQueueTime"`
	ExecTotalTime        float64 `json:"execTotalTime"`
}

type QuerySamples struct {
	TotalQueryableSamples int `json:"totalQueryableSamples"`
	PeakSamples           int `json:"peakSamples"`
}

type QueryStats struct {
	Timings QueryTimings `json:"timings"`
	Samples QuerySamples `json:"samples"`
}

func sanitizeURI(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return s
	}
	if u.User != nil {
		if _, pwdSet := u.User.Password(); pwdSet {
			u.User = url.UserPassword(u.User.Username(), "xxx")
		}
		return u.String()
	}
	return s
}

type unsupporedAPIs struct {
	mtx        sync.RWMutex
	noConfig   bool
	noFlags    bool
	noMetadata bool
}

func (ua *unsupporedAPIs) isSupported(s string) bool {
	ua.mtx.RLock()
	defer ua.mtx.RUnlock()
	switch s {
	case APIPathConfig:
		return !ua.noConfig
	case APIPathFlags:
		return !ua.noFlags
	case APIPathMetadata:
		return !ua.noMetadata
	default:
		return true
	}
}

func (ua *unsupporedAPIs) disable(s string) {
	slog.Debug("Disabling unsupported API", slog.String("api", s))
	ua.mtx.Lock()
	defer ua.mtx.Unlock()
	switch s {
	case APIPathConfig:
		ua.noConfig = true
	case APIPathFlags:
		ua.noFlags = true
	case APIPathMetadata:
		ua.noMetadata = true
	}
}

type Prometheus struct {
	rateLimiter ratelimit.Limiter
	headers     map[string]string
	cache       *queryCache
	locker      *partitionLocker
	apis        *unsupporedAPIs
	queries     chan queryRequest
	client      http.Client
	name        string
	unsafeURI   string // raw prometheus URI, for queries
	safeURI     string // prometheus URI but with auth info stripped, for logging
	publicURI   string // either set explicitly by user in the config or same as safeURI, this ends up as URI in query responses
	wg          sync.WaitGroup
	timeout     time.Duration
	concurrency int
}

func NewPrometheus(
	name, uri, publicURI string,
	headers map[string]string,
	timeout time.Duration,
	concurrency, rl int,
	tlsConf *tls.Config,
) *Prometheus {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if tlsConf != nil {
		transport.TLSClientConfig = tlsConf
	}

	unsafeURI := strings.TrimSuffix(uri, "/")
	safeURI := sanitizeURI(unsafeURI)
	publicURI = strings.TrimSuffix(publicURI, "/")
	if publicURI == "" {
		publicURI = safeURI
	}

	prom := Prometheus{ // nolint: exhaustruct
		name:        name,
		unsafeURI:   uri,
		safeURI:     safeURI,
		publicURI:   publicURI,
		headers:     headers,
		timeout:     timeout,
		client:      http.Client{Transport: gzhttp.Transport(transport)},
		locker:      newPartitionLocker((&sync.Mutex{})),
		rateLimiter: ratelimit.New(rl),
		concurrency: concurrency,
		apis:        &unsupporedAPIs{}, // nolint: exhaustruct
	}

	return &prom
}

func (prom *Prometheus) SafeURI() string {
	return prom.safeURI
}

func (prom *Prometheus) Close() {
	slog.Debug(
		"Stopping query workers",
		slog.String("name", prom.name),
		slog.String("uri", prom.safeURI),
	)
	close(prom.queries)
	prom.wg.Wait()
}

func (prom *Prometheus) StartWorkers() {
	slog.Debug(
		"Starting query workers",
		slog.String("name", prom.name),
		slog.String("uri", prom.safeURI),
		slog.Int("workers", prom.concurrency),
	)
	prom.queries = make(chan queryRequest, prom.concurrency*10)

	for w := 1; w <= prom.concurrency; w++ {
		prom.wg.Add(1)
		go func() {
			defer prom.wg.Done()
			queryWorker(prom, prom.queries)
		}()
	}
}

func (prom *Prometheus) doRequest(
	ctx context.Context,
	method, path string,
	args url.Values,
) (*http.Response, error) {
	u, _ := url.Parse(prom.unsafeURI)
	u.Path = strings.TrimSuffix(u.Path, "/")

	uri, err := url.JoinPath(u.String(), path)
	if err != nil {
		return nil, err
	}

	var body io.Reader
	if method == http.MethodPost {
		body = strings.NewReader(args.Encode())
	} else if eargs := args.Encode(); eargs != "" {
		uri += "?" + eargs
	}

	req, err := http.NewRequestWithContext(ctx, method, uri, body)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	for k, v := range prom.headers {
		req.Header.Set(k, v)
	}

	return prom.client.Do(req)
}

func (prom *Prometheus) requestContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, prom.timeout+time.Second)
}

func queryWorker(prom *Prometheus, queries chan queryRequest) {
	for job := range queries {
		job.result <- processJob(prom, job)
	}
}

func processJob(prom *Prometheus, job queryRequest) queryResult {
	cacheKey := job.query.CacheKey()
	if prom.cache != nil {
		if cached, ok := prom.cache.get(cacheKey, job.query.Endpoint()); ok {
			return cached.(queryResult)
		}
	}

	if !prom.apis.isSupported(job.query.Endpoint()) {
		return queryResult{err: ErrUnsupported} // nolint: exhaustruct
	}

	prometheusQueriesTotal.WithLabelValues(prom.name, job.query.Endpoint()).Inc()
	prometheusQueriesRunning.WithLabelValues(prom.name, job.query.Endpoint()).Inc()

	prom.rateLimiter.Take()
	result := job.query.Run()
	prometheusQueriesRunning.WithLabelValues(prom.name, job.query.Endpoint()).Dec()

	if result.err != nil {
		if errors.Is(result.err, context.Canceled) {
			return result
		}
		prometheusQueryErrorsTotal.WithLabelValues(prom.name, job.query.Endpoint(), errReason(result.err)).
			Inc()
		if isUnsupportedError(result.err) {
			prom.apis.disable(job.query.Endpoint())
			slog.Warn(
				"Looks like this server doesn't support some Prometheus API endpoints, all checks using this API will be disabled",
				slog.String("name", prom.name),
				slog.String("uri", prom.safeURI),
				slog.String("api", job.query.Endpoint()),
			)
			return queryResult{err: ErrUnsupported} // nolint: exhaustruct
		}
		slog.Error(
			"Query returned an error",
			slog.Any("err", result.err),
			slog.String("uri", prom.safeURI),
			slog.String("query", job.query.String()),
		)
		return result
	}

	if prom.cache != nil {
		prom.cache.set(cacheKey, result, job.query.CacheTTL())
	}

	return result
}

func formatTime(t time.Time) string {
	return strconv.FormatFloat(float64(t.Unix())+float64(t.Nanosecond())/1e9, 'f', -1, 64)
}

func dummyReadAll(r io.Reader) {
	_, _ = io.Copy(io.Discard, r)
}

func hash(s ...string) uint64 {
	h := xxhash.New()
	for _, v := range s {
		_, _ = h.WriteString(v)
		_, _ = h.WriteString("\n")
	}
	return h.Sum64()
}
