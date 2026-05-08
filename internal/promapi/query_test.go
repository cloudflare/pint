package promapi_test

import (
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestQuery(t *testing.T) {
	type testCaseT struct {
		mock      httpmock.Mocker
		name      string
		assertErr func(t *testing.T, err error)
		series    []promapi.Sample
		stats     promapi.QueryStats
		timeout   time.Duration
	}

	testCases := []testCaseT{
		{
			name:    "empty",
			timeout: time.Second,
			series:  []promapi.Sample{},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
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
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
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
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
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
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "bad_data: unhandled query")
			},
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
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "bad_response: invalid result type, expected vector, got matrix")
			},
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
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "connection timeout")
			},
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
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
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
			assertErr: func(t *testing.T, err error) {
				require.EqualError(
					t, err,
					"execution: query processing would load too many samples into memory in query execution",
				)
			},
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
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
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
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "bad_data: custom error message")
			},
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
			assertErr: func(t *testing.T, err error) {
				require.EqualError(
					t, err,
					`bad_response: JSON parse error: jsontext: invalid character '}' after object name (expecting ':') within "/data/resultType" after offset 40`,
				)
			},
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
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "bad_data: empty response object")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data"}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "badMetricKind",
			timeout: time.Second,
			assertErr: func(t *testing.T, err error) {
				var se *json.SemanticError
				require.ErrorAs(t, err, &se)
				require.Equal(t, reflect.TypeFor[promapi.SampleLabels](), se.GoType)
				require.Equal(t, byte('['), byte(se.JSONKind))
				require.Equal(t, "/data/result/0/metric", string(se.JSONPointer))
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[{"metric":[],"value":[1614859502.068,"1"]}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "badValueKind",
			timeout: time.Second,
			assertErr: func(t *testing.T, err error) {
				var se *json.SemanticError
				require.ErrorAs(t, err, &se)
				require.Equal(t, reflect.TypeFor[promapi.SampleTimestampValue](), se.GoType)
				require.Equal(t, byte('{'), byte(se.JSONKind))
				require.Equal(t, "/data/result/0/value", string(se.JSONPointer))
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":{}}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "badValueClosingToken",
			timeout: time.Second,
			assertErr: func(t *testing.T, err error) {
				var se *json.SemanticError
				require.ErrorAs(t, err, &se)
				require.Equal(t, reflect.TypeFor[promapi.SampleTimestampValue](), se.GoType)
				require.Equal(t, byte('{'), byte(se.JSONKind))
				require.Equal(t, "/data/result/0/value", string(se.JSONPointer))
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":{"ts":1}}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "badTimestampNotANumber",
			timeout: time.Second,
			assertErr: func(t *testing.T, err error) {
				var se *json.SemanticError
				require.ErrorAs(t, err, &se)
				require.Equal(t, reflect.TypeFor[promapi.SampleTimestampValue](), se.GoType)
				require.Equal(t, "/data/result/0/value", string(se.JSONPointer))
				require.Contains(t, err.Error(), `strconv.ParseFloat: parsing "notanumber": invalid syntax`)
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":["notanumber","1"]}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "badValueExtraElements",
			timeout: time.Second,
			assertErr: func(t *testing.T, err error) {
				var se *json.SemanticError
				require.ErrorAs(t, err, &se)
				require.Equal(t, reflect.TypeFor[promapi.SampleTimestampValue](), se.GoType)
				require.Equal(t, byte('"'), byte(se.JSONKind))
				require.Equal(t, "/data/result/0/value", string(se.JSONPointer))
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQuery).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1614859502.068,"1","extra"]}]}}`).
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
			tc.assertErr(t, err)
			if qr != nil {
				require.Equal(t, tc.series, qr.Series)
				require.Equal(t, tc.stats, qr.Stats)
			}
		})
	}
}

// errReader returns the provided data, then an error on the next read.
type errReader struct {
	r   io.Reader
	err error
}

func (e *errReader) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	if errors.Is(err, io.EOF) {
		return n, e.err
	}
	return n, err
}

func TestSampleLabelsUnmarshalJSONFromReadTokenErrors(t *testing.T) {
	type testCaseT struct {
		input io.Reader
		name  string
	}

	testCases := []testCaseT{
		{
			// PeekKind sees '{' but ReadToken fails reading it.
			name:  "error reading opening brace",
			input: &errReader{r: strings.NewReader("{"), err: io.ErrUnexpectedEOF},
		},
		{
			// Opening '{' is read but ReadToken fails on the first key.
			name:  "error reading key inside object",
			input: &errReader{r: strings.NewReader(`{"k`), err: io.ErrUnexpectedEOF},
		},
		{
			// All keys read but ReadToken fails on closing '}'.
			name:  "error reading closing brace",
			input: &errReader{r: strings.NewReader(`{"a":"b"`), err: io.ErrUnexpectedEOF},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dec := jsontext.NewDecoder(tc.input)
			var s promapi.SampleLabels
			err := s.UnmarshalJSONFrom(dec)
			require.Error(t, err)
		})
	}
}

func TestSampleTimestampValueUnmarshalJSONFromReadTokenErrors(t *testing.T) {
	type testCaseT struct {
		input io.Reader
		name  string
	}

	testCases := []testCaseT{
		{
			// PeekKind sees '[' but ReadToken fails reading it.
			name:  "error reading opening bracket",
			input: &errReader{r: strings.NewReader("["), err: io.ErrUnexpectedEOF},
		},
		{
			// Opening '[' is read but ReadToken fails on the timestamp.
			name:  "error reading timestamp token",
			input: &errReader{r: strings.NewReader(`[1`), err: io.ErrUnexpectedEOF},
		},
		{
			// Timestamp read but ReadToken fails on the value string.
			name:  "error reading value token",
			input: &errReader{r: strings.NewReader(`[1614859502.068,"1`), err: io.ErrUnexpectedEOF},
		},
		{
			// Value read but ReadToken fails on closing ']'.
			name:  "error reading closing bracket",
			input: &errReader{r: strings.NewReader(`[1614859502.068,"1"`), err: io.ErrUnexpectedEOF},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dec := jsontext.NewDecoder(tc.input)
			var s promapi.SampleTimestampValue
			err := s.UnmarshalJSONFrom(dec)
			require.Error(t, err)
		})
	}
}
