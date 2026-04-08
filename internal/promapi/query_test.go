package promapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestQuery(t *testing.T) {
	type testCaseT struct {
		mock    httpmock.Mocker
		name    string
		err     string
		series  []promapi.Sample
		stats   promapi.QueryStats
		timeout time.Duration
	}

	testCases := []testCaseT{
		{
			name:    "empty",
			timeout: time.Second,
			series:  []promapi.Sample{},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "single_result",
			timeout: time.Second * 5,
			series: []promapi.Sample{
				{
					Labels: labels.EmptyLabels(),
					Value:  1,
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1614859502.068,"1"]}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "three_results",
			timeout: time.Second,
			series: []promapi.Sample{
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"instance":"1"},"value":[1614859502.068,"1"]},{"metric":{"instance":"2"},"value":[1614859502.168,"2"]},{"metric":{"instance":"3"},"value":[1614859503.000,"3"]}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "error",
			timeout: time.Second,
			err:     "bad_data: unhandled query",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnCode(http.StatusBadRequest).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data","error":"unhandled query"}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "matrix",
			timeout: time.Second,
			err:     "bad_response: invalid result type, expected vector, got matrix",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"matrix","result":[]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "timeout",
			timeout: time.Millisecond * 20,
			err:     "connection timeout",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					Run(func(_ *http.Request) ([]byte, error) {
						time.Sleep(time.Second * 2)
						return []byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`), nil
					}).
					UnlimitedTimes()
			}),
		},
		{
			name:    "once",
			timeout: time.Second,
			series: []promapi.Sample{
				{
					Labels: labels.EmptyLabels(),
					Value:  1,
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1614859502.068,"1"]}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "overload",
			timeout: time.Second,
			err:     "execution: query processing would load too many samples into memory in query execution",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnCode(http.StatusUnprocessableEntity).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"execution","error":"query processing would load too many samples into memory in query execution"}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "stats",
			timeout: time.Second * 5,
			series: []promapi.Sample{
				{
					Labels: labels.EmptyLabels(),
					Value:  1,
				},
			},
			stats: promapi.QueryStats{
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
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1614859502.068,"1"]}],"stats":{"timings":{"evalTotalTime":10.1,"resultSortTime":0.5,"queryPreparationTime":1.5,"innerEvalTime":0.7,"execQueueTime":0.01,"execTotalTime":5.1},"samples":{"totalQueryableSamples":1000,"peakSamples":500}}}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "apiError",
			timeout: time.Second,
			err:     "bad_data: custom error message",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data","error":"custom error message"}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "badJson",
			timeout: time.Second,
			err:     `bad_response: JSON parse error: jsontext: invalid character '}' after object name (expecting ':') within "/data/resultType" after offset 40`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType"}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "emptyError",
			timeout: time.Second,
			err:     `bad_data: empty response object`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data"}`).
					UnlimitedTimes()
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := tc.mock(t)

			fg := promapi.NewFailoverGroup("test", srv.URL(), []*promapi.Prometheus{
				promapi.NewPrometheus("test", srv.URL(), srv.URL(), nil, tc.timeout, 1, 100, nil),
			}, true, "up", nil, nil, nil)
			reg := prometheus.NewRegistry()
			fg.StartWorkers(reg)
			defer fg.Close(reg)

			qr, err := fg.Query(t.Context(), tc.name)
			if tc.err != "" {
				require.EqualError(t, err, tc.err, tc)
			} else {
				require.NoError(t, err)
			}
			if qr != nil {
				require.Equal(t, tc.series, qr.Series)
				require.Equal(t, tc.stats, qr.Stats)
			}
		})
	}
}
