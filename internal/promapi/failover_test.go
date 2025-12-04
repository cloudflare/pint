package promapi

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestCacheCleaner(t *testing.T) {
	cache := newQueryCache(time.Minute, time.Now)
	quit := make(chan bool)

	// Add some entries to cache
	cache.set(1, nil, 0)
	cache.set(2, nil, 0)
	require.Len(t, cache.entries, 2)

	// Start cache cleaner with very short interval
	go cacheCleaner(cache, time.Millisecond*10, quit)

	// Wait for at least one gc cycle
	time.Sleep(time.Millisecond * 50)

	// Stop the cleaner
	quit <- true
}

func TestFailoverGroupStartWorkers(t *testing.T) {
	type testCaseT struct {
		name          string
		callTwice     bool
		expectStarted bool
	}

	testCases := []testCaseT{
		{
			name:          "starts workers on first call",
			callTwice:     false,
			expectStarted: true,
		},
		{
			name:          "idempotent on second call",
			callTwice:     true,
			expectStarted: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fg := &FailoverGroup{
				name:    "test",
				servers: []*Prometheus{},
			}
			reg := prometheus.NewRegistry()

			fg.StartWorkers(reg)
			require.Equal(t, tc.expectStarted, fg.started)

			if tc.callTwice {
				fg.StartWorkers(reg)
				require.Equal(t, tc.expectStarted, fg.started)
			}

			fg.Close(reg)
		})
	}
}

func TestFailoverGroupClose(t *testing.T) {
	type testCaseT struct {
		name       string
		startFirst bool
	}

	testCases := []testCaseT{
		{
			name:       "close on not started group is no-op",
			startFirst: false,
		},
		{
			name:       "close on started group",
			startFirst: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fg := &FailoverGroup{
				name:    "test",
				servers: []*Prometheus{},
			}
			reg := prometheus.NewRegistry()

			if tc.startFirst {
				fg.StartWorkers(reg)
				require.True(t, fg.started)
			}

			// Close should not panic regardless of started state
			require.NotPanics(t, func() {
				fg.Close(reg)
			})
		})
	}
}

func TestFailoverGroupCleanCache(t *testing.T) {
	type testCaseT struct {
		setup        func() *FailoverGroup
		name         string
		expectEmpty  bool
		expectPanics bool
	}

	testCases := []testCaseT{
		{
			name: "cleans stale cache entries",
			setup: func() *FailoverGroup {
				pastTime := time.Now().Add(-time.Hour)
				cache := newQueryCache(time.Nanosecond, func() time.Time { return pastTime })
				cache.set(1, nil, 0)
				cache.now = time.Now
				return &FailoverGroup{
					name:    "test",
					servers: []*Prometheus{{cache: cache}},
				}
			},
			expectEmpty:  true,
			expectPanics: false,
		},
		{
			name: "handles nil cache without panic",
			setup: func() *FailoverGroup {
				return &FailoverGroup{
					name:    "test",
					servers: []*Prometheus{{cache: nil}},
				}
			},
			expectEmpty:  false,
			expectPanics: false,
		},
		{
			name: "handles empty servers without panic",
			setup: func() *FailoverGroup {
				return &FailoverGroup{
					name:    "test",
					servers: []*Prometheus{},
				}
			},
			expectEmpty:  false,
			expectPanics: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fg := tc.setup()

			if tc.expectPanics {
				require.Panics(t, func() {
					fg.CleanCache()
				})
			} else {
				require.NotPanics(t, func() {
					fg.CleanCache()
				})
			}

			if tc.expectEmpty && len(fg.servers) > 0 && fg.servers[0].cache != nil {
				require.Empty(t, fg.servers[0].cache.entries)
			}
		})
	}
}
