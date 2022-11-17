package promapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestRange(t *testing.T) {
	type handlerFunc func(t *testing.T, w http.ResponseWriter, r *http.Request)

	type testCaseT struct {
		query   string
		start   time.Time
		end     time.Time
		step    time.Duration
		timeout time.Duration
		out     promapi.SeriesTimeRanges
		err     string
		handler handlerFunc
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
			buf.WriteString(fmt.Sprintf("%s %s - %s\n", r.Labels, r.Start.UTC().Format(time.RFC3339), r.End.UTC().Format(time.RFC3339)))
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
			handler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				err := r.ParseForm()
				if err != nil {
					t.Fatal(err)
				}
				require.Equal(t, r.Form.Get("query"), "1m")
				require.Equal(t, r.Form.Get("step"), "60")

				start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
				require.Equal(t, float64(timeParse("2022-06-14T00:00:00Z").Unix()), start, "invalid start")

				end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
				require.Equal(t, float64(timeParse("2022-06-14T00:01:00Z").Unix()), end, "invalid end")

				diff := time.Unix(int64(end), 0).Sub(time.Unix(int64(start), 0))
				require.Equal(t, diff, time.Minute)

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
			},
		},
		{
			query:   "1h",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T01:00:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			out:     promapi.SeriesTimeRanges{},
			handler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				err := r.ParseForm()
				if err != nil {
					t.Fatal(err)
				}
				require.Equal(t, r.Form.Get("query"), "1h")
				require.Equal(t, r.Form.Get("step"), "60")

				start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
				require.Equal(t, float64(timeParse("2022-06-14T00:00:00Z").Unix()), start, "invalid start")

				end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
				require.Equal(t, float64(timeParse("2022-06-14T01:00:00Z").Unix()), end, "invalid end")

				diff := time.Unix(int64(end), 0).Sub(time.Unix(int64(start), 0))
				require.Equal(t, diff, time.Hour)

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
			},
		},
		{
			query:   "2h",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T02:00:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			out:     promapi.SeriesTimeRanges{},
			handler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				err := r.ParseForm()
				if err != nil {
					t.Fatal(err)
				}
				require.Equal(t, r.Form.Get("query"), "2h")
				require.Equal(t, r.Form.Get("step"), "60")

				start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
				require.Equal(t, float64(timeParse("2022-06-14T00:00:00Z").Unix()), start, "invalid start")

				end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
				require.Equal(t, float64(timeParse("2022-06-14T02:00:00Z").Unix()), end, "invalid end")

				diff := time.Unix(int64(end), 0).Sub(time.Unix(int64(start), 0))
				require.Equal(t, diff, time.Hour*2)

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
			},
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
						End:    timeParse("2022-06-14T03:00:00Z"),
					},
					{
						Labels: labels.FromStrings("instance", "2"),
						Start:  timeParse("2022-06-14T00:00:00Z"),
						End:    timeParse("2022-06-14T03:00:00Z"),
					},
					{
						Labels: labels.FromStrings("instance", "3"),
						Start:  timeParse("2022-06-14T00:00:00Z"),
						End:    timeParse("2022-06-14T03:00:00Z"),
					},
				},
			},
			handler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				err := r.ParseForm()
				if err != nil {
					t.Fatal(err)
				}
				require.Equal(t, r.Form.Get("query"), "3h")
				require.Equal(t, r.Form.Get("step"), "300")

				start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
				end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)

				switch start {
				case float64(timeParse("2022-06-14T00:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T01:59:59Z").Unix()), end, "invalid end for #0")
				case float64(timeParse("2022-06-14T02:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T03:00:00Z").Unix()), end, "invalid end for #1")

				default:
					t.Fatalf("unknown start: %.2f", start)
				}

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				var values []string
				for i := start; i < end; i += 300 {
					values = append(values, fmt.Sprintf(`[%3f,"1"]`, i))
				}
				_, _ = w.Write([]byte(fmt.Sprintf(
					`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]},{"metric":{"instance":"2"}, "values":[%s]},{"metric":{"instance":"3"}, "values":[%s]}]}}`,
					strings.Join(values, ","), strings.Join(values, ","), strings.Join(values, ","))))
			},
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
						End:    timeParse("2022-06-14T07:00:00Z"),
					},
				},
			},
			handler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				err := r.ParseForm()
				if err != nil {
					t.Fatal(err)
				}
				require.Equal(t, r.Form.Get("query"), "7h")
				require.Equal(t, r.Form.Get("step"), "60")

				start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
				end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)

				switch start {
				case float64(timeParse("2022-06-14T00:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T01:59:59Z").Unix()), end, "invalid end for #0")
				case float64(timeParse("2022-06-14T02:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T03:59:59Z").Unix()), end, "invalid end for #1")
				case float64(timeParse("2022-06-14T04:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T05:59:59Z").Unix()), end, "invalid end for #2")
				case float64(timeParse("2022-06-14T06:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T07:00:00Z").Unix()), end, "invalid end for #3")
				default:
					t.Fatalf("unknown start: %.2f", start)
				}

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				var values []string
				for i := start; i < end; i += 60 {
					values = append(values, fmt.Sprintf(`[%3f,"1"]`, i))
				}
				_, _ = w.Write([]byte(fmt.Sprintf(
					`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]}]}}`,
					strings.Join(values, ","))))
			},
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
						End:    timeParse("2022-06-14T07:30:00Z"),
					},
				},
			},
			handler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				err := r.ParseForm()
				if err != nil {
					t.Fatal(err)
				}
				require.Equal(t, r.Form.Get("query"), "7h30m")
				require.Equal(t, r.Form.Get("step"), "300")

				start, _ := strconv.Atoi(r.Form.Get("start"))
				end, _ := strconv.Atoi(r.Form.Get("end"))

				switch start {
				case int(timeParse("2022-06-14T00:00:00Z").Unix()):
					require.Equal(t, int(timeParse("2022-06-14T01:59:59Z").Unix()), end, "invalid end for #0")
				case int(timeParse("2022-06-14T02:00:00Z").Unix()):
					require.Equal(t, int(timeParse("2022-06-14T03:59:59Z").Unix()), end, "invalid end for #1")
				case int(timeParse("2022-06-14T04:00:00Z").Unix()):
					require.Equal(t, int(timeParse("2022-06-14T05:59:59Z").Unix()), end, "invalid end for #2")
				case int(timeParse("2022-06-14T06:00:00Z").Unix()):
					require.Equal(t, int(timeParse("2022-06-14T07:30:00Z").Unix()), end, "invalid end for #3")
				default:
					t.Fatalf("unknown start: %d", start)
				}

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				var values []string
				for i := start; i < end; i += 300 {
					values = append(values, fmt.Sprintf(`[%d,"1"]`, i))
				}
				_, _ = w.Write([]byte(fmt.Sprintf(
					`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]}]}}`,
					strings.Join(values, ","))))
			},
		},
		{
			query:   "3h/timeout",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T03:00:00Z"),
			step:    time.Minute * 5,
			timeout: time.Second,
			err:     "timeout: query timed out in expression evaluation",
			handler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				err := r.ParseForm()
				if err != nil {
					t.Fatal(err)
				}
				require.Equal(t, r.Form.Get("query"), "3h/timeout")
				require.Equal(t, r.Form.Get("step"), "300")

				start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
				end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)

				switch start {
				case float64(timeParse("2022-06-14T00:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T01:59:59Z").Unix()), end, "invalid end for #0")
					w.WriteHeader(200)
					w.Header().Set("Content-Type", "application/json")
					var values []string
					for i := start; i <= end; i += 300 {
						values = append(values, fmt.Sprintf(`[%3f,"1"]`, i))
					}
					_, _ = w.Write([]byte(fmt.Sprintf(
						`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]}]}}`,
						strings.Join(values, ","))))
				case float64(timeParse("2022-06-14T02:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T03:00:00Z").Unix()), end, "invalid end for #1")
					w.WriteHeader(503)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{
                                       "status": "error",
                                       "errorType": "timeout",
                                       "error": "query timed out in expression evaluation"
                               }`))
				default:
					t.Fatalf("unknown start: %.2f", start)
				}
			},
		},
		{
			query:   "vector",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T00:05:00Z"),
			step:    time.Second,
			timeout: time.Second,
			err:     "bad_response: invalid result type, expected matrix, got vector",
			handler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				err := r.ParseForm()
				if err != nil {
					t.Fatal(err)
				}
				require.Equal(t, r.Form.Get("query"), "vector")
				require.Equal(t, r.Form.Get("step"), "1")

				start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
				require.Equal(t, float64(timeParse("2022-06-14T00:00:00Z").Unix()), start, "invalid start")

				end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
				require.Equal(t, float64(timeParse("2022-06-14T00:05:00Z").Unix()), end, "invalid end")

				diff := time.Unix(int64(end), 0).Sub(time.Unix(int64(start), 0))
				require.Equal(t, diff, time.Minute*5)

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"status":"success",
					"data":{
						"resultType":"vector",
						"result":[{"metric":{},"value":[1614859502.068,"1"]}]
					}
				}`))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc.handler(t, w, r)
			}))
			defer srv.Close()

			fg := promapi.NewFailoverGroup("test", []*promapi.Prometheus{
				promapi.NewPrometheus("test", srv.URL, nil, tc.timeout, 1, 100),
			}, 1000, true, "up", nil, nil)
			fg.StartWorkers()
			defer fg.Close()

			for i := 1; i < 5; i++ {
				t.Run(tc.query, func(t *testing.T) {
					qr, err := fg.RangeQuery(context.Background(), tc.query, promapi.NewAbsoluteRange(tc.start, tc.end, tc.step))
					if tc.err != "" {
						require.EqualError(t, err, tc.err, tc)
					} else {
						require.NoError(t, err)
						require.Equal(t, printRange(tc.out.Ranges), printRange(qr.Series.Ranges), tc)
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
