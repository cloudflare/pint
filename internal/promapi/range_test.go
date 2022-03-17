package promapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestRange(t *testing.T) {
	done := sync.Map{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Fatal(err)
		}
		query := r.Form.Get("query")

		start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
		end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
		diff := time.Unix(int64(end), 0).Sub(time.Unix(int64(start), 0))
		t.Logf("query=%s diff=%s start=%s end=%s", query, diff, time.Unix(int64(start), 0), time.Unix(int64(end), 0))

		if diffs, ok := done.Load(query); ok {
			doneDiffs := diffs.([]string)
			for _, doneDiff := range doneDiffs {
				// some queries are allowed to re-run because they fail and never cache anything
				if doneDiff == diff.String() &&
					query != "too_many_samples1" &&
					query != "error1" && query != "error2" &&
					query != "vector1" && query != "vector2" &&
					query != "slow1" {
					t.Errorf("%q already requested diff=%s", query, diff)
					t.FailNow()
				}
			}
			doneDiffs = append(doneDiffs, diff.String())
			done.Store(query, doneDiffs)
		} else {
			done.Store(query, []string{diff.String()})
		}

		switch query {
		case "empty1", "empty2":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		case "single_result1", "single_result2":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[
				{"metric":{"instance":"1"},"values":[
					[1614859502.068,"0"],
					[1614859562.068,"1"],
					[1614859622.068,"3"],
					[1614859682.068,"4"],
					[1614859742.068,"11"]
				]}
			]}}`))
		case "vector1", "vector2":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "slow1":
			w.WriteHeader(200)
			time.Sleep(time.Second)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		case "too_many_samples1":
			switch diff.String() {
			case "168h0m0s", "42h0m0s", "10h30m0s", "2h38m0s", "40m0s", "10m0s":
				w.WriteHeader(422)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"status":"error",
					"errorType":"execution",
					"error":"query processing would load too many samples into memory in query execution"
				}`))
			default:
				t.Errorf("invalid too_many_samples diff: %s", diff)
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`unknown start/end`))
			}
		case "duplicate_series1":
			switch diff.String() {
			case "168h0m0s", "84h0m0s", "42h0m0s", "21h0m0s", "10h30m0s":
				w.WriteHeader(422)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"status":"error",
					"errorType":"execution",
					"error":"found duplicate series for the match group {...} on the right hand-side of the operation: [{...}, {...}];many-to-many matching not allowed: matching labels must be unique on one side"
				}`))
			case "5h15m0s":
				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[
							{"metric":{"instance":"1"},"values":[
								[1614859502.068,"0"]
							]}
						]}}`))
			default:
				t.Errorf("invalid duplicate_series diff: %s", diff)
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`unknown start/end`))
			}
		case "retry_until_success1", "retry_until_success2":
			switch diff.String() {
			case "168h0m0s", "42h0m0s", "10h30m0s", "2h38m0s":
				w.WriteHeader(422)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"status":"error",
					"errorType":"execution",
					"error":"query processing would load too many samples into memory in query execution"
				}`))
			case "40m0s":
				w.WriteHeader(200)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[
						{"metric":{"instance":"1"},"values":[
							[1614859502.068,"0"],
							[1614859562.068,"1"],
							[1614859622.068,"3"],
							[1614859682.068,"4"],
							[1614859742.068,"11"]
						]}
					]}}`))
			default:
				t.Errorf("invalid retry_until_success diff: %s", diff)
				w.WriteHeader(500)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`unknown start/end`))
			}
		default:
			w.WriteHeader(400)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"unhandled path"}`))
		}
	}))
	defer srv.Close()

	type argsFactory func() (time.Time, time.Time, time.Duration)

	type testCaseT struct {
		query   string
		args    argsFactory
		timeout time.Duration
		samples []*model.SampleStream
		err     string
		runs    int
	}

	now := time.Now()

	testCases := []testCaseT{
		// cache hit
		{
			query: "empty1",
			args: func() (time.Time, time.Time, time.Duration) {
				return now, now, time.Minute
			},
			timeout: time.Second,
			samples: []*model.SampleStream{},
			runs:    5,
		},
		// cache miss
		{
			query: "empty2",
			args: func() (time.Time, time.Time, time.Duration) {
				return time.Now(), time.Now(), time.Minute
			},
			timeout: time.Second,
			samples: []*model.SampleStream{},
			runs:    5,
		},
		// cache hit
		{
			query: "single_result1",
			args: func() (time.Time, time.Time, time.Duration) {
				return now, now, time.Minute
			},
			timeout: time.Second,
			samples: []*model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: []model.SamplePair{
						{Timestamp: 1614859502068, Value: 0},
						{Timestamp: 1614859562068, Value: 1},
						{Timestamp: 1614859622068, Value: 3},
						{Timestamp: 1614859682068, Value: 4},
						{Timestamp: 1614859742068, Value: 11},
					},
				},
			},
			runs: 5,
		},
		// cache miss
		{
			query: "single_result2",
			args: func() (time.Time, time.Time, time.Duration) {
				return time.Now(), time.Now(), time.Minute
			},
			timeout: time.Second,
			samples: []*model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: []model.SamplePair{
						{Timestamp: 1614859502068, Value: 0},
						{Timestamp: 1614859562068, Value: 1},
						{Timestamp: 1614859622068, Value: 3},
						{Timestamp: 1614859682068, Value: 4},
						{Timestamp: 1614859742068, Value: 11},
					},
				},
			},
			runs: 5,
		},
		// cache hit
		{
			query: "error1",
			args: func() (time.Time, time.Time, time.Duration) {
				return now, now, time.Minute
			},
			timeout: time.Second,
			err:     "bad_data: unhandled path",
			runs:    5,
		},
		// cache miss
		{
			query: "error2",
			args: func() (time.Time, time.Time, time.Duration) {
				return time.Now(), time.Now(), time.Minute
			},
			timeout: time.Second,
			err:     "bad_data: unhandled path",
			runs:    5,
		},
		// cache hit
		{
			query: "vector1",
			args: func() (time.Time, time.Time, time.Duration) {
				return now, now, time.Minute
			},
			timeout: time.Second,
			err:     "unknown result type: vector",
			runs:    5,
		},
		// cache miss
		{
			query: "vector2",
			args: func() (time.Time, time.Time, time.Duration) {
				return time.Now(), time.Now(), time.Minute
			},
			timeout: time.Second,
			err:     "unknown result type: vector",
			runs:    5,
		},
		// give up after all the retries
		{
			query: "too_many_samples1",
			args: func() (time.Time, time.Time, time.Duration) {
				start := time.Unix(1577836800, 0)
				end := time.Unix(1578441600, 0)
				return start, end, time.Minute * 5
			},
			timeout: time.Second,
			err:     "no more retries possible",
			runs:    5,
		},
		// retry timeouts
		{
			query: "slow1",
			args: func() (time.Time, time.Time, time.Duration) {
				start := time.Unix(1577836800, 0)
				end := time.Unix(1578441600, 0)
				return start, end, time.Minute * 5
			},
			timeout: time.Millisecond * 20,
			err:     "no more retries possible",
			runs:    5,
		},
		// cache hit
		{
			query: "retry_until_success1",
			args: func() (time.Time, time.Time, time.Duration) {
				start := time.Unix(1577836800, 0)
				end := time.Unix(1578441600, 0)
				return start, end, time.Minute * 5
			},
			timeout: time.Second,
			samples: []*model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: []model.SamplePair{
						{Timestamp: 1614859502068, Value: 0},
						{Timestamp: 1614859562068, Value: 1},
						{Timestamp: 1614859622068, Value: 3},
						{Timestamp: 1614859682068, Value: 4},
						{Timestamp: 1614859742068, Value: 11},
					},
				},
			},
			runs: 5,
		},
		// cache miss
		{
			query: "retry_until_success2",
			args: func() (time.Time, time.Time, time.Duration) {
				start := time.Unix(1577836800, 0)
				end := time.Unix(1578441600, 0)
				return start, end, time.Minute * 5
			},
			timeout: time.Second,
			samples: []*model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: []model.SamplePair{
						{Timestamp: 1614859502068, Value: 0},
						{Timestamp: 1614859562068, Value: 1},
						{Timestamp: 1614859622068, Value: 3},
						{Timestamp: 1614859682068, Value: 4},
						{Timestamp: 1614859742068, Value: 11},
					},
				},
			},
			runs: 5,
		},
		// duplicate series
		{
			query: "duplicate_series1",
			args: func() (time.Time, time.Time, time.Duration) {
				start := time.Unix(1577836800, 0)
				end := time.Unix(1578441600, 0)
				return start, end, time.Minute * 5
			},
			timeout: time.Second,
			samples: []*model.SampleStream{
				{
					Metric: model.Metric{"instance": "1"},
					Values: []model.SamplePair{
						{Timestamp: 1614859502068, Value: 0},
					},
				},
			}, runs: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			assert := assert.New(t)

			prom := promapi.NewPrometheus("test", srv.URL, tc.timeout)

			wg := sync.WaitGroup{}
			wg.Add(tc.runs)
			for i := 1; i <= tc.runs; i++ {
				go func() {
					start, end, step := tc.args()
					qr, err := prom.RangeQuery(context.Background(), tc.query, start, end, step)
					if tc.err != "" {
						assert.EqualError(err, tc.err, tc)
					} else {
						assert.NoError(err)
					}
					if qr != nil {
						assert.Equal(qr.Samples, tc.samples, tc)
					}
					wg.Done()
				}()
			}
			wg.Wait()
		})
	}
}
