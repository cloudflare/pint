package promapi_test

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/promapi"
)

func newAbsoluteRange(start, end time.Time, step time.Duration) absoluteRange {
	return absoluteRange{start: start, end: end, step: step}
}

type absoluteRange struct {
	start time.Time
	end   time.Time
	step  time.Duration
}

func (ar absoluteRange) Start() time.Time {
	return ar.start
}

func (ar absoluteRange) End() time.Time {
	return ar.end
}

func (ar absoluteRange) Dur() time.Duration {
	return ar.end.Sub(ar.start)
}

func (ar absoluteRange) Step() time.Duration {
	return ar.step
}

func (ar absoluteRange) String() string {
	return fmt.Sprintf(
		"%s-%s/%s",
		ar.start.Format(time.RFC3339),
		ar.end.Format(time.RFC3339),
		output.HumanizeDuration(ar.step))
}

func TestRange(t *testing.T) {
	type testCaseT struct {
		start   time.Time
		end     time.Time
		mock    httpmock.Mocker
		query   string
		err     string
		out     promapi.SeriesTimeRanges
		stats   promapi.QueryStats
		step    time.Duration
		timeout time.Duration
	}

	timeParse := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	printRange := func(tr []promapi.MetricTimeRange) string {
		var buf strings.Builder
		for _, r := range tr {
			fmt.Fprintf(&buf, "%s %s - %s\n", r.Labels, r.Start.UTC().Format(time.RFC3339), r.End.UTC().Format(time.RFC3339))
		}
		return buf.String()
	}

	testCases := []testCaseT{
		{
			query:   "1m",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T00:01:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			out:     promapi.SeriesTimeRanges{},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"matrix","result":[]}}`).
					UnlimitedTimes()
			}),
		},
		{
			query:   "5m",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T03:00:00Z"),
			step:    time.Minute * 5,
			timeout: time.Second,
			out: promapi.SeriesTimeRanges{
				Ranges: promapi.MetricTimeRanges{
					{
						Labels: labels.FromStrings("instance", "1"),
						Start:  timeParse("2022-06-14T01:00:00Z"),
						End:    timeParse("2022-06-14T01:04:59Z"),
					},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Run(func(r *http.Request) ([]byte, error) {
						_ = r.ParseForm()
						values := make([]string, 0, 1)
						values = append(values, fmt.Sprintf(`[%d.0,"1"]`, timeParse("2022-06-14T01:00:00Z").Unix()))
						return fmt.Appendf(nil, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]}]}}`, strings.Join(values, ",")), nil
					}).
					UnlimitedTimes()
			}),
		},
		{
			query:   "1h",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T01:00:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			out:     promapi.SeriesTimeRanges{},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"matrix","result":[]}}`).
					UnlimitedTimes()
			}),
		},
		{
			query:   "2h",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T02:00:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			out:     promapi.SeriesTimeRanges{},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"matrix","result":[]}}`).
					UnlimitedTimes()
			}),
		},
		{
			query:   "2h1m",
			start:   timeParse("2022-06-14T16:34:00Z"),
			end:     timeParse("2022-06-14T18:35:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			out: promapi.SeriesTimeRanges{
				Ranges: promapi.MetricTimeRanges{
					{
						Labels: labels.FromStrings("instance", "1"),
						Start:  timeParse("2022-06-14T16:34:00Z"),
						End:    timeParse("2022-06-14T18:35:59Z"),
					},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Run(func(r *http.Request) ([]byte, error) {
						_ = r.ParseForm()
						var values []string
						for i := float64(timeParse("2022-06-14T16:34:00Z").Unix()); i <= float64(timeParse("2022-06-14T18:35:00Z").Unix()); i += 60 {
							values = append(values, fmt.Sprintf(`[%3f,"1"]`, i))
						}
						return fmt.Appendf(nil, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]}]}}`, strings.Join(values, ",")), nil
					}).
					UnlimitedTimes()
			}),
		},
		{
			query:   "3h",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T03:00:00Z"),
			step:    time.Minute * 5,
			timeout: time.Second,
			out: promapi.SeriesTimeRanges{
				Ranges: promapi.MetricTimeRanges{
					{
						Labels: labels.FromStrings("instance", "1"),
						Start:  timeParse("2022-06-14T00:00:00Z"),
						End:    timeParse("2022-06-14T02:59:59Z"),
					},
					{
						Labels: labels.FromStrings("instance", "2"),
						Start:  timeParse("2022-06-14T00:00:00Z"),
						End:    timeParse("2022-06-14T02:59:59Z"),
					},
					{
						Labels: labels.FromStrings("instance", "3"),
						Start:  timeParse("2022-06-14T00:00:00Z"),
						End:    timeParse("2022-06-14T02:59:59Z"),
					},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Run(func(r *http.Request) ([]byte, error) {
						_ = r.ParseForm()
						start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
						end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
						var values []string
						for i := start; i < end; i += 300 {
							values = append(values, fmt.Sprintf(`[%3f,"1"]`, i))
						}
						return fmt.Appendf(nil,
							`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]},{"metric":{"instance":"2"}, "values":[%s]},{"metric":{"instance":"3"}, "values":[%s]}]}}`,
							strings.Join(values, ","), strings.Join(values, ","), strings.Join(values, ","),
						), nil
					}).
					UnlimitedTimes()
			}),
		},
		{
			query:   "gap",
			start:   time.Unix(1677780240, 0),
			end:     time.Unix(1677786840, 0),
			step:    time.Minute * 5,
			timeout: time.Second,
			out: promapi.SeriesTimeRanges{
				Ranges: promapi.MetricTimeRanges{
					{
						Labels: labels.EmptyLabels(),
						Start:  timeParse("2023-03-02T18:04:00Z"),
						End:    timeParse("2023-03-02T19:33:59Z"),
					},
					// last sample is for 2023-03-02T19:29:00Z
					// gap at for 2023-03-02T19:34:00Z
					{
						Labels: labels.EmptyLabels(),
						Start:  timeParse("2023-03-02T19:39:00Z"),
						End:    timeParse("2023-03-02T19:58:59Z"),
					},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Return(`{
						"status":"success",
						"data":{
							"resultType":"matrix",
							"result":[
								{
									"metric":{},
									"values":[
										[1677780240,"1"],
										[1677780540,"1"],
										[1677780840,"1"],
										[1677781140,"1"],
										[1677781440,"1"],
										[1677781740,"1"],
										[1677782040,"1"],
										[1677782340,"1"],
										[1677782640,"1"],
										[1677782940,"1"],
										[1677783240,"1"],
										[1677783540,"1"],
										[1677783840,"1"],
										[1677784140,"1"],
										[1677784440,"1"],
										[1677784740,"1"],
										[1677785040,"1"],
										[1677785340,"1"],

										[1677785940,"1"],
										[1677786240,"1"],
										[1677786540,"1"],
										[1677786840,"1"]
									]
								}
							]
						}
					}`).
					UnlimitedTimes()
			}),
		},
		{
			query:   "7h",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T07:00:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			out: promapi.SeriesTimeRanges{
				Ranges: promapi.MetricTimeRanges{
					{
						Labels: labels.FromStrings("instance", "1"),
						Start:  timeParse("2022-06-14T00:00:00Z"),
						End:    timeParse("2022-06-14T06:59:59Z"),
					},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Run(func(r *http.Request) ([]byte, error) {
						_ = r.ParseForm()
						start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
						end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
						var values []string
						for i := start; i < end; i += 60 {
							values = append(values, fmt.Sprintf(`[%3f,"1"]`, i))
						}
						return fmt.Appendf(nil,
							`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]}]}}`,
							strings.Join(values, ","),
						), nil
					}).
					UnlimitedTimes()
			}),
		},
		{
			query:   "7h30m",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T07:30:00Z"),
			step:    time.Minute * 5,
			timeout: time.Second,
			out: promapi.SeriesTimeRanges{
				Ranges: promapi.MetricTimeRanges{
					{
						Labels: labels.FromStrings("instance", "1"),
						Start:  timeParse("2022-06-14T00:00:00Z"),
						End:    timeParse("2022-06-14T07:29:59Z"),
					},
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Run(func(r *http.Request) ([]byte, error) {
						_ = r.ParseForm()
						start, _ := strconv.Atoi(r.Form.Get("start"))
						end, _ := strconv.Atoi(r.Form.Get("end"))
						var values []string
						for i := start; i < end; i += 300 {
							values = append(values, fmt.Sprintf(`[%d,"1"]`, i))
						}
						return fmt.Appendf(nil,
							`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]}]}}`,
							strings.Join(values, ","),
						), nil
					}).
					UnlimitedTimes()
			}),
		},
		{
			query:   "3h/timeout",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T03:00:00Z"),
			step:    time.Minute * 5,
			timeout: time.Second,
			err:     "timeout: query timed out in expression evaluation",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Run(func(r *http.Request) ([]byte, error) {
						_ = r.ParseForm()
						start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
						end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
						var values []string
						for i := start; i <= end; i += 300 {
							values = append(values, fmt.Sprintf(`[%3f,"1"]`, i))
						}
						return fmt.Appendf(nil,
							`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]}]}}`,
							strings.Join(values, ","),
						), nil
					}).
					Once()
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnCode(http.StatusServiceUnavailable).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"timeout","error":"query timed out in expression evaluation"}`).
					UnlimitedTimes()
			}),
		},
		{
			query:   "apiError",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T00:01:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			err:     "bad_data: custom error message",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data","error":"custom error message"}`).
					UnlimitedTimes()
			}),
		},
		{
			query:   "badJson",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T00:01:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			err:     `bad_response: JSON parse error: jsontext: invalid character '}' after object name (expecting ':') within "/data/resultType" after offset 40`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType"}}`).
					UnlimitedTimes()
			}),
		},
		{
			query:   "emptyError",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T00:01:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			err:     `bad_data: empty response object`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data"}`).
					UnlimitedTimes()
			}),
		},
		{
			query:   "vector",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T00:05:00Z"),
			step:    time.Second,
			timeout: time.Second,
			err:     "bad_response: invalid result type, expected matrix, got vector",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1614859502.068,"1"]}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			query:   "stats",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T07:00:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			out: promapi.SeriesTimeRanges{
				Ranges: promapi.MetricTimeRanges{
					{
						Labels: labels.FromStrings("instance", "1"),
						Start:  timeParse("2022-06-14T00:00:00Z"),
						End:    timeParse("2022-06-14T06:59:59Z"),
					},
				},
			},
			stats: promapi.QueryStats{
				Timings: promapi.QueryTimings{
					EvalTotalTime:        10.1 * 4,
					ResultSortTime:       0.5 * 4,
					QueryPreparationTime: 1.5 * 4,
					InnerEvalTime:        0.7 * 4,
					ExecQueueTime:        0.01 * 4,
					ExecTotalTime:        5.1 * 4,
				},
				Samples: promapi.QuerySamples{
					TotalQueryableSamples: 1000 * 4,
					PeakSamples:           500 * 4,
				},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectPost(promapi.APIPathQueryRange).
					ReturnHeader("Content-Type", "application/json").
					Run(func(r *http.Request) ([]byte, error) {
						_ = r.ParseForm()
						start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
						end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
						var values []string
						for i := start; i < end; i += 60 {
							values = append(values, fmt.Sprintf(`[%3f,"1"]`, i))
						}
						return fmt.Appendf(nil,
							`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]}],"stats":{"timings":{"evalTotalTime":10.1,"resultSortTime":0.5,"queryPreparationTime":1.5,"innerEvalTime":0.7,"execQueueTime":0.01,"execTotalTime":5.1},"samples":{"totalQueryableSamples":1000,"peakSamples":500}}}}`,
							strings.Join(values, ","),
						), nil
					}).
					UnlimitedTimes()
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			srv := tc.mock(t)

			fg := promapi.NewFailoverGroup("test", srv.URL(), []*promapi.Prometheus{
				promapi.NewPrometheus("test", srv.URL(), "", nil, tc.timeout, 1, 100, nil),
			}, true, "up", nil, nil, nil)
			reg := prometheus.NewRegistry()
			fg.StartWorkers(reg)
			defer fg.Close(reg)

			for i := 1; i < 5; i++ {
				t.Run(tc.query, func(t *testing.T) {
					qr, err := fg.RangeQuery(t.Context(), tc.query, newAbsoluteRange(tc.start, tc.end, tc.step))
					if tc.err != "" {
						require.EqualError(t, err, tc.err, tc)
					} else {
						require.NoError(t, err)
						require.Equal(t, printRange(tc.out.Ranges), printRange(qr.Series.Ranges), tc)
						require.Equal(t, tc.stats, qr.Stats)
					}
				})
			}
		})
	}
}

func generateSamples(start, end time.Time, step time.Duration) (samples []model.SamplePair) {
	for {
		samples = append(samples, model.SamplePair{Timestamp: model.TimeFromUnix(start.Unix()), Value: 1})
		start = start.Add(step)
		if start.After(end) {
			break
		}
	}
	return samples
}
