package promapi

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/klauspost/compress/gzhttp"
	"github.com/rs/zerolog/log"
	"go.uber.org/ratelimit"

	"github.com/cloudflare/pint/internal/output"
)

var cacheExpiry = time.Minute * 5

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
	CacheKey() string
	Run() queryResult
}

type queryRequest struct {
	query  querier
	result chan queryResult
}

type queryResult struct {
	value   any
	err     error
	expires time.Time
}

type Prometheus struct {
	name        string
	uri         string
	timeout     time.Duration
	concurrency int
	client      http.Client
	cache       *lru.ARCCache
	locker      *partitionLocker
	rateLimiter ratelimit.Limiter
	wg          sync.WaitGroup
	queries     chan queryRequest
}

func NewPrometheus(name, uri string, timeout time.Duration, concurrency, cacheSize, rl int) *Prometheus {
	cache, _ := lru.NewARC(cacheSize)

	prom := Prometheus{
		name:        name,
		uri:         uri,
		timeout:     timeout,
		client:      http.Client{Transport: gzhttp.Transport(http.DefaultTransport)},
		cache:       cache,
		locker:      newPartitionLocker((&sync.Mutex{})),
		rateLimiter: ratelimit.New(rl),
		concurrency: concurrency,
	}
	return &prom
}

func (prom *Prometheus) purgeExpiredCache() {
	now := time.Now()
	for _, key := range prom.cache.Keys() {
		if val, found := prom.cache.Peek(key); found {
			if c, ok := val.(queryResult); ok {
				if !c.expires.IsZero() && c.expires.Before(now) {
					prom.cache.Remove(key)
				}
			}
		}
	}
}

func (prom *Prometheus) Close() {
	log.Debug().Str("name", prom.name).Str("uri", prom.uri).Msg("Stopping query workers")
	close(prom.queries)
	prom.wg.Wait()
}

func (prom *Prometheus) StartWorkers() {
	log.Debug().
		Str("name", prom.name).
		Str("uri", prom.uri).
		Int("workers", prom.concurrency).
		Msg("Starting query workers")

	prom.queries = make(chan queryRequest, prom.concurrency*10)

	for w := 1; w <= prom.concurrency; w++ {
		prom.wg.Add(1)
		go func() {
			defer prom.wg.Done()
			queryWorker(prom, prom.queries)
		}()
	}
}

func (prom *Prometheus) doRequest(ctx context.Context, method, path string, args url.Values) (*http.Response, error) {
	u, _ := url.Parse(prom.uri)
	u.Path = strings.TrimSuffix(u.Path, "/")

	uri, err := url.JoinPath(u.String(), path)
	if err != nil {
		return nil, err
	}

	if prom.timeout > 0 {
		args.Set("timeout", prom.timeout.String())
	}

	req, err := http.NewRequestWithContext(ctx, method, uri, strings.NewReader(args.Encode()))
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	return prom.client.Do(req)
}

func queryWorker(prom *Prometheus, queries chan queryRequest) {
	for job := range queries {
		job := job

		cacheKey := job.query.CacheKey()
		if cacheKey != "" {
			if cached, ok := prom.cache.Get(cacheKey); ok {
				job.result <- cached.(queryResult)
				prometheusCacheHitsTotal.WithLabelValues(prom.name, job.query.Endpoint()).Inc()
				log.Debug().
					Str("uri", prom.uri).
					Str("query", job.query.String()).
					Str("key", cacheKey).
					Msg("Cache hit")
				continue
			}
		}
		prometheusCacheMissTotal.WithLabelValues(prom.name, job.query.Endpoint()).Inc()
		log.Debug().
			Str("uri", prom.uri).
			Str("query", job.query.String()).
			Str("key", cacheKey).
			Msg("Cache miss")

		prometheusQueriesTotal.WithLabelValues(prom.name, job.query.Endpoint()).Inc()
		prometheusQueriesRunning.WithLabelValues(prom.name, job.query.Endpoint()).Inc()
		prom.rateLimiter.Take()
		start := time.Now()
		result := job.query.Run()
		dur := time.Since(start)
		log.Debug().
			Str("uri", prom.uri).
			Str("query", job.query.String()).
			Str("endpoint", job.query.Endpoint()).
			Str("duration", output.HumanizeDuration(dur)).
			Msg("Query completed")
		prometheusQueriesRunning.WithLabelValues(prom.name, job.query.Endpoint()).Dec()
		if result.err != nil {
			prometheusQueryErrorsTotal.WithLabelValues(prom.name, job.query.Endpoint(), errReason(result.err)).Inc()
			log.Error().
				Err(result.err).
				Str("uri", prom.uri).
				Str("query", job.query.String()).
				Msg("Query returned an error")
			job.result <- result
			continue
		}

		if cacheKey != "" {
			prom.cache.Add(cacheKey, result)
		}
		prometheusCacheSize.WithLabelValues(prom.name).Set(float64(prom.cache.Len()))

		job.result <- result
	}
}

func formatTime(t time.Time) string {
	return strconv.FormatFloat(float64(t.Unix())+float64(t.Nanosecond())/1e9, 'f', -1, 64)
}

func dummyReadAll(r io.Reader) {
	_, _ = io.Copy(io.Discard, r)
}
