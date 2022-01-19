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
	//done := map[string]struct{}{}

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
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		case "single_result":
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
		case "vector":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "slow":
			w.WriteHeader(200)
			time.Sleep(time.Second)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		case "too_many_samples":
			start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
			end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
			diff := time.Unix(int64(end), 0).Sub(time.Unix(int64(start), 0))
			t.Log(diff.String())
			switch diff.String() {
			case "168h0m0s", "84h0m0s", "42h0m0s", "21h0m0s", "10h30m0s", "5h15m0s", "2h37m30s", "1h18m45s", "39m23s", "19m42s":
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
		case "retry_until_success":
			start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
			end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
			diff := time.Unix(int64(end), 0).Sub(time.Unix(int64(start), 0))
			t.Log(diff.String())
			switch diff.String() {
			case "168h0m0s", "84h0m0s", "42h0m0s", "21h0m0s", "10h30m0s", "5h15m0s", "2h37m30s", "1h18m45s", "19m42s":
				w.WriteHeader(422)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"status":"error",
					"errorType":"execution",
					"error":"query processing would load too many samples into memory in query execution"
				}`))
			case "39m23s":
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
				t.Errorf("invalid too_many_samples diff: %s", diff)
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
			query: "empty",
			args: func() (time.Time, time.Time, time.Duration) {
				return now, now, time.Minute
			},
			timeout: time.Second,
			samples: []*model.SampleStream{},
			runs:    5,
		},
		// cache miss
		{
			query: "empty",
			args: func() (time.Time, time.Time, time.Duration) {
				return time.Now(), time.Now(), time.Minute
			},
			timeout: time.Second,
			samples: []*model.SampleStream{},
			runs:    5,
		},
		// cache hit
		{
			query: "single_result",
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
			query: "single_result",
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
			query: "error",
			args: func() (time.Time, time.Time, time.Duration) {
				return now, now, time.Minute
			},
			timeout: time.Second,
			err:     "bad_data: unhandled path",
			runs:    5,
		},
		// cache miss
		{
			query: "error",
			args: func() (time.Time, time.Time, time.Duration) {
				return time.Now(), time.Now(), time.Minute
			},
			timeout: time.Second,
			err:     "bad_data: unhandled path",
			runs:    5,
		},
		// cache hit
		{
			query: "vector",
			args: func() (time.Time, time.Time, time.Duration) {
				return now, now, time.Minute
			},
			timeout: time.Second,
			err:     "unknown result type: vector",
			runs:    5,
		},
		// cache miss
		{
			query: "vector",
			args: func() (time.Time, time.Time, time.Duration) {
				return time.Now(), time.Now(), time.Minute
			},
			timeout: time.Second,
			err:     "unknown result type: vector",
			runs:    5,
		},
		// give up after all the retries
		{
			query: "too_many_samples",
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
			query: "slow",
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
			query: "retry_until_success",
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
			query: "retry_until_success",
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
