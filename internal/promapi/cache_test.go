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

func TestQueryCacheGetAndSet(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Hour)

	var i uint64
	for i = 1; i <= maxSize; i++ {
		v, ok := cache.get(i, "/foo")
		require.Equal(t, false, ok)
		require.Zero(t, v)

		cache.set(i, queryResult{err: mockErr}, 1)
		v, ok = cache.get(i, "/foo")
		require.Equal(t, false, ok)
		require.Zero(t, v)

		cache.set(i, queryResult{err: mockErr}, 1)
		v, ok = cache.get(i, "/foo")
		require.Equal(t, true, ok)
		require.NotZero(t, v)
		require.Equal(t, mockErr, v.err)
	}

	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.hits["/foo"])
	require.Equal(t, 200, cache.misses["/foo"])
	require.Equal(t, 0, cache.evictions)

	cache.gc()
	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.hits["/foo"])
	require.Equal(t, 200, cache.misses["/foo"])
	require.Equal(t, 0, cache.evictions)
}

func TestQueryCacheGetAndSetAfterZero(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Hour)

	v, ok := cache.get(1, "/foo")
	require.Equal(t, false, ok)
	require.Zero(t, v)

	cache.set(1, queryResult{err: mockErr}, 0)
	v, ok = cache.get(1, "/foo")
	require.Equal(t, true, ok)
	require.NotZero(t, v)
	require.Equal(t, mockErr, v.err)
}

func TestQueryCachePurgeMaxSize(t *testing.T) {
	const maxSize = 100
	mockErr := errors.New("Fake Error")
	cache := newQueryCache(maxSize, time.Hour)

	var i uint64
	for i = 1; i <= maxSize; i++ {
		cache.set(i, queryResult{err: mockErr}, 0)
		v, ok := cache.get(i, "/foo")
		require.Equal(t, true, ok)
		require.NotZero(t, v)
		require.Equal(t, mockErr, v.err)
	}

	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 100, cache.hits["/foo"])
	require.Equal(t, 0, cache.misses["/foo"])
	require.Equal(t, 0, cache.evictions)

	for i = maxSize + 1; i <= maxSize+10; i++ {
		v, ok := cache.get(i, "/foo")
		require.Equal(t, false, ok)
		require.Zero(t, v)

		cache.set(i, queryResult{err: mockErr}, 0)
		v, ok = cache.get(i, "/foo")
		require.Equal(t, true, ok)
		require.NotZero(t, v)
		require.Equal(t, mockErr, v.err)
	}

	require.Equal(t, 100, len(cache.entries))
	require.Equal(t, 110, cache.hits["/foo"])
	require.Equal(t, 10, cache.misses["/foo"])
	require.Equal(t, 10, cache.evictions)
}

func TestQueryCachePurgeNoRequests(t *testing.T) {
	cache := newQueryCache(100, time.Second)

	expires := time.Now().Add(time.Hour)

	var i uint64
	for i = 1; i <= 50; i++ {
		cache.set(i, queryResult{expires: expires}, 0)
	}
	require.Equal(t, 50, len(cache.entries))
	require.Equal(t, 0, cache.evictions)

	cache.gc()
	require.Equal(t, 0, len(cache.entries))
	require.Equal(t, 50, cache.evictions)
	for i = 1; i <= 50; i++ {
		_, ok := cache.get(i, "/foo")
		require.Equal(t, false, ok)
	}
}

func TestQueryCachePurgeMaxLife(t *testing.T) {
	cache := newQueryCache(100, time.Second)

	expires := time.Now().Add(time.Hour)

	var i uint64
	for i = 1; i <= 50; i++ {
		cache.set(i, queryResult{expires: expires}, 0)
		_, ok := cache.get(i, "/foo")
		require.Equal(t, true, ok)
	}
	require.Equal(t, 50, len(cache.entries))
	require.Equal(t, 0, cache.evictions)

	time.Sleep(time.Second * 2)
	for i = 1; i <= 50; i++ {
		_, ok := cache.get(i, "/foo")
		require.Equal(t, true, ok)
	}
	require.Equal(t, 50, len(cache.entries))
	require.Equal(t, 0, cache.evictions)

	time.Sleep(time.Second * 2)
	cache.gc()
	require.Equal(t, 0, len(cache.entries))
	require.Equal(t, 50, cache.evictions)
	for i = 1; i <= 50; i++ {
		_, ok := cache.get(i, "/foo")
		require.Equal(t, false, ok)
	}
}

func TestQueryCachePurgeRequests(t *testing.T) {
	cache := newQueryCache(100, time.Second)

	var i uint64
	for i = 1; i <= 50; i++ {
		_, ok := cache.get(i, "/foo")
		require.Equal(t, false, ok)
	}

	cache.gc()
	require.Equal(t, 50, len(cache.requests))

	time.Sleep(time.Second * 2)
	for i = 1; i <= 25; i++ {
		_, ok := cache.get(i, "/foo")
		require.Equal(t, false, ok)
	}
	cache.gc()
	require.Equal(t, 25, len(cache.requests))
}

func TestQueryCachePurgeExpired(t *testing.T) {
	cache := newQueryCache(100, time.Hour)

	expires := time.Now().Add(time.Second)

	var i uint64
	for i = 1; i <= 50; i++ {
		cache.set(i, queryResult{expires: expires}, 0)
		_, ok := cache.get(i, "/foo")
		require.Equal(t, true, ok)
	}
	require.Equal(t, 50, len(cache.entries))
	require.Equal(t, 0, cache.evictions)

	cache.gc()
	require.Equal(t, 50, len(cache.entries))
	require.Equal(t, 0, cache.evictions)

	time.Sleep(time.Second * 2)
	cache.gc()
	require.Equal(t, 0, len(cache.entries))
	require.Equal(t, 50, cache.evictions)
	for i = 1; i <= 50; i++ {
		_, ok := cache.get(i, "/foo")
		require.Equal(t, false, ok)
	}
}

func TestCacheCollector(t *testing.T) {
	const maxSize = 100
	cache := newQueryCache(maxSize, time.Hour)

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
		cache.set(i, queryResult{}, 1)
		_, _ = cache.get(i, endpoint)
		cache.set(i, queryResult{}, 1)
		_, _ = cache.get(i, endpoint)
	}

	require.NoError(t, testutil.CollectAndCompare(
		collector, strings.NewReader(`
# HELP pint_prometheus_cache_evictions_total Total number of times an entry was evicted from query cache due to size limit or TTL
# TYPE pint_prometheus_cache_evictions_total counter
pint_prometheus_cache_evictions_total{name="prom"} 0
# HELP pint_prometheus_cache_hits_total Total number of query cache hits
# TYPE pint_prometheus_cache_hits_total counter
pint_prometheus_cache_hits_total{endpoint="/foo/0",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/1",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/2",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/3",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/4",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/5",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/6",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/7",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/8",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/9",name="prom"} 10
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
		cache.set(i, queryResult{}, 0)
	}

	require.NoError(t, testutil.CollectAndCompare(
		collector, strings.NewReader(`
# HELP pint_prometheus_cache_evictions_total Total number of times an entry was evicted from query cache due to size limit or TTL
# TYPE pint_prometheus_cache_evictions_total counter
pint_prometheus_cache_evictions_total{name="prom"} 10
# HELP pint_prometheus_cache_hits_total Total number of query cache hits
# TYPE pint_prometheus_cache_hits_total counter
pint_prometheus_cache_hits_total{endpoint="/foo/0",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/1",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/2",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/3",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/4",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/5",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/6",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/7",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/8",name="prom"} 10
pint_prometheus_cache_hits_total{endpoint="/foo/9",name="prom"} 10
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
}
