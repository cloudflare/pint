package promapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/require"

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
		case "mixed":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"mixed":[{"type":"gauge","help":"Text1","unit":"abc"},{"type":"counter","help":"Text2","unit":""}]}}`))
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
			metric:  "mixed",
			timeout: time.Second,
			metadata: promapi.MetadataResult{
				URI: srv.URL,
				Metadata: []v1.Metadata{
					{Type: "gauge", Help: "Text1", Unit: "abc"},
					{Type: "counter", Help: "Text2", Unit: ""},
				},
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
			fg := promapi.NewFailoverGroup("test", []*promapi.Prometheus{
				promapi.NewPrometheus("test", srv.URL, nil, tc.timeout, 1, 100),
			}, 1000, true, "up")
			fg.StartWorkers()
			defer fg.Close()

			metadata, err := fg.Metadata(context.Background(), tc.metric)
			if tc.err != "" {
				require.EqualError(t, err, tc.err, tc)
			} else {
				require.NoError(t, err)
			}
			if metadata != nil {
				require.Equal(t, *metadata, tc.metadata)
			}
		})
	}
}
