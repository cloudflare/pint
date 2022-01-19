package promapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestConfig(t *testing.T) {
	done := sync.Map{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/30s/api/v1/status/config":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 30s\n"}}`))
		case "/1m/api/v1/status/config":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 1m\n"}}`))
		case "/default/api/v1/status/config":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  {}\n"}}`))
		case "/once/api/v1/status/config":
			if _, wasDone := done.Load(r.URL.Path); wasDone {
				w.WriteHeader(500)
				_, _ = w.Write([]byte("path already requested\n"))
				return
			}
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  {}\n"}}`))
			done.Store(r.URL.Path, true)
		case "/slow/api/v1/status/config":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			time.Sleep(time.Second)
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  {}\n"}}`))
		case "/error/api/v1/status/config":
			w.WriteHeader(500)
			_, _ = w.Write([]byte("fake error\n"))
		case "/badYaml/api/v1/status/config":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"invalid yaml"}}`))
		default:
			w.WriteHeader(400)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"unhandled path"}`))
		}
	}))
	defer srv.Close()

	type testCaseT struct {
		prefix  string
		timeout time.Duration
		cfg     promapi.PrometheusConfig
		err     string
		runs    int
	}

	defaults := promapi.PrometheusConfig{
		Global: promapi.ConfigSectionGlobal{
			ScrapeInterval:     time.Minute,
			ScrapeTimeout:      time.Second * 10,
			EvaluationInterval: time.Minute,
			ExternalLabels:     nil,
		},
	}

	testCases := []testCaseT{
		{
			prefix:  "/default",
			timeout: time.Second,
			cfg:     defaults,
			runs:    5,
		},
		{
			prefix:  "/1m",
			timeout: time.Second,
			cfg:     defaults,
			runs:    5,
		},
		{
			prefix:  "/30s",
			timeout: time.Second,
			cfg: promapi.PrometheusConfig{
				Global: promapi.ConfigSectionGlobal{
					ScrapeInterval:     time.Second * 30,
					ScrapeTimeout:      time.Second * 10,
					EvaluationInterval: time.Minute,
					ExternalLabels:     nil,
				},
			},
			runs: 1,
		},
		{
			prefix:  "/slow",
			timeout: time.Millisecond * 10,
			err:     fmt.Sprintf(`failed to query Prometheus config: Get "%s/slow/api/v1/status/config": context deadline exceeded`, srv.URL),
			runs:    5,
		},
		{
			prefix:  "/error",
			timeout: time.Second,
			err:     "failed to query Prometheus config: server_error: server error: 500",
			runs:    5,
		},
		{
			prefix:  "/badYaml",
			timeout: time.Second,
			err:     fmt.Sprintf("failed to decode config data in %s/badYaml response: yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `invalid...` into promapi.PrometheusConfig", srv.URL),
			runs:    5,
		},
		{
			prefix:  "/once",
			timeout: time.Second,
			cfg:     defaults,
			runs:    10,
		},
		// make sure /once fails on second query
		{
			prefix:  "/once",
			timeout: time.Second,
			runs:    2,
			err:     "failed to query Prometheus config: server_error: server error: 500",
		},
	}

	for _, tc := range testCases {
		t.Run(strings.TrimPrefix(tc.prefix, "/"), func(t *testing.T) {
			assert := assert.New(t)

			prom := promapi.NewPrometheus("test", srv.URL+tc.prefix, tc.timeout)

			wg := sync.WaitGroup{}
			wg.Add(tc.runs)
			for i := 1; i <= tc.runs; i++ {
				go func() {
					cfg, err := prom.Config(context.Background())
					if tc.err != "" {
						assert.EqualError(err, tc.err, tc)
					} else {
						assert.NoError(err)
					}
					if cfg != nil {
						assert.Equal(*cfg, tc.cfg)
					}
					wg.Done()
				}()
			}
			wg.Wait()
		})
	}
}
