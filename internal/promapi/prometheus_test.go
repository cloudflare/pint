package promapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPrometheusRequestHeaderExpandEnv(t *testing.T) {
	tests := []struct {
		header      string
		value       string
		environment map[string]string
		want        string
	}{
		{
			header:      "Aurhorization",
			value:       "Bearer $CI_JWT",
			environment: map[string]string{"CI_JWT": "8iL6E1vh5qsGpccR"},
			want:        "Bearer 8iL6E1vh5qsGpccR",
		},
		{
			header:      "Escaped",
			value:       "This $$Dollar sign is escaped",
			environment: map[string]string{"Dollar": "This variable shouldn't be expanded"},
			want:        "This $Dollar sign is escaped",
		},
		{
			header:      "Undefined",
			value:       "Variable $undef is undefined",
			environment: map[string]string{},
			want:        "Variable  is undefined",
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		require.Equal(t, query["expected"], r.Header[query.Get("header")])
	}))
	defer ts.Close()

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			headers := make(map[string]string)
			headers[tt.header] = tt.value

			for env, value := range tt.environment {
				os.Setenv(env, value)
			}

			prom := NewPrometheus("test", ts.URL, headers, 10*time.Second, 1, 100)
			prom.doRequest(context.Background(), http.MethodGet, "/", url.Values{"header": {tt.header}, "expected": {tt.want}})
		})
	}
}
