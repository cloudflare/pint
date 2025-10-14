package promapi

import (
	"maps"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type cacheEntry struct {
	data      any
	expiresAt time.Time
	lastGet   time.Time
}

type endpointStats struct {
	hits   int
	misses int
}

func (e *endpointStats) hit()  { e.hits++ }
func (e *endpointStats) miss() { e.misses++ }

func newQueryCache(maxStale time.Duration, now nowFunc) *queryCache {
	// nolint: exhaustruct
	return &queryCache{
		now:      now,
		entries:  map[uint64]*cacheEntry{},
		stats:    map[string]*endpointStats{},
		maxStale: maxStale,
	}
}

type nowFunc func() time.Time

type queryCache struct {
	now       nowFunc
	entries   map[uint64]*cacheEntry
	stats     map[string]*endpointStats
	maxStale  time.Duration
	evictions int
	capacity  int
	mu        sync.Mutex
}

func (c *queryCache) endpointStats(endpoint string) *endpointStats {
	e, ok := c.stats[endpoint]
	if ok {
		return e
	}

	e = &endpointStats{hits: 0, misses: 0}
	c.stats[endpoint] = e
	return e
}

func (c *queryCache) get(key uint64, endpoint string) (v any, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var ce *cacheEntry
	ce, ok = c.entries[key]
	if !ok {
		c.endpointStats(endpoint).miss()
		return v, ok
	}

	ce.lastGet = c.now()
	c.endpointStats(endpoint).hit()

	return ce.data, true
}

// Cache results if it was requested at least twice EVER - which means it's either
// popular and requested multiple times within a loop OR this cache key survives between loops.
func (c *queryCache) set(key uint64, val any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &cacheEntry{
		data:      val,
		lastGet:   c.now(),
		expiresAt: time.Time{},
	}
	if ttl > 0 {
		c.entries[key].expiresAt = c.now().Add(ttl)
	}
	c.capacity = max(c.capacity, len(c.entries))
}

func (c *queryCache) gc() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var evictions int

	now := c.now()

	for key, ce := range c.entries {
		if c.needsEviction(now, ce) {
			evictions++
			delete(c.entries, key)
		}
	}

	c.evictions += evictions

	// Re-create the map if we evicted >= 25% of entries.
	if c.capacity > 0 && (c.capacity-len(c.entries)) >= (c.capacity/4) {
		entries := make(map[uint64]*cacheEntry, len(c.entries))
		maps.Copy(entries, c.entries)
		c.entries = entries
		c.capacity = len(c.entries)
	}
}

func (c *queryCache) needsEviction(now time.Time, ce *cacheEntry) bool {
	return (!ce.expiresAt.IsZero() && ce.expiresAt.Before(now)) || now.Sub(ce.lastGet) >= c.maxStale
}

type cacheCollector struct {
	cache     *queryCache
	entries   *prometheus.Desc
	capacity  *prometheus.Desc
	evictions *prometheus.Desc
	hits      *prometheus.Desc
	misses    *prometheus.Desc
}

func newCacheCollector(cache *queryCache, name string) *cacheCollector {
	return &cacheCollector{
		cache: cache,
		entries: prometheus.NewDesc(
			"pint_prometheus_cache_size",
			"Total number of entries currently stored in Prometheus query cache.",
			nil,
			prometheus.Labels{"name": name},
		),
		capacity: prometheus.NewDesc(
			"pint_prometheus_cache_capacity",
			"The capacity of Prometheus query cache.",
			nil,
			prometheus.Labels{"name": name},
		),
		evictions: prometheus.NewDesc(
			"pint_prometheus_cache_evictions_total",
			"Total number of times an entry was evicted from query cache due to size limit or TTL.",
			nil,
			prometheus.Labels{"name": name},
		),
		hits: prometheus.NewDesc(
			"pint_prometheus_cache_hits_total",
			"Total number of query cache hits.",
			[]string{"endpoint"},
			prometheus.Labels{"name": name},
		),
		misses: prometheus.NewDesc(
			"pint_prometheus_cache_miss_total",
			"Total number of query cache misses.",
			[]string{"endpoint"},
			prometheus.Labels{"name": name},
		),
	}
}

func (c *cacheCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.entries
	ch <- c.capacity
	ch <- c.evictions
	ch <- c.hits
	ch <- c.misses
}

func (c *cacheCollector) Collect(ch chan<- prometheus.Metric) {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	ch <- prometheus.MustNewConstMetric(c.entries, prometheus.GaugeValue, float64(len(c.cache.entries)))
	ch <- prometheus.MustNewConstMetric(c.capacity, prometheus.GaugeValue, float64(c.cache.capacity))
	ch <- prometheus.MustNewConstMetric(c.evictions, prometheus.CounterValue, float64(c.cache.evictions))

	for endpoint, stats := range c.cache.stats {
		ch <- prometheus.MustNewConstMetric(c.hits, prometheus.CounterValue, float64(stats.hits), endpoint)
		ch <- prometheus.MustNewConstMetric(c.misses, prometheus.CounterValue, float64(stats.misses), endpoint)
	}
}
