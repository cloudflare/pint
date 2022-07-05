package promapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Fatal(err)
		}
		metric := r.Form.Get("metric")

		switch metric {
		case "gauge":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"gauge":[{"type":"gauge","help":"Text","unit":""}]}}`))
		case "counter":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"counter":[{"type":"counter","help":"Text","unit":""}]}}`))
		case "notfound":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		case "once":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"once":[{"type":"gauge","help":"Text","unit":""}]}}`))
		case "slow":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			time.Sleep(time.Second)
			_, _ = w.Write([]byte(`{"status":"success","data":{"once":[{"type":"gauge","help":"Text","unit":""}]}}`))
		case "error":
			w.WriteHeader(500)
			_, _ = w.Write([]byte("fake error\n"))
		default:
			w.WriteHeader(400)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"unhandled metric"}`))
		}
	}))
	defer srv.Close()

	type testCaseT struct {
		metric   string
		timeout  time.Duration
		metadata promapi.MetadataResult
		err      string
	}

	testCases := []testCaseT{
		{
			metric:  "gauge",
			timeout: time.Second,
			metadata: promapi.MetadataResult{
				URI:      srv.URL,
				Metadata: []v1.Metadata{{Type: "gauge", Help: "Text", Unit: ""}},
			},
		},
		{
			metric:  "counter",
			timeout: time.Second,
			metadata: promapi.MetadataResult{
				URI:      srv.URL,
				Metadata: []v1.Metadata{{Type: "counter", Help: "Text", Unit: ""}},
			},
		},
		{
			metric:  "slow",
			timeout: time.Millisecond * 10,
			err:     "connection timeout",
		},
		{
			metric:  "error",
			timeout: time.Second,
			err:     "server_error: server error: 500",
		},
		{
			metric:  "once",
			timeout: time.Second,
			metadata: promapi.MetadataResult{
				URI:      srv.URL,
				Metadata: []v1.Metadata{{Type: "gauge", Help: "Text", Unit: ""}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.metric, func(t *testing.T) {
			assert := assert.New(t)

			prom := promapi.NewPrometheus("test", srv.URL, tc.timeout, 1, 100, 100)
			prom.StartWorkers()
			defer prom.Close()

			metadata, err := prom.Metadata(context.Background(), tc.metric)
			if tc.err != "" {
				assert.EqualError(err, tc.err, tc)
			} else {
				assert.NoError(err)
			}
			if metadata != nil {
				assert.Equal(*metadata, tc.metadata)
			}
		})
	}
}
