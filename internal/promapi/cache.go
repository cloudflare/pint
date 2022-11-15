package promapi

import (
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type cacheEntry struct {
	data      queryResult
	expiresAt time.Time
	lastGet   time.Time
	cost      int
	gets      int
}

type cacheLine struct {
	key       uint64
	val       *cacheEntry
	ttl       time.Duration
	isExpired bool
}

type cacheLines []cacheLine

func (cl cacheLines) Len() int {
	return len(cl)
}

func (cl cacheLines) Swap(i, j int) {
	cl[i], cl[j] = cl[j], cl[i]
}

// [expired, costly, cheap, high ttl]
func (cl cacheLines) Less(i, j int) bool {
	if cl[i].isExpired != cl[j].isExpired {
		return cl[i].isExpired
	}

	if cl[i].val.cost != cl[j].val.cost {
		if cl[i].val.gets == 0 || cl[j].val.gets == 0 {
			return cl[i].val.cost >= cl[j].val.cost
		}

		ca := float64(cl[i].val.cost) / float64(cl[i].val.gets)
		cb := float64(cl[j].val.cost) / float64(cl[j].val.gets)
		if ca != cb {
			return ca >= cb
		}
	}

	return cl[i].ttl < cl[j].ttl
}

type endpointStats struct {
	hits   int
	misses int
}

func (e *endpointStats) hit()  { e.hits++ }
func (e *endpointStats) miss() { e.misses++ }

func newQueryCache(maxSize int, maxStale time.Duration, maxEntry float64) *queryCache {
	return &queryCache{
		entries:  map[uint64]*cacheEntry{},
		stats:    map[string]*endpointStats{},
		maxCost:  maxSize,
		maxStale: maxStale,
		maxEntry: int(float64(maxSize) * maxEntry),
	}
}

type queryCache struct {
	mu        sync.Mutex
	entries   map[uint64]*cacheEntry
	stats     map[string]*endpointStats
	maxStale  time.Duration
	maxEntry  int
	cost      int
	maxCost   int
	evictions int
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

	ce.gets++
	c.endpointStats(endpoint).hit()

	ce.lastGet = time.Now()
	return ce.data, true
}

// Cache results if it was requested at least twice EVER - which means it's either
// popular and requested multiple times within a loop OR this cache key survives between loops.
func (c *queryCache) set(key uint64, val queryResult, ttl time.Duration, cost int, endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cost > c.maxEntry {
		return
	}

	oe, ok := c.entries[key]
	if ok {
		c.cost -= oe.cost
	}

	// If we're not updating in-place then we need to make room for this entry
	if !ok && c.cost+cost > c.maxCost {
		c.makeRoom(cost)
	}

	c.cost += cost
	c.entries[key] = &cacheEntry{
		data: val,
		cost: cost,
	}
	if ttl > 0 {
		c.entries[key].expiresAt = time.Now().Add(ttl)
	}
}

func (c *queryCache) makeRoom(needed int) {
	now := time.Now()
	purgeEmptyBefore := now.Add(c.maxStale * -1)
	for key, ce := range c.entries {
		if (!ce.expiresAt.IsZero() && ce.expiresAt.Before(now)) || ce.lastGet.Before(purgeEmptyBefore) {
			c.cost -= ce.cost
			needed -= ce.cost
			delete(c.entries, key)
			c.evictions++
		}
	}
	if needed <= 0 {
		return
	}

	entries := make(cacheLines, 0, len(c.entries))
	for key, ce := range c.entries {
		entries = append(entries, cacheLine{
			key:       key,
			val:       ce,
			ttl:       ce.expiresAt.Sub(now).Round(time.Second),
			isExpired: ce.expiresAt.Before(now),
		})
	}
	sort.Stable(entries)

	for i := len(entries) - 1; i >= 0; i-- {
		c.cost -= entries[i].val.cost
		needed -= entries[i].val.cost
		delete(c.entries, entries[i].key)
		c.evictions++
		if needed <= 0 {
			return
		}
	}
}

func (c *queryCache) gc() {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries := map[uint64]*cacheEntry{}

	now := time.Now()
	purgeEmptyBefore := now.Add(c.maxStale * -1)
	for key, ce := range c.entries {
		if ce.lastGet.Before(purgeEmptyBefore) || (!ce.expiresAt.IsZero() && ce.expiresAt.Before(now)) {
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
