package promapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/default" + promapi.APIPathFlags:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		case "/foo" + promapi.APIPathFlags:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"foo":"bar"}}`))
		case "/once" + promapi.APIPathFlags:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		case "/slow" + promapi.APIPathFlags:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			time.Sleep(time.Second * 2)
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		case "/error" + promapi.APIPathFlags:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("fake error\n"))
		case "/badYaml" + promapi.APIPathFlags:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"xxx"}}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"unhandled path"}`))
		}
	}))
	defer srv.Close()

	type testCaseT struct {
		flags   promapi.FlagsResult
		prefix  string
		err     string
		timeout time.Duration
	}

	testCases := []testCaseT{
		{
			prefix:  "/default",
			timeout: time.Second,
			flags: promapi.FlagsResult{
				URI:   srv.URL + "/default",
				Flags: v1.FlagsResult{},
			},
		},
		{
			prefix:  "/foo",
			timeout: time.Second,
			flags: promapi.FlagsResult{
				URI:   srv.URL + "/foo",
				Flags: v1.FlagsResult{"foo": "bar"},
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
			err:     "server_error: 500 Internal Server Error",
		},
		{
			prefix:  "/badYaml",
			timeout: time.Second,
			err:     `bad_response: JSON parse error: expected colon after object key`,
		},
	}

	for _, tc := range testCases {
		t.Run(strings.TrimPrefix(tc.prefix, "/"), func(t *testing.T) {
			fg := promapi.NewFailoverGroup("test", srv.URL+tc.prefix, []*promapi.Prometheus{
				promapi.NewPrometheus("test", srv.URL+tc.prefix, "", nil, tc.timeout, 1, 100, nil),
			}, true, "up", nil, nil, nil)

			reg := prometheus.NewRegistry()
			fg.StartWorkers(reg)
			defer fg.Close(reg)

			flags, err := fg.Flags(t.Context())
			if tc.err != "" {
				require.EqualError(t, err, tc.err, tc)
			} else {
				require.NoError(t, err)
				require.Equal(t, *flags, tc.flags)
			}
		})
	}
}
