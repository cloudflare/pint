package promapi

import (
	"crypto/sha1"
	"fmt"
	"io"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/rs/zerolog/log"
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
	CacheKey() string
	Run() (any, error)
}

type queryRequest struct {
	query  querier
	result chan queryResult
}

type queryResult struct {
	value any
	err   error
}

type Prometheus struct {
	name        string
	uri         string
	api         v1.API
	timeout     time.Duration
	concurrency int
	cache       *lru.Cache

	wg      sync.WaitGroup
	queries chan queryRequest
}

func NewPrometheus(name, uri string, timeout time.Duration, concurrency int) *Prometheus {
	client, err := api.NewClient(api.Config{Address: uri})
	if err != nil {
		// config validation should prevent this from ever happening
		// panic so we don't need to return an error and it's easier to
		// use this code in tests
		panic(err)
	}
	cache, _ := lru.New(1000)

	prom := Prometheus{
		name:        name,
		uri:         uri,
		api:         v1.NewAPI(client),
		timeout:     timeout,
		cache:       cache,
		concurrency: concurrency,
	}
	return &prom
}

func (prom *Prometheus) Close() {
	close(prom.queries)
	prom.wg.Wait()
}

func (prom *Prometheus) StartWorkers() {
	prom.queries = make(chan queryRequest, prom.concurrency*10)

	for w := 1; w <= prom.concurrency; w++ {
		prom.wg.Add(1)
		go func() {
			defer prom.wg.Done()
			queryWorker(prom, prom.queries)
		}()
	}
}

func queryWorker(prom *Prometheus, queries chan queryRequest) {
	for job := range queries {
		job := job

		cacheKey := job.query.CacheKey()
		if cacheKey != "" {
			if cached, ok := prom.cache.Get(cacheKey); ok {
				job.result <- queryResult{value: cached}
				prometheusCacheHitsTotal.WithLabelValues(prom.name, job.query.Endpoint()).Inc()
				log.Debug().
					Str("uri", prom.uri).
					Str("query", job.query.String()).
					Msg("Cache hit")
				continue
			}
		}

		prometheusQueriesTotal.WithLabelValues(prom.name, job.query.Endpoint()).Inc()
		prometheusQueriesRunning.WithLabelValues(prom.name, job.query.Endpoint()).Inc()
		result, err := job.query.Run()
		prometheusQueriesRunning.WithLabelValues(prom.name, job.query.Endpoint()).Dec()
		if err != nil {
			prometheusQueryErrorsTotal.WithLabelValues(prom.name, job.query.Endpoint(), errReason(err)).Inc()
			log.Error().
				Err(err).
				Str("uri", prom.uri).
				Str("query", job.query.String()).
				Msg("Query returned an error")
			job.result <- queryResult{err: err}
			continue
		}

		if cacheKey != "" {
			log.Debug().
				Str("uri", prom.uri).
				Str("query", job.query.String()).
				Msg("Adding result to cache")
			prom.cache.Add(cacheKey, result)
		}

		job.result <- queryResult{value: result}
	}
}

func hash(s string) string {
	h := sha1.New()
	_, _ = io.WriteString(h, s)
	return fmt.Sprintf("%x", h.Sum(nil))
}
