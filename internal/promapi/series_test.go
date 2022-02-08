package promapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestSeries(t *testing.T) {
	done := sync.Map{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match := r.URL.Query()[("match[]")]

		switch strings.Join(match, ", ") {
		case "empty":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status": "success",
				"data": []
			}`))
		case "single":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status": "success",
				"data": [{"__name__":"single", "foo": "bar"}]
			}`))
		case "two":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status": "success",
				"data": [
					{"__name__":"two", "foo": "bar"},
					{"__name__":"two", "foo": "baz"}
				]
			}`))
		case "single, two":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status": "success",
				"data": [
					{"__name__":"single", "foo": "bar"},
					{"__name__":"two", "foo": "bar"},
					{"__name__":"two", "foo": "baz"}
				]
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
		case "once":
			if _, wasDone := done.Load(match); wasDone {
				w.WriteHeader(500)
				_, _ = w.Write([]byte("query already requested\n"))
				return
			}
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status": "success",
				"data": [
					{"__name__":"once", "foo": "bar"},
					{"__name__":"once", "foo": "baz"}
				]
			}`))
			done.Store(match, true)
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
		matches []string
		timeout time.Duration
		series  []model.LabelSet
		err     *regexp.Regexp
		runs    int
	}

	testCases := []testCaseT{
		{
			matches: []string{"empty"},
			timeout: time.Second,
			series:  []model.LabelSet{},
			runs:    5,
		},
		{
			matches: []string{"single"},
			timeout: time.Second,
			series: []model.LabelSet{
				{model.MetricNameLabel: "single", "foo": "bar"},
			},
			runs: 5,
		},
		{
			matches: []string{"two"},
			timeout: time.Second,
			series: []model.LabelSet{
				{model.MetricNameLabel: "two", "foo": "bar"},
				{model.MetricNameLabel: "two", "foo": "baz"},
			},
			runs: 5,
		},
		{
			matches: []string{"single", "two"},
			timeout: time.Second,
			series: []model.LabelSet{
				{model.MetricNameLabel: "single", "foo": "bar"},
				{model.MetricNameLabel: "two", "foo": "bar"},
				{model.MetricNameLabel: "two", "foo": "baz"},
			},
			runs: 5,
		},
		{
			matches: []string{"timeout"},
			timeout: time.Millisecond * 50,
			series:  []model.LabelSet{},
			runs:    5,
			err:     regexp.MustCompile(`Get "http.+\/api\/v1\/series\?end=.+&match%5B%5D=timeout&start=.+": context deadline exceeded`),
		},
		{
			matches: []string{"error"},
			timeout: time.Second,
			series:  []model.LabelSet{},
			runs:    5,
			err:     regexp.MustCompile("unhandled query"),
		},
	}

	for _, tc := range testCases {
		t.Run(strings.Join(tc.matches, ","), func(t *testing.T) {
			assert := assert.New(t)

			prom := promapi.NewPrometheus("test", srv.URL, tc.timeout)

			wg := sync.WaitGroup{}
			wg.Add(tc.runs)
			for i := 1; i <= tc.runs; i++ {
				go func() {
					m, err := prom.Series(context.Background(), tc.matches)
					if tc.err != nil {
						assert.Error(err)
						assert.True(tc.err.MatchString(err.Error()))
					} else {
						assert.NoError(err)
					}
					if m != nil {
						assert.Equal(tc.series, m)
					}
					wg.Done()
				}()
			}
			wg.Wait()
		})
	}
}
