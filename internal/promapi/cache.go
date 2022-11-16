package promapi

import (
	"container/list"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type cacheEntry struct {
	lst       *list.Element
	data      queryResult
	expiresAt time.Time
	cost      int
}

type endpointStats struct {
	hits   int
	misses int
}

func (e *endpointStats) hit()  { e.hits++ }
func (e *endpointStats) miss() { e.misses++ }

func newQueryCache(maxSize int) *queryCache {
	return &queryCache{
		entries: map[uint64]*cacheEntry{},
		stats:   map[string]*endpointStats{},
		maxCost: maxSize,
		useList: list.New(),
	}
}

type queryCache struct {
	mu        sync.Mutex
	entries   map[uint64]*cacheEntry
	stats     map[string]*endpointStats
	cost      int
	maxCost   int
	evictions int
	useList   *list.List
}

func (c *queryCache) endpointStats(endpoint string) *endpointStats {
	e, ok := c.stats[endpoint]
	if ok {
		return e
	}

	e = &endpointStats{}
	c.stats[endpoint] = e
	return e
}

func (c *queryCache) get(key uint64, endpoint string) (v queryResult, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var ce *cacheEntry
	ce, ok = c.entries[key]
	if !ok {
		c.endpointStats(endpoint).miss()
		return v, ok
	}

	c.useList.MoveToFront(ce.lst)
	c.endpointStats(endpoint).hit()

	return ce.data, true
}

// Cache results if it was requested at least twice EVER - which means it's either
// popular and requested multiple times within a loop OR this cache key survives between loops.
func (c *queryCache) set(key uint64, val queryResult, ttl time.Duration, cost int, endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var lst *list.Element
	oe, ok := c.entries[key]
	if ok {
		c.cost -= oe.cost
		lst = oe.lst
	} else {
		lst = c.useList.PushFront(key)
	}

	// If we're not updating in-place then we need to make room for this entry
	if !ok && c.cost+cost > c.maxCost {
		c.makeRoom(cost)
	}

	c.cost += cost
	c.entries[key] = &cacheEntry{
		data: val,
		cost: cost,
		lst:  lst,
	}
	if ttl > 0 {
		c.entries[key].expiresAt = time.Now().Add(ttl)
	}
}

func (c *queryCache) makeRoom(needed int) {
	for c.useList.Len() > 0 && needed > 0 {
		if lst := c.useList.Back(); lst != nil {
			key := lst.Value.(uint64)
			c.cost -= c.entries[key].cost
			needed -= c.entries[key].cost
			delete(c.entries, key)
			c.useList.Remove(lst)
			c.evictions++
		}
	}
}

func (c *queryCache) gc() {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries := map[uint64]*cacheEntry{}

	now := time.Now()
	for key, ce := range c.entries {
		if !ce.expiresAt.IsZero() && ce.expiresAt.Before(now) {
			c.useList.Remove(ce.lst)
			c.cost -= ce.cost
			c.evictions++
			continue
		}
		entries[key] = ce
	}
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
	ch <- prometheus.MustNewConstMetric(c.entries, prometheus.GaugeValue, float64(c.cache.cost))

	for endpoint, stats := range c.cache.stats {
		ch <- prometheus.MustNewConstMetric(c.hits, prometheus.CounterValue, float64(stats.hits), endpoint)
		ch <- prometheus.MustNewConstMetric(c.misses, prometheus.CounterValue, float64(stats.misses), endpoint)
	}
	ch <- prometheus.MustNewConstMetric(c.evictions, prometheus.CounterValue, float64(c.cache.evictions))
}
