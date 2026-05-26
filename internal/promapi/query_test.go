package promapi_test

import (
	"context"
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
		ctx       func(t *testing.T) context.Context
		mock      httpmock.Mocker
		name      string
		assertErr func(t *testing.T, err error)
		series    []promapi.Sample
		stats     promapi.QueryStats
		timeout   time.Duration
	}

	testCases := []testCaseT{
		{
			name:    "empty result",
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
			name:    "API error",
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
			name:    "unexpected matrix result type",
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
			name:    "connection timeout",
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
			name:    "cached response",
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
			name:    "query overload",
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
			name:    "response with stats",
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
			name:    "API error with message",
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
			name:    "invalid JSON",
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
			name:    "API error without message",
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
			name:    "invalid metric type in response",
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
			name:    "invalid value type in response",
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
			name:    "missing value closing token",
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
			name:    "non-numeric timestamp",
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
			name:    "extra elements in value array",
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
		{
			name:    "context cancelled",
			timeout: time.Second,
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "context canceled")
			},
			mock: httpmock.New(func(_ *httpmock.Server) {}),
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

			ctx := t.Context()
			if tc.ctx != nil {
				ctx = tc.ctx(t)
			}

			qr, err := fg.Query(ctx, tc.name).Wait()
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
		err   string
	}

	testCases := []testCaseT{
		{
			name:  "error on first read",
			input: &errReader{r: strings.NewReader(""), err: io.ErrUnexpectedEOF},
			err:   "jsontext: read error: unexpected EOF",
		},
		{
			name:  "error reading key inside object",
			input: &errReader{r: strings.NewReader(`{"k`), err: io.ErrUnexpectedEOF},
			err:   "jsontext: read error: unexpected EOF",
		},
		{
			name:  "error reading closing brace",
			input: &errReader{r: strings.NewReader(`{"a":"b"`), err: io.ErrUnexpectedEOF},
			err:   "jsontext: read error: unexpected EOF",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dec := jsontext.NewDecoder(tc.input)
			var s promapi.SampleLabels
			err := s.UnmarshalJSONFrom(dec)
			require.EqualError(t, err, tc.err)
		})
	}
}

func TestSampleTimestampValueUnmarshalJSONFromReadTokenErrors(t *testing.T) {
	type testCaseT struct {
		input io.Reader
		name  string
		err   string
	}

	testCases := []testCaseT{
		{
			name:  "error on first read",
			input: &errReader{r: strings.NewReader(""), err: io.ErrUnexpectedEOF},
			err:   "jsontext: read error: unexpected EOF",
		},
		{
			name:  "error reading timestamp token",
			input: &errReader{r: strings.NewReader(`[1`), err: io.ErrUnexpectedEOF},
			err:   "jsontext: read error: unexpected EOF",
		},
		{
			name:  "error reading value token",
			input: &errReader{r: strings.NewReader(`[1614859502.068,"1`), err: io.ErrUnexpectedEOF},
			err:   "jsontext: read error: unexpected EOF",
		},
		{
			name:  "error reading closing bracket",
			input: &errReader{r: strings.NewReader(`[1614859502.068,"1"`), err: io.ErrUnexpectedEOF},
			err:   "jsontext: read error: unexpected EOF",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dec := jsontext.NewDecoder(tc.input)
			var s promapi.SampleTimestampValue
			err := s.UnmarshalJSONFrom(dec)
			require.EqualError(t, err, tc.err)
		})
	}
}
