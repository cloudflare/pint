package promapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/default/api/v1/status/flags":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		case "/once/api/v1/status/flags":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		case "/slow/api/v1/status/flags":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			time.Sleep(time.Second)
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		case "/error/api/v1/status/flags":
			w.WriteHeader(500)
			_, _ = w.Write([]byte("fake error\n"))
		case "/badYaml/api/v1/status/flags":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"xxx"}}`))
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
		flags   promapi.FlagsResult
		err     string
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
			err:     `bad_response: v1.apiResponse.Data: ReadObject: expect : after object field, but found }, error found in #10 byte of ...|a":{"xxx"}}|..., bigger context ...|{"status":"success","data":{"xxx"}}|...`,
		},
	}

	for _, tc := range testCases {
		t.Run(strings.TrimPrefix(tc.prefix, "/"), func(t *testing.T) {
			prom := promapi.NewPrometheus("test", srv.URL+tc.prefix, tc.timeout, 1, 1000, 100)
			prom.StartWorkers()
			defer prom.Close()

			flags, err := prom.Flags(context.Background())
			if tc.err != "" {
				require.EqualError(t, err, tc.err, tc)
			} else {
				require.NoError(t, err)
				require.Equal(t, *flags, tc.flags)
			}
		})
	}
}
