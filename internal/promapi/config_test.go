package promapi_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestConfig(t *testing.T) {
	type testCaseT struct {
		mock        httpmock.Mocker
		errCheck    func(t *testing.T, err error)
		name        string
		cfg         promapi.PrometheusConfig
		timeout     time.Duration
		useFailover bool
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
			name:    "default",
			timeout: time.Second,
			cfg:     defaults,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathConfig).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"yaml":"global:\n  {}\n"}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "1m",
			timeout: time.Second,
			cfg:     defaults,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathConfig).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 1m\n"}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "30s",
			timeout: time.Second,
			cfg: promapi.PrometheusConfig{
				Global: promapi.ConfigSectionGlobal{
					ScrapeInterval:     time.Second * 30,
					ScrapeTimeout:      time.Second * 10,
					EvaluationInterval: time.Minute,
					ExternalLabels:     nil,
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathConfig).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 30s\n"}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "slow",
			timeout: time.Millisecond * 10,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				require.EqualError(t, err, "connection timeout")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathConfig).
					Run(func(_ *http.Request) ([]byte, error) {
						time.Sleep(time.Second * 2)
						return []byte(`{"status":"success","data":{"yaml":"global:\n  {}\n"}}`), nil
					}).
					UnlimitedTimes()
			}),
		},
		{
			name:    "error",
			timeout: time.Second,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				require.EqualError(t, err, "server_error: 500 Internal Server Error")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathConfig).
					ReturnCode(http.StatusInternalServerError).
					Return("fake error\n").
					UnlimitedTimes()
			}),
		},
		{
			name:    "badYaml",
			timeout: time.Second,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				require.ErrorContains(t, err, "failed to decode config data in")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathConfig).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"yaml":"invalid yaml"}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "badJson",
			timeout: time.Second,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				require.EqualError(t, err, `bad_response: JSON parse error: jsontext: invalid character '}' after object name (expecting ':') within "/data/yaml" after offset 34`)
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathConfig).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"yaml"}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "apiError",
			timeout: time.Second,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				require.EqualError(t, err, "bad_data: custom error message")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathConfig).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data","error":"custom error message"}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "emptyError",
			timeout: time.Second,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				require.EqualError(t, err, "bad_data: empty response object")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathConfig).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data"}`).
					UnlimitedTimes()
			}),
		},
		// Verifies that FailoverGroup.Config returns a FailoverGroupError
		// immediately when the server returns a non-unavailable error (bad_data).
		{
			name:        "failover/non-unavailable error",
			timeout:     time.Second,
			useFailover: true,
			errCheck: func(t *testing.T, err error) {
				t.Helper()
				require.EqualError(t, err, "bad_data: custom error message")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathConfig).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data","error":"custom error message"}`).
					UnlimitedTimes()
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := tc.mock(t)

			prom := promapi.NewPrometheus("test", srv.URL(), "", nil, tc.timeout, 1, 100, nil)

			var cfg *promapi.ConfigResult
			var err error
			if tc.useFailover {
				fg := promapi.NewFailoverGroup("test", srv.URL(), []*promapi.Prometheus{prom}, true, "up", nil, nil, nil)
				reg := prometheus.NewRegistry()
				fg.StartWorkers(reg)
				t.Cleanup(func() { fg.Close(reg) })
				cfg, err = fg.Config(t.Context(), 0)
			} else {
				prom.StartWorkers()
				t.Cleanup(prom.Close)
				cfg, err = prom.Config(t.Context(), time.Minute)
			}

			if tc.errCheck != nil {
				tc.errCheck(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.cfg, cfg.Config)
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
						assert.NotEqual(t, r.Header.Get(k), v)
					} else {
						assert.Equal(t, r.Header.Get(k), v)
					}
				}
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 30s\n"}}`))
			}))
			t.Cleanup(srv.Close)

			fg := promapi.NewFailoverGroup("test", srv.URL, []*promapi.Prometheus{
				promapi.NewPrometheus("test", srv.URL, "", tc.config, time.Second, 1, 100, nil),
			}, true, "up", nil, nil, nil)

			reg := prometheus.NewRegistry()
			fg.StartWorkers(reg)
			defer fg.Close(reg)

			_, err := fg.Config(t.Context(), 0)
			require.NoError(t, err)
		})
	}
}
