package promapi

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func newQueryCache(maxSize int, maxSinceLastRequest time.Duration) *queryCache {
	return &queryCache{
		requests:            map[uint64]cacheRequest{},
		entries:             map[uint64]queryResult{},
		maxSize:             maxSize,
		maxSinceLastRequest: maxSinceLastRequest,
		hits:                map[string]int{},
		misses:              map[string]int{},
	}
}

type cacheRequest struct {
	lastGet time.Time
	count   int
}

type queryCache struct {
	mu                  sync.Mutex
	requests            map[uint64]cacheRequest
	entries             map[uint64]queryResult
	maxSize             int
	maxSinceLastRequest time.Duration
	hits                map[string]int
	misses              map[string]int
	evictions           int
}

func (c *queryCache) get(key uint64, endpoint string) (v queryResult, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var r cacheRequest
	r, ok = c.requests[key]
	if ok {
		r.lastGet = time.Now()
		r.count++
		c.requests[key] = r
	} else {
		c.requests[key] = cacheRequest{
			lastGet: time.Now(),
			count:   1,
		}
	}

	v, ok = c.entries[key]
	if ok {
		c.hits[endpoint]++
	} else {
		c.misses[endpoint]++
	}

	return v, ok
}

func (c *queryCache) set(key uint64, val queryResult, minRequests int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if minRequests > 0 {
		if r, ok := c.requests[key]; !ok || r.count <= minRequests {
			return
		}
	}

	if len(c.entries) >= c.maxSize {
		c.purgeOne()
	}

	c.entries[key] = val
}

func (c *queryCache) purgeOne() {
	var purgeKey uint64
	purgeRequests := -100
	var purgeLastGet time.Time
	for k, r := range c.requests {
		if r.count < purgeRequests || purgeRequests < 0 || (r.count == purgeRequests && r.lastGet.Before(purgeLastGet)) {
			purgeKey = k
			purgeRequests = r.count
			purgeLastGet = r.lastGet
		}
	}
	delete(c.entries, purgeKey)
	delete(c.requests, purgeKey)
	c.evictions++
}

func (c *queryCache) gc() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var r cacheRequest
	var ok bool

	requests := map[uint64]cacheRequest{}
	entries := map[uint64]queryResult{}
	now := time.Now()
	for k, v := range c.entries {
		r, ok = c.requests[k]
		if !ok || (ok && now.Sub(r.lastGet) >= c.maxSinceLastRequest) {
			c.evictions++
			continue
		}
		if v.expires.IsZero() || !v.expires.Before(now) {
			if ok {
				requests[k] = r
			}
			entries[k] = v
		} else {
			c.evictions++
		}
	}

	for k, r := range c.requests {
		if _, ok = requests[k]; ok {
			continue
		}
		if now.Sub(r.lastGet) < c.maxSinceLastRequest {
			requests[k] = r
		}
	}

	c.requests = requests
	c.entries = entries
}

type cacheCollector struct {
	cache     *queryCache
	entries   *prometheus.Desc
	hits      *prometheus.Desc
	misses    *prometheus.Desc
	evictions *prometheus.Desc
}

func newCacheCollector(cache *queryCache, name string) *cacheCollector {
	return &cacheCollector{
		cache: cache,
		entries: prometheus.NewDesc(
			"pint_prometheus_cache_size",
			"Total number of entries currently stored in Prometheus query cache",
			nil,
			prometheus.Labels{"name": name},
		),
		hits: prometheus.NewDesc(
			"pint_prometheus_cache_hits_total",
			"Total number of query cache hits",
			[]string{"endpoint"},
			prometheus.Labels{"name": name},
		),
		misses: prometheus.NewDesc(
			"pint_prometheus_cache_miss_total",
			"Total number of query cache misses",
			[]string{"endpoint"},
			prometheus.Labels{"name": name},
		),
		evictions: prometheus.NewDesc(
			"pint_prometheus_cache_evictions_total",
			"Total number of times an entry was evicted from query cache due to size limit or TTL",
			nil,
			prometheus.Labels{"name": name},
		),
	}
}

func (c *cacheCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.entries
	ch <- c.hits
	ch <- c.misses
	ch <- c.evictions
}

func (c *cacheCollector) Collect(ch chan<- prometheus.Metric) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	ch <- prometheus.MustNewConstMetric(c.entries, prometheus.GaugeValue, float64(len(c.cache.entries)))
	for endpoint, hits := range c.cache.hits {
		ch <- prometheus.MustNewConstMetric(c.hits, prometheus.CounterValue, float64(hits), endpoint)
	}
	for endpoint, misses := range c.cache.misses {
		ch <- prometheus.MustNewConstMetric(c.misses, prometheus.CounterValue, float64(misses), endpoint)
	}
	ch <- prometheus.MustNewConstMetric(c.evictions, prometheus.CounterValue, float64(c.cache.evictions))
}
