package promapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Fatal(err)
		}
		query := r.Form.Get("query")

		switch query {
		case "empty":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[]
				}
			}`))
		case "single_result":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "three_results":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[
						{"metric":{"instance": "1"},"value":[1614859502.068,"1"]},
						{"metric":{"instance": "2"},"value":[1614859502.168,"2"]},
						{"metric":{"instance": "3"},"value":[1614859503.000,"3"]}
					]
				}
			}`))
		case "once":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "matrix":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"matrix",
					"result":[]
				}
			}`))
		case "timeout":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			time.Sleep(time.Second * 2)
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[]
				}
			}`))
		case "overload":
			w.WriteHeader(422)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
					"status":"error",
					"errorType":"execution",
					"error":"query processing would load too many samples into memory in query execution"
				}`))
		case "stats":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
					"status":"success",
					"data":{
						"resultType":"vector",
						"result":[{"metric":{},"value":[1614859502.068,"1"]}],
						"stats": {
							"timings": {
								"evalTotalTime": 10.1,
							  	"resultSortTime": 0.5,
							  	"queryPreparationTime": 1.5,
							  	"innerEvalTime": 0.7,
							  	"execQueueTime": 0.01,
							  	"execTotalTime": 5.1
							},
							"samples": {
							  	"totalQueryableSamples": 1000,
							  	"peakSamples": 500
							}
						}
					}
				}`))
		default:
			w.WriteHeader(400)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"error",
				"errorType":"bad_data",
				"error":"unhandled query"
			}`))
		}
	}))
	defer srv.Close()

	type testCaseT struct {
		query   string
		timeout time.Duration
		result  promapi.QueryResult
		err     string
	}

	testCases := []testCaseT{
		{
			query:   "empty",
			timeout: time.Second,
			result: promapi.QueryResult{
				URI:    srv.URL,
				Series: []promapi.Sample{},
			},
		},
		{
			query:   "single_result",
			timeout: time.Second * 5,
			result: promapi.QueryResult{
				URI: srv.URL,
				Series: []promapi.Sample{
					{
						Labels: labels.EmptyLabels(),
						Value:  1,
					},
				},
			},
		},
		{
			query:   "three_results",
			timeout: time.Second,
			result: promapi.QueryResult{
				URI: srv.URL,
				Series: []promapi.Sample{
					{
						Labels: labels.FromStrings("instance", "1"),
						Value:  1,
					},
					{
						Labels: labels.FromStrings("instance", "2"),
						Value:  2,
					},
					{
						Labels: labels.FromStrings("instance", "3"),
						Value:  3,
					},
				},
			},
		},
		{
			query:   "error",
			timeout: time.Second,
			err:     "bad_data: unhandled query",
		},
		{
			query:   "matrix",
			timeout: time.Second,
			err:     "bad_response: invalid result type, expected vector, got matrix",
		},
		{
			query:   "timeout",
			timeout: time.Millisecond * 20,
			err:     "connection timeout",
		},
		{
			query:   "once",
			timeout: time.Second,
			result: promapi.QueryResult{
				URI: srv.URL,
				Series: []promapi.Sample{
					{
						Labels: labels.EmptyLabels(),
						Value:  1,
					},
				},
			},
		},
		{
			query:   "overload",
			timeout: time.Second,
			err:     "execution: query processing would load too many samples into memory in query execution",
		},
		{
			query:   "stats",
			timeout: time.Second * 5,
			result: promapi.QueryResult{
				URI: srv.URL,
				Series: []promapi.Sample{
					{
						Labels: labels.EmptyLabels(),
						Value:  1,
					},
				},
				Stats: promapi.QueryStats{
					Timings: promapi.QueryTimings{
						EvalTotalTime:        10.1,
						ResultSortTime:       0.5,
						QueryPreparationTime: 1.5,
						InnerEvalTime:        0.7,
						ExecQueueTime:        0.01,
						ExecTotalTime:        5.1,
					},
					Samples: promapi.QuerySamples{
						TotalQueryableSamples: 1000,
						PeakSamples:           500,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			fg := promapi.NewFailoverGroup("test", srv.URL, []*promapi.Prometheus{
				promapi.NewPrometheus("test", srv.URL, srv.URL, nil, tc.timeout, 1, 100, nil),
			}, true, "up", nil, nil, nil)
			reg := prometheus.NewRegistry()
			fg.StartWorkers(reg)
			defer fg.Close(reg)

			qr, err := fg.Query(context.Background(), tc.query)
			if tc.err != "" {
				require.EqualError(t, err, tc.err, tc)
			} else {
				require.NoError(t, err)
			}
			if qr != nil {
				require.Equal(t, tc.result.URI, qr.URI)
				require.Equal(t, tc.result.Series, qr.Series)
				require.Equal(t, tc.result.Stats, qr.Stats)
			}
		})
	}
}
