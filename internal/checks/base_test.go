package checks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestParseSeverity(t *testing.T) {
	type testCaseT struct {
		input       string
		output      string
		shouldError bool
	}

	testCases := []testCaseT{
		{"xxx", "", true},
		{"Bug", "", true},
		{"fatal", "Fatal", false},
		{"bug", "Bug", false},
		{"info", "Information", false},
		{"warning", "Warning", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			sev, err := checks.ParseSeverity(tc.input)
			hadError := err != nil

			if hadError != tc.shouldError {
				t.Fatalf("checks.ParseSeverity() returned err=%v, expected=%v", err, tc.shouldError)
			}

			if hadError {
				return
			}

			if sev.String() != tc.output {
				t.Fatalf("checks.ParseSeverity() returned severity=%q, expected=%q", sev, tc.output)
			}
		})
	}
}

func simpleProm(name, uri string, timeout time.Duration, required bool) *promapi.FailoverGroup {
	return promapi.NewFailoverGroup(
		name,
		uri,
		[]*promapi.Prometheus{
			promapi.NewPrometheus(name, uri, "", map[string]string{"X-Debug": "1"}, timeout, 16, 1000, nil),
		},
		required,
		"up",
		[]*regexp.Regexp{},
		[]*regexp.Regexp{},
		[]string{"mytag"},
	)
}

func newSimpleProm(uri string) *promapi.FailoverGroup {
	return simpleProm("prom", uri, time.Second*5, true)
}

func noProm(_ string) *promapi.FailoverGroup {
	return nil
}

type newCheckFn func(*promapi.FailoverGroup) checks.RuleChecker

type problemsFn func(string) []checks.Problem

type newPrometheusFn func(string) *promapi.FailoverGroup

type newCtxFn func(context.Context, string) context.Context

type otherPromsFn func(string) []*promapi.FailoverGroup

type checkTest struct {
	description string
	content     string
	prometheus  newPrometheusFn
	otherProms  otherPromsFn
	ctx         newCtxFn
	checker     newCheckFn
	entries     []discovery.Entry
	problems    problemsFn
	mocks       []*prometheusMock
}

func runTests(t *testing.T, testCases []checkTest) {
	for _, tc := range testCases {
		// original test
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			var uri string
			if len(tc.mocks) > 0 {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					defer r.Body.Close()
					for i := range tc.mocks {
						if tc.mocks[i].maybeApply(w, r) {
							return
						}
					}
					buf, _ := io.ReadAll(r.Body)
					t.Errorf("no matching response for %s %s request: %s, body: %s", r.Method, r.URL, r.URL.Query(), string(buf))
					t.FailNow()
				}))
				defer srv.Close()
				uri = srv.URL
			}

			var proms []*promapi.FailoverGroup
			reg := prometheus.NewRegistry()
			prom := tc.prometheus(uri)
			if prom != nil {
				proms = append(proms, prom)
				prom.StartWorkers(reg)
				defer prom.Close(reg)
			}

			if tc.otherProms != nil {
				for _, op := range tc.otherProms(uri) {
					proms = append(proms, op)
					op.StartWorkers(reg)
					defer op.Close(reg)
				}
			}

			ctx, cancel := context.WithCancel(context.Background())
			entries, err := parseContent(tc.content)
			require.NoError(t, err, "cannot parse rule content")
			for _, entry := range entries {
				if tc.ctx != nil {
					ctx = tc.ctx(ctx, uri)
				}
				ctx = context.WithValue(ctx, promapi.AllPrometheusServers, proms)
				problems := tc.checker(prom).Check(ctx, entry.Path, entry.Rule, tc.entries)
				require.Equal(t, tc.problems(uri), problems)
			}
			cancel()

			// verify that all mocks were used
			for _, mock := range tc.mocks {
				require.True(t, mock.wasUsed(), "unused mock in %s: %s", tc.description, mock.conds)
			}
		})

		// broken rules to test check against rules with syntax error
		entries, err := parseContent(`
- alert: foo
  expr: 'foo{}{} > 0'
  annotations:
    summary: '{{ $labels.job }} is incorrect'

- record: foo
  expr: 'foo{}{}'
`)
		require.NoError(t, err, "cannot parse rule content")
		t.Run(tc.description+" (bogus rules)", func(_ *testing.T) {
			for _, entry := range entries {
				_ = tc.checker(newSimpleProm("prom")).Check(context.Background(), entry.Path, entry.Rule, tc.entries)
			}
		})
	}
}

func parseContent(content string) (entries []discovery.Entry, err error) {
	p := parser.NewParser(false, parser.PrometheusSchema)
	rules, err := p.Parse([]byte(content))
	if err != nil {
		return nil, err
	}

	for _, rule := range rules {
		entries = append(entries, discovery.Entry{
			Path: discovery.Path{
				Name:          "fake.yml",
				SymlinkTarget: "fake.yml",
			},
			ModifiedLines: rule.Lines.Expand(),
			Rule:          rule,
		})
	}

	return entries, nil
}

func mustParseContent(content string) (entries []discovery.Entry) {
	entries, err := parseContent(content)
	if err != nil {
		panic(err)
	}
	return entries
}

func noProblems(_ string) []checks.Problem {
	return nil
}

type requestCondition interface {
	isMatch(*http.Request) bool
}

type responseWriter interface {
	respond(w http.ResponseWriter, r *http.Request)
}

type prometheusMock struct {
	resp  responseWriter
	conds []requestCondition
	mu    sync.Mutex
	used  bool
}

func (pm *prometheusMock) maybeApply(w http.ResponseWriter, r *http.Request) bool {
	for _, cond := range pm.conds {
		if !cond.isMatch(r) {
			return false
		}
	}
	pm.markUsed()
	pm.resp.respond(w, r)
	return true
}

func (pm *prometheusMock) markUsed() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.used = true
}

func (pm *prometheusMock) wasUsed() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.used
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
	buf, _ := io.ReadAll(r.Body)
	defer func() {
		r.Body = io.NopCloser(bytes.NewBuffer(buf))
	}()

	r.Body = io.NopCloser(bytes.NewBuffer(buf))
	err := r.ParseForm()
	if err != nil {
		return false
	}
	return r.Form.Get(fc.key) == fc.value
}

var (
	requireConfigPath     = requestPathCond{path: promapi.APIPathConfig}
	requireFlagsPath      = requestPathCond{path: promapi.APIPathFlags}
	requireQueryPath      = requestPathCond{path: promapi.APIPathQuery}
	requireRangeQueryPath = requestPathCond{path: promapi.APIPathQueryRange}
	requireMetadataPath   = requestPathCond{path: promapi.APIPathMetadata}
)

type httpResponse struct {
	body string
	code int
}

func (hr httpResponse) respond(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(hr.code)
	_, _ = w.Write([]byte(hr.body))
}

type promError struct {
	errorType v1.ErrorType
	err       string
	code      int
}

func (pe promError) respond(w http.ResponseWriter, _ *http.Request) {
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
	stats   promapi.QueryStats
}

func (vr vectorResponse) respond(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	result := struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string             `json:"resultType"`
			Result     model.Vector       `json:"result"`
			Stats      promapi.QueryStats `json:"stats"`
		} `json:"data"`
	}{
		Status: "success",
		Data: struct {
			ResultType string             `json:"resultType"`
			Result     model.Vector       `json:"result"`
			Stats      promapi.QueryStats `json:"stats"`
		}{
			ResultType: "vector",
			Result:     vr.samples,
			Stats:      vr.stats,
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
	stats   promapi.QueryStats
}

func (mr matrixResponse) respond(w http.ResponseWriter, r *http.Request) {
	start, _ := strconv.ParseFloat(r.Form.Get("start"), 64)
	end, _ := strconv.ParseFloat(r.Form.Get("end"), 64)
	samples := []*model.SampleStream{}
	for _, s := range mr.samples {
		var values []model.SamplePair
		for _, v := range s.Values {
			ts := float64(v.Timestamp.Time().Unix())
			if ts >= start && ts <= end {
				values = append(values, v)
			}
		}
		if len(values) > 0 {
			samples = append(samples, &model.SampleStream{
				Metric: s.Metric,
				Values: values,
			})
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	result := struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string                `json:"resultType"`
			Result     []*model.SampleStream `json:"result"`
			Stats      promapi.QueryStats    `json:"stats"`
		} `json:"data"`
	}{
		Status: "success",
		Data: struct {
			ResultType string                `json:"resultType"`
			Result     []*model.SampleStream `json:"result"`
			Stats      promapi.QueryStats    `json:"stats"`
		}{
			ResultType: "matrix",
			Result:     samples,
			Stats:      mr.stats,
		},
	}
	d, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(d)
}

type configResponse struct {
	yaml string
}

func (cr configResponse) respond(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	result := struct {
		Status string          `json:"status"`
		Data   v1.ConfigResult `json:"data"`
	}{
		Status: "success",
		Data:   v1.ConfigResult{YAML: cr.yaml},
	}
	d, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(d)
}

type flagsResponse struct {
	flags map[string]string
}

func (fg flagsResponse) respond(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	result := struct {
		Data   v1.FlagsResult `json:"data"`
		Status string         `json:"status"`
	}{
		Status: "success",
		Data:   fg.flags,
	}
	d, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(d)
}

type metadataResponse struct {
	metadata map[string][]v1.Metadata
}

func (mr metadataResponse) respond(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	// _, _ = w.Write([]byte(`{"status":"success","data":{"gauge":[{"type":"gauge","help":"Text","unit":""}]}}`))
	result := struct {
		Data   map[string][]v1.Metadata `json:"data"`
		Status string                   `json:"status"`
	}{
		Status: "success",
		Data:   mr.metadata,
	}
	d, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(d)
}

type sleepResponse struct {
	resp  responseWriter
	sleep time.Duration
}

func (sr sleepResponse) respond(w http.ResponseWriter, r *http.Request) {
	time.Sleep(sr.sleep)
	if sr.resp != nil {
		sr.resp.respond(w, r)
	}
}

var (
	respondWithBadData = func() responseWriter {
		return promError{code: 400, errorType: v1.ErrBadData, err: "bad input data"}
	}
	respondWithInternalError = func() responseWriter {
		return promError{code: 500, errorType: v1.ErrServer, err: "internal error"}
	}
	respondWithTooManySamples = func() responseWriter {
		return promError{code: 422, errorType: v1.ErrExec, err: "query processing would load too many samples into memory in query execution"}
	}
	respondWithTimeoutExpandingSeriesSamples = func() responseWriter {
		return promError{code: 422, errorType: v1.ErrExec, err: "expanding series: context deadline exceeded"}
	}
	respondWithEmptyVector = func() responseWriter {
		return vectorResponse{samples: model.Vector{}}
	}
	respondWithEmptyMatrix = func() responseWriter {
		return matrixResponse{samples: []*model.SampleStream{}}
	}
	respondWithSingleInstantVector = func() responseWriter {
		return vectorResponse{
			samples: []*model.Sample{generateSample(map[string]string{})},
		}
	}
	respondWithSingleRangeVector1D = func() responseWriter {
		return matrixResponse{
			samples: []*model.SampleStream{
				generateSampleStream(
					map[string]string{},
					time.Now().Add(time.Hour*-24),
					time.Now(),
					time.Minute*5,
				),
			},
		}
	}
	respondWithSingleRangeVector1W = func() responseWriter {
		return matrixResponse{
			samples: []*model.SampleStream{
				generateSampleStream(
					map[string]string{},
					time.Now().Add(time.Hour*24*-7),
					time.Now(),
					time.Minute*5,
				),
			},
		}
	}
)

func generateSample(labels map[string]string) *model.Sample {
	metric := model.Metric{}
	for k, v := range labels {
		metric[model.LabelName(k)] = model.LabelValue(v)
	}
	return &model.Sample{
		Metric:    metric,
		Value:     model.SampleValue(1),
		Timestamp: model.TimeFromUnix(time.Now().Unix()),
	}
}

func generateSampleWithValue(labels map[string]string, val float64) *model.Sample {
	metric := model.Metric{}
	for k, v := range labels {
		metric[model.LabelName(k)] = model.LabelValue(v)
	}
	return &model.Sample{
		Metric:    metric,
		Value:     model.SampleValue(val),
		Timestamp: model.TimeFromUnix(time.Now().Unix()),
	}
}

func generateSampleStream(labels map[string]string, from, until time.Time, step time.Duration) (s *model.SampleStream) {
	if from.After(until) {
		panic(fmt.Sprintf("generateSampleStream() got from > until: %s ~ %s", from.UTC().Format(time.RFC3339), until.UTC().Format(time.RFC3339)))
	}
	metric := model.Metric{}
	for k, v := range labels {
		metric[model.LabelName(k)] = model.LabelValue(v)
	}
	s = &model.SampleStream{
		Metric: metric,
	}
	for !from.After(until) {
		s.Values = append(s.Values, model.SamplePair{
			Timestamp: model.TimeFromUnix(from.Unix()),
			Value:     1,
		})
		from = from.Add(step)
	}
	return s
}

func checkErrorBadData(name, uri, err string) string {
	return fmt.Sprintf("`%s` Prometheus server at %s failed with: `%s`.", name, uri, err)
}

func checkErrorUnableToRun(c, name, uri, err string) string {
	return fmt.Sprintf("Couldn't run `%s` checks due to `%s` Prometheus server at %s connection error: `%s`.", c, name, uri, err)
}

func checkErrorTooExpensiveToRun(c, name, uri, err string) string {
	return fmt.Sprintf("Couldn't run `%s` checks on `%s` Prometheus server at %s because some queries are too expensive: `%s`.", c, name, uri, err)
}

func checkUnsupported(c, name, uri, path string) string {
	return fmt.Sprintf("Couldn't run `%s` checks on `%s` Prometheus server at %s because it's unsupported: this server doesn't seem to support `%s` API endpoint.", c, name, uri, path)
}
