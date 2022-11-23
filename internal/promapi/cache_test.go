package promapi

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestQueryCacheOnlySet(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	var i uint64
	for i = 1; i <= maxSize; i++ {
		cache.set(i, mockErr, 0, 1, "/foo")
	}

	require.Equal(t, maxSize, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 0, cache.evictions)
}

func TestQueryCacheReplace(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	cache.set(6, mockErr, 0, 7, "/foo")
	cache.set(6, mockErr, 0, 7, "/foo")
	cache.set(6, mockErr, 0, 7, "/foo")

	require.Equal(t, 7, cache.cost)
	require.Equal(t, 1, len(cache.entries))
	require.Equal(t, 1, cache.useList.Len())
	require.Equal(t, 0, cache.evictions)
}

func TestQueryCacheGetAndSet(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	var i uint64
	for i = 1; i <= maxSize; i++ {
		// first get
		v, ok := cache.get(i, "/foo")
		require.Equal(t, false, ok, "should be missing from cache on first get")
		require.Zero(t, v)

		// first set
		cache.set(i, mockErr, time.Minute, 2, "/foo")

		// second get, should be in cache now
		v, ok = cache.get(i, "/foo")
		require.Equal(t, true, ok, "should be present in cache on third get")
		require.NotZero(t, v)
		require.Equal(t, mockErr, v)
	}

	require.Equal(t, 100, cache.cost)
	require.Equal(t, 50, len(cache.entries))
	require.Equal(t, 50, cache.useList.Len())
	require.Equal(t, 100, cache.stats["/foo"].hits)
	require.Equal(t, 100, cache.stats["/foo"].misses)
	require.Equal(t, 50, cache.evictions)

	cache.gc()
	require.Equal(t, 100, cache.cost)
	require.Equal(t, 50, len(cache.entries))
	require.Equal(t, 50, cache.useList.Len())
	require.Equal(t, 100, cache.stats["/foo"].hits)
	require.Equal(t, 100, cache.stats["/foo"].misses)
	require.Equal(t, 50, cache.evictions)
}

func TestQueryCachePurgeMaxCost(t *testing.T) {
	const maxSize = 460
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	var i uint64
	for i = 1; i <= 100; i++ {
		cost := int(i % 10)
		if cost == 0 {
			cost = 1
		}
		cache.set(i, mockErr, 0, cost, "/foo")
		_, _ = cache.get(i, "/foo")
	}

	require.Equal(t, cache.maxCost, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 0, cache.evictions)

	for i = 101; i <= 110; i++ {
		cost := int(i % 10)
		if cost == 0 {
			cost = 1
		}
		cost++
		cache.set(i, mockErr, 0, cost, "/bar")
		_, _ = cache.get(i, "/foo")
	}
	require.Equal(t, 460, cache.cost)
	require.Equal(t, 96, len(cache.entries))
	require.Equal(t, 96, cache.useList.Len())
	require.Equal(t, 14, cache.evictions)
}

func TestQueryCachePurgeZeroTTL(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	var i uint64
	for i = 1; i <= maxSize; i++ {
		cache.set(i, mockErr, 0, 1, "/foo")
		_, _ = cache.get(i, "/foo")
	}
	require.Equal(t, 100, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 0, cache.evictions)

	time.Sleep(time.Second)

	cache.gc()
	require.Equal(t, 100, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 0, cache.evictions)
}

func TestQueryCachePurgeExpired(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	var i uint64
	for i = 1; i <= maxSize; i++ {
		_, _ = cache.get(i, "/foo")
		_, _ = cache.get(i, "/foo")
		cache.set(i, mockErr, time.Second, 1, "/foo")
		_, _ = cache.get(i, "/foo")
	}
	require.Equal(t, 100, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 0, cache.evictions)

	for i = 1; i <= maxSize/2; i++ {
		cache.entries[i].expiresAt = time.Now().Add(time.Second * -1)
	}

	cache.gc()
	require.Equal(t, 50, cache.cost)
	require.Equal(t, 50, len(cache.entries))
	require.Equal(t, 50, cache.useList.Len())
	require.Equal(t, 50, cache.evictions)
}

func TestQueryCacheOverrideExpired(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	var i uint64
	for i = 1; i <= maxSize; i++ {
		cache.set(i, mockErr, time.Second, 1, "/foo")
		_, _ = cache.get(i, "/foo")
	}
	require.Equal(t, 100, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 0, cache.evictions)

	cache.entries[maxSize/2].expiresAt = time.Now().Add(time.Second * -1)

	cache.set(maxSize+1, mockErr, time.Second, 1, "/foo")
	_, _ = cache.get(maxSize+1, "/foo")

	require.Equal(t, 100, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 1, cache.evictions)
}

func TestQueryCacheEvictLRU(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	var i, j uint64
	for i = 1; i <= maxSize; i++ {
		cache.set(i, mockErr, time.Second, 1, "/foo")
		for j = 1; j <= i; j++ {
			_, _ = cache.get(i, "/foo")
		}
	}
	require.Equal(t, 100, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 0, cache.evictions)

	cache.gc()
	require.Equal(t, 100, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 0, cache.evictions)

	for i = maxSize + 1; i <= maxSize+20; i++ {
		cache.set(i, mockErr, time.Second, 1, "/foo")
	}
	require.Equal(t, 100, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 20, cache.evictions)

	var ok bool
	for i = 1; i <= 20; i++ {
		_, ok = cache.get(i, "/foo")
		require.False(t, ok)
	}
}

func TestQueryCacheEvictMaxStale(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Second)

	var i, j uint64
	for i = 1; i <= maxSize; i++ {
		cache.set(i, mockErr, time.Minute, 1, "/foo")
		for j = 1; j <= i; j++ {
			_, _ = cache.get(i, "/foo")
		}
	}
	require.Equal(t, 100, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 0, cache.evictions)

	cache.gc()
	require.Equal(t, 100, cache.cost)
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.useList.Len())
	require.Equal(t, 0, cache.evictions)

	time.Sleep(time.Second + time.Millisecond*100)
	for i = 1; i <= 50; i++ {
		_, _ = cache.get(i, "/foo")
	}
	cache.gc()
	require.Equal(t, 50, cache.cost)
	require.Equal(t, 50, len(cache.entries))
	require.Equal(t, 50, cache.useList.Len())
	require.Equal(t, 50, cache.evictions)

	var ok bool
	for i = 1; i <= 50; i++ {
		_, ok = cache.get(i, "/foo")
		require.True(t, ok)
	}
	for i = 51; i <= maxSize; i++ {
		_, ok = cache.get(i, "/foo")
		require.False(t, ok)
	}
}

func TestCacheCollector(t *testing.T) {
	const maxSize = 100
	cache := newQueryCache(maxSize, time.Minute)

	names := []string{
		"pint_prometheus_cache_size",
		"pint_prometheus_cache_hits_total",
		"pint_prometheus_cache_miss_total",
		"pint_prometheus_cache_evictions_total",
	}

	collector := newCacheCollector(cache, "prom")
	require.NoError(t, testutil.CollectAndCompare(
		collector, strings.NewReader(`
# HELP pint_prometheus_cache_evictions_total Total number of times an entry was evicted from query cache due to size limit or TTL
# TYPE pint_prometheus_cache_evictions_total counter
pint_prometheus_cache_evictions_total{name="prom"} 0
# HELP pint_prometheus_cache_size Total number of entries currently stored in Prometheus query cache
# TYPE pint_prometheus_cache_size gauge
pint_prometheus_cache_size{name="prom"} 0
`),
		names...,
	))

	var i uint64
	for i = 1; i <= maxSize; i++ {
		endpoint := fmt.Sprintf("/foo/%d", i%10)
		_, _ = cache.get(i, endpoint)
		_, _ = cache.get(i, endpoint)
		cache.set(i, queryResult{}, time.Minute, 1, endpoint)
		_, _ = cache.get(i, endpoint)
		cache.set(i, queryResult{}, time.Minute, 1, endpoint)
		_, _ = cache.get(i, endpoint)
	}

	require.NoError(t, testutil.CollectAndCompare(
		collector, strings.NewReader(`
# HELP pint_prometheus_cache_evictions_total Total number of times an entry was evicted from query cache due to size limit or TTL
# TYPE pint_prometheus_cache_evictions_total counter
pint_prometheus_cache_evictions_total{name="prom"} 0
# HELP pint_prometheus_cache_hits_total Total number of query cache hits
# TYPE pint_prometheus_cache_hits_total counter
pint_prometheus_cache_hits_total{endpoint="/foo/0",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/1",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/2",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/3",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/4",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/5",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/6",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/7",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/8",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/9",name="prom"} 20
# HELP pint_prometheus_cache_miss_total Total number of query cache misses
# TYPE pint_prometheus_cache_miss_total counter
pint_prometheus_cache_miss_total{endpoint="/foo/0",name="prom"} 20
pint_prometheus_cache_miss_total{endpoint="/foo/1",name="prom"} 20
pint_prometheus_cache_miss_total{endpoint="/foo/2",name="prom"} 20
pint_prometheus_cache_miss_total{endpoint="/foo/3",name="prom"} 20
pint_prometheus_cache_miss_total{endpoint="/foo/4",name="prom"} 20
pint_prometheus_cache_miss_total{endpoint="/foo/5",name="prom"} 20
pint_prometheus_cache_miss_total{endpoint="/foo/6",name="prom"} 20
pint_prometheus_cache_miss_total{endpoint="/foo/7",name="prom"} 20
pint_prometheus_cache_miss_total{endpoint="/foo/8",name="prom"} 20
pint_prometheus_cache_miss_total{endpoint="/foo/9",name="prom"} 20
# HELP pint_prometheus_cache_size Total number of entries currently stored in Prometheus query cache
# TYPE pint_prometheus_cache_size gauge
pint_prometheus_cache_size{name="prom"} 100
`),
		names...,
	))

	for i = maxSize + 1; i <= maxSize+10; i++ {
		endpoint := fmt.Sprintf("/foo/%d", i%10)
		_, _ = cache.get(i, endpoint)
		_, _ = cache.get(i, endpoint)
		cache.set(i, queryResult{}, time.Minute, 1, endpoint)
	}

	require.NoError(t, testutil.CollectAndCompare(
		collector, strings.NewReader(`
# HELP pint_prometheus_cache_evictions_total Total number of times an entry was evicted from query cache due to size limit or TTL
# TYPE pint_prometheus_cache_evictions_total counter
pint_prometheus_cache_evictions_total{name="prom"} 10
# HELP pint_prometheus_cache_hits_total Total number of query cache hits
# TYPE pint_prometheus_cache_hits_total counter
pint_prometheus_cache_hits_total{endpoint="/foo/0",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/1",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/2",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/3",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/4",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/5",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/6",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/7",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/8",name="prom"} 20
pint_prometheus_cache_hits_total{endpoint="/foo/9",name="prom"} 20
# HELP pint_prometheus_cache_miss_total Total number of query cache misses
# TYPE pint_prometheus_cache_miss_total counter
pint_prometheus_cache_miss_total{endpoint="/foo/0",name="prom"} 22
pint_prometheus_cache_miss_total{endpoint="/foo/1",name="prom"} 22
pint_prometheus_cache_miss_total{endpoint="/foo/2",name="prom"} 22
pint_prometheus_cache_miss_total{endpoint="/foo/3",name="prom"} 22
pint_prometheus_cache_miss_total{endpoint="/foo/4",name="prom"} 22
pint_prometheus_cache_miss_total{endpoint="/foo/5",name="prom"} 22
pint_prometheus_cache_miss_total{endpoint="/foo/6",name="prom"} 22
pint_prometheus_cache_miss_total{endpoint="/foo/7",name="prom"} 22
pint_prometheus_cache_miss_total{endpoint="/foo/8",name="prom"} 22
pint_prometheus_cache_miss_total{endpoint="/foo/9",name="prom"} 22
# HELP pint_prometheus_cache_size Total number of entries currently stored in Prometheus query cache
# TYPE pint_prometheus_cache_size gauge
pint_prometheus_cache_size{name="prom"} 100
`),
		names...,
	))
}

func BenchmarkQueryCacheOnlySet(b *testing.B) {
	const maxSize = 1000
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	endpoint := "/foo"
	for n := 0; n < b.N; n++ {
		cache.set(1, mockErr, 0, 1, endpoint)
	}
}

func BenchmarkQueryCacheSetGrow(b *testing.B) {
	const maxSize = 1000
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	var i uint64
	for i = 1; i <= maxSize; i++ {
		cache.set(i, mockErr, 0, 1, "/foo")
	}

	endpoint := "/foo"
	for n := 1; n <= b.N; n++ {
		cache.set(uint64(maxSize+n), mockErr, 0, 1, endpoint)
	}
}

func BenchmarkQueryCacheGetMiss(b *testing.B) {
	const maxSize = 1000
	cache := newQueryCache(maxSize, time.Minute)

	for n := 0; n < b.N; n++ {
		cache.get(uint64(n), "/foo")
	}
}

func BenchmarkQueryCacheGC(b *testing.B) {
	const maxSize = 1000
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Minute)

	var i uint64
	var ttl time.Duration
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		if n%2 == 0 {
			ttl = 0
		} else {
			ttl = time.Millisecond
		}
		for i = 1; i <= maxSize; i++ {
			cache.set(i, mockErr, ttl, 1, "/foo")
		}
		time.Sleep(time.Millisecond * 2)
		b.StartTimer()
		cache.gc()
	}
}
