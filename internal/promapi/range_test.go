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
	"github.com/stretchr/testify/assert"
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
		samples []*model.SampleStream
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

	testCases := []testCaseT{
		{
			query:   "1m",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T00:01:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			samples: nil,
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
			samples: nil,
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
			samples: nil,
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
			samples: []*model.SampleStream{{
				Metric: model.Metric{"instance": "1"},
				Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T03:00:00Z"), time.Minute*5),
			}},
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
					require.Equal(t, float64(timeParse("2022-06-14T01:55:00Z").Unix()), end, "invalid end for #0")
				case float64(timeParse("2022-06-14T02:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T03:00:00Z").Unix()), end, "invalid end for #1")

				default:
					t.Fatalf("unknown start: %.2f", start)
				}

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				var values []string
				for i := start; i <= end; i += 300 {
					values = append(values, fmt.Sprintf(`[%3f,"1"]`, i))
				}
				_, _ = w.Write([]byte(fmt.Sprintf(
					`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"instance":"1"}, "values":[%s]}]}}`,
					strings.Join(values, ","))))
			},
		},
		{
			query:   "7h",
			start:   timeParse("2022-06-14T00:00:00Z"),
			end:     timeParse("2022-06-14T07:00:00Z"),
			step:    time.Minute,
			timeout: time.Second,
			samples: []*model.SampleStream{{
				Metric: model.Metric{"instance": "1"},
				Values: generateSamples(timeParse("2022-06-14T00:00:00Z"), timeParse("2022-06-14T07:00:00Z"), time.Minute),
			}},
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
					require.Equal(t, float64(timeParse("2022-06-14T01:59:00Z").Unix()), end, "invalid end for #0")
				case float64(timeParse("2022-06-14T02:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T03:59:00Z").Unix()), end, "invalid end for #1")
				case float64(timeParse("2022-06-14T04:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T05:59:00Z").Unix()), end, "invalid end for #2")
				case float64(timeParse("2022-06-14T06:00:00Z").Unix()):
					require.Equal(t, float64(timeParse("2022-06-14T07:00:00Z").Unix()), end, "invalid end for #3")
				default:
					t.Fatalf("unknown start: %.2f", start)
				}

				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				var values []string
				for i := start; i <= end; i += 60 {
					values = append(values, fmt.Sprintf(`[%3f,"1"]`, i))
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
					require.Equal(t, float64(timeParse("2022-06-14T01:55:00Z").Unix()), end, "invalid end for #0")
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
			err:     "unknown result type: vector",
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
			assert := assert.New(t)

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc.handler(t, w, r)
			}))
			defer srv.Close()

			prom := promapi.NewPrometheus("test", srv.URL, tc.timeout, 1)
			prom.StartWorkers()
			defer prom.Close()

			qr, err := prom.RangeQuery(context.Background(), tc.query, promapi.NewAbsoluteRange(tc.start, tc.end, tc.step))
			if tc.err != "" {
				assert.EqualError(err, tc.err, tc)
			} else {
				assert.NoError(err)
			}
			if qr != nil {
				assert.Equal(qr.Samples, tc.samples, tc)
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
