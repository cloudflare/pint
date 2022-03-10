package checks_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
)

type newCheckFn func(string) checks.RuleChecker

type problemsFn func(string) []checks.Problem

type checkTestT struct {
	description string
	content     string
	checker     newCheckFn
	problems    problemsFn
	mocks       []prometheusMock
}

func runTestsT(t *testing.T, testCases []checkTestT, opts ...cmp.Option) {
	p := parser.NewParser()
	brokenRules, err := p.Parse([]byte(`
- alert: foo
  expr: 'foo{}{} > 0'
  annotations:
    summary: '{{ $labels.job }} is incorrect'

- record: foo
  expr: 'foo{}{}'
`))
	require.NoError(t, err, "failed to parse broken test rules")

	ctx := context.Background()
	for _, tc := range testCases {
		// original test
		t.Run(tc.description, func(t *testing.T) {
			var uri string
			if len(tc.mocks) > 0 {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					for i := range tc.mocks {
						if tc.mocks[i].maybeApply(w, r) {
							tc.mocks[i].wasUsed = true
							return
						}
					}
					t.Errorf("no matching response for %s request", r.URL.Path)
					t.FailNow()
				}))
				defer srv.Close()
				uri = srv.URL
			}

			rules, err := p.Parse([]byte(tc.content))
			require.NoError(t, err, "cannot parse rule content")
			for _, rule := range rules {
				problems := tc.checker(uri).Check(ctx, rule)
				require.Equal(t, tc.problems(uri), problems)
			}

			// verify that all mocks were used
			for _, mock := range tc.mocks {
				require.True(t, mock.wasUsed, "unused mock in %s: %s", tc.description, mock.conds)
			}
		})

		// broken rules to test check against rules with syntax error
		t.Run(tc.description+" (bogus rules)", func(t *testing.T) {
			for _, rule := range brokenRules {
				_ = tc.checker("").Check(ctx, rule)
			}
		})
	}
}

func noProblems(uri string) []checks.Problem {
	return nil
}

type requestCondition interface {
	isMatch(*http.Request) bool
}

type responseWriter interface {
	respond(w http.ResponseWriter)
}

type prometheusMock struct {
	conds   []requestCondition
	resp    responseWriter
	wasUsed bool
}

func (pm *prometheusMock) maybeApply(w http.ResponseWriter, r *http.Request) bool {
	for _, cond := range pm.conds {
		if !cond.isMatch(r) {
			return false
		}
	}
	pm.wasUsed = true
	pm.resp.respond(w)
	return true
}

type requestPathCond struct {
	path string
}

func (rpc requestPathCond) isMatch(r *http.Request) bool {
	return r.URL.Path == rpc.path
}

type formCond struct {
	key   string
	value string
}

func (fc formCond) isMatch(r *http.Request) bool {
	err := r.ParseForm()
	if err != nil {
		return false
	}
	return r.Form.Get(fc.key) == fc.value
}

var (
	// requireConfigPath     = requestPathCond{path: "/api/v1/config"}
	requireQueryPath      = requestPathCond{path: "/api/v1/query"}
	requireRangeQueryPath = requestPathCond{path: "/api/v1/query_range"}
)

type promError struct {
	code      int
	errorType v1.ErrorType
	err       string
}

func (pe promError) respond(w http.ResponseWriter) {
	w.WriteHeader(pe.code)
	w.Header().Set("Content-Type", "application/json")
	perr := struct {
		Status    string       `json:"status"`
		ErrorType v1.ErrorType `json:"errorType"`
		Error     string       `json:"error"`
	}{
		Status:    "error",
		ErrorType: pe.errorType,
		Error:     pe.err,
	}
	d, err := json.MarshalIndent(perr, "", "  ")
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(d)
}

type vectorResponse struct {
	samples model.Vector
}

func (vr vectorResponse) respond(w http.ResponseWriter) {
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	result := struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string       `json:"resultType"`
			Result     model.Vector `json:"result"`
		} `json:"data"`
	}{
		Status: "success",
		Data: struct {
			ResultType string       `json:"resultType"`
			Result     model.Vector `json:"result"`
		}{
			ResultType: "vector",
			Result:     vr.samples,
		},
	}
	d, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(d)
}

type matrixResponse struct {
	samples []*model.SampleStream
}

func (mr matrixResponse) respond(w http.ResponseWriter) {
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "application/json")
	result := struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string                `json:"resultType"`
			Result     []*model.SampleStream `json:"result"`
		} `json:"data"`
	}{
		Status: "success",
		Data: struct {
			ResultType string                `json:"resultType"`
			Result     []*model.SampleStream `json:"result"`
		}{
			ResultType: "matrix",
			Result:     mr.samples,
		},
	}
	d, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(d)
}

var (
	respondWithBadData             = promError{code: 400, errorType: v1.ErrBadData, err: "bad input data"}
	respondWithInternalError       = promError{code: 500, errorType: v1.ErrServer, err: "internal error"}
	respondWithEmptyVector         = vectorResponse{samples: model.Vector{}}
	respondWithEmptyMatrix         = matrixResponse{samples: []*model.SampleStream{}}
	respondWithSingleInstantVector = vectorResponse{
		samples: generateVector(map[string]string{}),
	}
	respondWithSingleRangeVector1W = matrixResponse{
		samples: []*model.SampleStream{
			generateSampleStream(
				map[string]string{},
				time.Now().Add(time.Hour*24*-7),
				time.Now(),
				time.Minute*5,
			),
		},
	}
)

func generateVector(labels map[string]string) (v model.Vector) {
	metric := model.Metric{}
	for k, v := range labels {
		metric[model.LabelName(k)] = model.LabelValue(v)
	}
	v = append(v, &model.Sample{
		Metric:    metric,
		Value:     model.SampleValue(1),
		Timestamp: model.TimeFromUnix(time.Now().Unix()),
	})
	return
}

func generateSampleStream(labels map[string]string, from, until time.Time, step time.Duration) (s *model.SampleStream) {
	metric := model.Metric{}
	for k, v := range labels {
		metric[model.LabelName(k)] = model.LabelValue(v)
	}
	s = &model.SampleStream{
		Metric: metric,
	}
	for from.Before(until) {
		s.Values = append(s.Values, model.SamplePair{
			Timestamp: model.TimeFromUnix(from.Unix()),
			Value:     1,
		})
		from = from.Add(step)
	}
	return
}

func checkErrorBadData(name, uri, err string) string {
	return fmt.Sprintf(`prometheus %q at %s failed with: %s`, name, uri, err)
}

func checkErrorUnableToRun(c, name, uri, err string) string {
	return fmt.Sprintf(`cound't run %q checks due to prometheus %q at %s connection error: %s`, c, name, uri, err)
}
