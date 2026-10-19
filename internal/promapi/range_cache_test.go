package promapi

import (
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"
)

func TestRangeQueryCacheLifetime(t *testing.T) {
	testCases := []struct {
		name         string
		wantRequests []int64
		lookback     time.Duration
		checkEvery   time.Duration
		runFor       time.Duration
	}{
		{
			name:       "eight hour lookback",
			lookback:   8 * time.Hour,
			checkEvery: 30 * time.Minute,
			runFor:     3 * time.Hour,
			// The first iteration fetches 4*2h slices: 8h/2h.
			// Each later iteration reuses historical slices and fetches one new latest slice.
			wantRequests: []int64{4, 5, 6, 7, 8, 9, 10},
		},
		{
			name:       "seven day lookback",
			lookback:   7 * 24 * time.Hour,
			checkEvery: time.Hour,
			runFor:     6 * time.Hour,
			// The first iteration fetches 84*2h slices: 7d * 24h / 2h.
			// Each later iteration reuses historical slices and fetches one new latest slice.
			wantRequests: []int64{84, 85, 86, 87, 88, 89, 90},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int64
			srv := httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(APIPathQueryRange).
					Run(func(_ *http.Request) ([]byte, error) {
						requests.Add(1)
						return []byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`), nil
					}).
					UnlimitedTimes()
			})(t)
			now := time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC)
			prom := NewPrometheus("test", srv.URL(), "", nil, time.Second, 1, 10000, 0, nil)
			prom.cache = newQueryCache(time.Hour, func() time.Time { return now })
			prom.StartWorkers()

			startedAt := now
			for elapsed := time.Duration(0); elapsed <= tc.runFor; elapsed += tc.checkEvery {
				now = startedAt.Add(elapsed)
				params := RelativeRange{
					start:    now.Add(-tc.lookback),
					end:      now,
					lookback: tc.lookback,
					step:     5 * time.Minute,
				}
				_, err := prom.RangeQuery(t.Context(), "up", params)
				require.NoError(t, err)
				require.Equal(t, tc.wantRequests[int(elapsed/tc.checkEvery)], requests.Load(),
					"Prometheus request count after %s of repeated checks", elapsed)
				prom.cache.gc()
			}
		})
	}
}
