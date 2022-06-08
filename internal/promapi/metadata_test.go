package promapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/assert"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestMedatata(t *testing.T) {
	done := sync.Map{}
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
			if _, wasDone := done.Load(r.URL.Path); wasDone {
				w.WriteHeader(500)
				_, _ = w.Write([]byte("path already requested\n"))
				return
			}
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"once":[{"type":"gauge","help":"Text","unit":""}]}}`))
			done.Store(r.URL.Path, true)
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
		runs     int
	}

	testCases := []testCaseT{
		{
			metric:  "gauge",
			timeout: time.Second,
			metadata: promapi.MetadataResult{
				URI:      srv.URL,
				Metadata: []v1.Metadata{{Type: "gauge", Help: "Text", Unit: ""}},
			},
			runs: 5,
		},
		{
			metric:  "counter",
			timeout: time.Second,
			metadata: promapi.MetadataResult{
				URI:      srv.URL,
				Metadata: []v1.Metadata{{Type: "counter", Help: "Text", Unit: ""}},
			},
			runs: 5,
		},
		{
			metric:  "slow",
			timeout: time.Millisecond * 10,
			err:     fmt.Sprintf(`failed to query Prometheus metric metadata: Get "%s/api/v1/metadata?limit=&metric=slow": context deadline exceeded`, srv.URL),
			runs:    5,
		},
		{
			metric:  "error",
			timeout: time.Second,
			err:     "failed to query Prometheus metric metadata: server_error: server error: 500",
			runs:    5,
		},
		{
			metric:  "once",
			timeout: time.Second,
			metadata: promapi.MetadataResult{
				URI:      srv.URL,
				Metadata: []v1.Metadata{{Type: "gauge", Help: "Text", Unit: ""}},
			},
			runs: 10,
		},
		// make sure /once fails on second query
		{
			metric:  "once",
			timeout: time.Second,
			runs:    2,
			err:     "failed to query Prometheus metric metadata: server_error: server error: 500",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.metric, func(t *testing.T) {
			assert := assert.New(t)

			prom := promapi.NewPrometheus("test", srv.URL, tc.timeout)

			wg := sync.WaitGroup{}
			wg.Add(tc.runs)
			for i := 1; i <= tc.runs; i++ {
				go func() {
					metadata, err := prom.Metadata(context.Background(), tc.metric)
					if tc.err != "" {
						assert.EqualError(err, tc.err, tc)
					} else {
						assert.NoError(err)
					}
					if metadata != nil {
						assert.Equal(*metadata, tc.metadata)
					}
					wg.Done()
				}()
			}
			wg.Wait()
		})
	}
}
