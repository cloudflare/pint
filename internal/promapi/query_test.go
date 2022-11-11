package promapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/common/model"
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
			time.Sleep(time.Second)
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[]
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
				Series: []model.Sample{},
			},
		},
		{
			query:   "single_result",
			timeout: time.Second * 5,
			result: promapi.QueryResult{
				URI: srv.URL,
				Series: []model.Sample{
					{
						Metric:    model.Metric{},
						Value:     model.SampleValue(1),
						Timestamp: model.Time(1614859502068),
					},
				},
			},
		},
		{
			query:   "three_results",
			timeout: time.Second,
			result: promapi.QueryResult{
				URI: srv.URL,
				Series: []model.Sample{
					{
						Metric:    model.Metric{"instance": "1"},
						Value:     model.SampleValue(1),
						Timestamp: model.Time(1614859502068),
					},
					{
						Metric:    model.Metric{"instance": "2"},
						Value:     model.SampleValue(2),
						Timestamp: model.Time(1614859502168),
					},
					{
						Metric:    model.Metric{"instance": "3"},
						Value:     model.SampleValue(3),
						Timestamp: model.Time(1614859503000),
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
				Series: []model.Sample{
					{
						Metric:    model.Metric{},
						Value:     model.SampleValue(1),
						Timestamp: model.Time(1614859502068),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			fg := promapi.NewFailoverGroup("test", []*promapi.Prometheus{
				promapi.NewPrometheus("test", srv.URL, nil, tc.timeout, 1, 100),
			}, 1000, true)
			fg.StartWorkers(time.Minute)
			defer fg.Close()

			qr, err := fg.Query(context.Background(), tc.query)
			if tc.err != "" {
				require.EqualError(t, err, tc.err, tc)
			} else {
				require.NoError(t, err)
			}
			if qr != nil {
				require.Equal(t, tc.result.URI, qr.URI)
				require.Equal(t, tc.result.Series, qr.Series)
			}
		})
	}
}
