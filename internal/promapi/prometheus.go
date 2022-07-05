package promapi

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/rs/zerolog/log"

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
	cache       *lru.ARCCache
	locker      *partitionLocker
	wg          sync.WaitGroup
	queries     chan queryRequest
}

func NewPrometheus(name, uri string, timeout time.Duration, concurrency, cacheSize int) *Prometheus {
	client, err := api.NewClient(api.Config{Address: uri})
	if err != nil {
		// config validation should prevent this from ever happening
		// panic so we don't need to return an error and it's easier to
		// use this code in tests
		panic(err)
	}
	cache, _ := lru.NewARC(cacheSize)

	prom := Prometheus{
		name:        name,
		uri:         uri,
		api:         v1.NewAPI(client),
		timeout:     timeout,
		cache:       cache,
		locker:      newPartitionLocker((&sync.Mutex{})),
		concurrency: concurrency,
	}
	return &prom
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
					Str("key", cacheKey).
					Msg("Cache hit")
				continue
			}
		}

		prometheusQueriesTotal.WithLabelValues(prom.name, job.query.Endpoint()).Inc()
		prometheusQueriesRunning.WithLabelValues(prom.name, job.query.Endpoint()).Inc()
		start := time.Now()
		result, err := job.query.Run()
		dur := time.Since(start)
		log.Debug().
			Str("uri", prom.uri).
			Str("query", job.query.String()).
			Str("endpoint", job.query.Endpoint()).
			Str("duration", output.HumanizeDuration(dur)).
			Msg("Query completed")
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
			prom.cache.Add(cacheKey, result)
		}
		prometheusCacheSize.WithLabelValues(prom.name).Set(float64(prom.cache.Len()))

		job.result <- queryResult{value: result}
	}
}
