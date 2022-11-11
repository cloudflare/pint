package promapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestConfig(t *testing.T) {
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
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  {}\n"}}`))
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
		cfg     promapi.ConfigResult
		err     string
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
			cfg: promapi.ConfigResult{
				URI:    srv.URL + "/default",
				Config: defaults,
			},
		},
		{
			prefix:  "/1m",
			timeout: time.Second,
			cfg: promapi.ConfigResult{
				URI:    srv.URL + "/1m",
				Config: defaults,
			},
		},
		{
			prefix:  "/30s",
			timeout: time.Second,
			cfg: promapi.ConfigResult{
				URI: srv.URL + "/30s",
				Config: promapi.PrometheusConfig{
					Global: promapi.ConfigSectionGlobal{
						ScrapeInterval:     time.Second * 30,
						ScrapeTimeout:      time.Second * 10,
						EvaluationInterval: time.Minute,
						ExternalLabels:     nil,
					},
				},
			},
		},
		{
			prefix:  "/slow",
			timeout: time.Millisecond * 10,
			err:     "connection timeout",
		},
		{
			prefix:  "/error",
			timeout: time.Second,
			err:     "server_error: server error: 500",
		},
		{
			prefix:  "/badYaml",
			timeout: time.Second,
			err:     fmt.Sprintf("failed to decode config data in %s/badYaml response: yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `invalid...` into promapi.PrometheusConfig", srv.URL),
		},
	}

	for _, tc := range testCases {
		t.Run(strings.TrimPrefix(tc.prefix, "/"), func(t *testing.T) {
			prom := promapi.NewPrometheus("test", srv.URL+tc.prefix, nil, tc.timeout, 1, 100)
			prom.StartWorkers()
			defer prom.Close()

			cfg, err := prom.Config(context.Background())
			if tc.err != "" {
				require.EqualError(t, err, tc.err, tc)
			} else {
				require.NoError(t, err)
				require.Equal(t, *cfg, tc.cfg)
			}
		})
	}
}

func TestConfigHeaders(t *testing.T) {
	type testCaseT struct {
		config     map[string]string
		request    map[string]string
		shouldFail bool
	}

	testCases := []testCaseT{
		{
			config:  nil,
			request: nil,
		},
		{
			config:     nil,
			request:    map[string]string{"X-Foo": "bar"},
			shouldFail: true,
		},
		{
			config:  map[string]string{"X-Foo": "bar", "X-Bar": "foo"},
			request: map[string]string{"X-Foo": "bar", "X-Bar": "foo"},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.request {
					if tc.shouldFail {
						require.NotEqual(t, r.Header.Get(k), v)
					} else {
						require.Equal(t, r.Header.Get(k), v)
					}
				}
				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 30s\n"}}`))
			}))
			defer srv.Close()

			fg := promapi.NewFailoverGroup("test", []*promapi.Prometheus{
				promapi.NewPrometheus("test", srv.URL, tc.config, time.Second, 1, 100),
			}, 1000, true)

			fg.StartWorkers(time.Minute)
			defer fg.Close()

			_, err := fg.Config(context.Background())
			require.NoError(t, err)
		})
	}
}
