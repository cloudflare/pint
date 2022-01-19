package checks_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestVectorMatchingCheck(t *testing.T) {
	// zerolog.SetGlobalLevel(zerolog.FatalLevel)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Fatal(err)
		}
		query := r.Form.Get("query")

		switch query {
		case "count(foo / ignoring(notfound) foo)",
			"count(foo_with_notfound / ignoring(notfound) foo_with_notfound)",
			`count({__name__="foo_with_notfound"} / ignoring(notfound) foo_with_notfound)`,
			"count(foo_with_notfound / ignoring(notfound) foo)",
			"count(foo / ignoring(notfound) foo_with_notfound)":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "count(foo_with_notfound / bar)", "count(xxx / foo)", "count(foo / xxx)", "count(missing / foo)", "count(foo / missing)", "count(foo / ignoring(xxx) app_registry)", "count(foo / on(notfound) bar)", "count(memory_bytes / ignoring(job) (memory_limit > 0))", "count((memory_bytes / ignoring(job) (memory_limit > 0)) * on(app_name) group_left(a,b,c) app_registry)", "count(foo / on(notfound) bar_with_notfound)", "count(foo_with_notfound / on(notfound) bar)", "count(foo_with_notfound / ignoring(notfound) bar)":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[]
				}
			}`))
		case "topk(1, foo)":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{"__name__": "foo", "instance": "instance1", "job": "foo_job"},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "topk(1, bar)":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{"__name__": "bar", "instance": "instance1", "job": "bar_job"},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "topk(1, foo_with_notfound)":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{"__name__": "foo", "instance": "instance1", "job": "foo_job", "notfound": "xxx"},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "topk(1, bar_with_notfound)":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{"__name__": "bar", "instance": "instance1", "job": "bar_job", "notfound": "xxx"},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "topk(1, (memory_bytes / ignoring(job) (memory_limit > 0)))":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{"instance": "instance1", "job": "foo_job"},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "topk(1, memory_bytes)":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{"__name__": "memory_bytes", "instance": "instance1", "job": "foo_job"},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "topk(1, (memory_limit > 0))":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{"instance": "instance1", "job": "bar_job"},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "topk(1, app_registry)":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{"__name__": "app_registry", "app_name": "foo"},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "topk(1, missing)", "topk(1, xxx)":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[]
				}
			}`))
		case "topk(1, vector(0))":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"0"]}]
				}
			}`))
		default:
			w.WriteHeader(400)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"error",
				"errorType":"bad_data",
				"error":"unhandled query"
			}`))
			t.FailNow()
		}
	}))
	defer srv.Close()

	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "one to one matching",
			content:     "- record: foo\n  expr: foo_with_notfound / bar\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
			problems: []checks.Problem{
				{
					Fragment: "foo_with_notfound / bar",
					Lines:    []int{2},
					Reporter: "promql/vector_matching",
					Text:     `both sides of the query have different labels: [instance job notfound] != [instance job]`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "ignore left query errors",
			content:     "- record: foo\n  expr: xxx / foo\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "ignore righ query errors",
			content:     "- record: foo\n  expr: foo / xxx\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "ignore missing left series",
			content:     "- record: foo\n  expr: missing / foo\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "ignore missing or vector",
			content:     "- record: foo\n  expr: sum(missing or vector(0))\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "ignore present or vector",
			content:     "- record: foo\n  expr: sum(foo or vector(0))\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "ignore missing right series",
			content:     "- record: foo\n  expr: foo / missing\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "ignore with mismatched series",
			content:     "- record: foo\n  expr: foo / ignoring(xxx) app_registry\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
			problems: []checks.Problem{
				{
					Fragment: "foo / ignoring(xxx) app_registry",
					Lines:    []int{2},
					Reporter: "promql/vector_matching",
					Text:     `using ignoring("xxx") won't produce any results because both sides of the query have different labels: [instance job] != [app_name]`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "one to one matching with on() - both missing",
			content:     "- record: foo\n  expr: foo / on(notfound) bar\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
			problems: []checks.Problem{
				{
					Fragment: "foo / on(notfound) bar",
					Lines:    []int{2},
					Reporter: "promql/vector_matching",
					Text:     `using on("notfound") won't produce any results because both sides of the query don't have this label`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "one to one matching with ignoring() - both missing",
			content:     "- record: foo\n  expr: foo / ignoring(notfound) foo\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "one to one matching with ignoring() - both present",
			content:     "- record: foo\n  expr: foo_with_notfound / ignoring(notfound) foo_with_notfound\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "one to one matching with ignoring() - left missing",
			content:     "- record: foo\n  expr: foo / ignoring(notfound) foo_with_notfound\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "one to one matching with ignoring() - right missing",
			content:     "- record: foo\n  expr: foo_with_notfound / ignoring(notfound) foo\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "one to one matching with ignoring() - mismatch",
			content:     "- record: foo\n  expr: foo_with_notfound / ignoring(notfound) bar\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
		{
			description: "one to one matching with on() - left missing",
			content:     "- record: foo\n  expr: foo / on(notfound) bar_with_notfound\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
			problems: []checks.Problem{
				{
					Fragment: "foo / on(notfound) bar_with_notfound",
					Lines:    []int{2},
					Reporter: "promql/vector_matching",
					Text:     `using on("notfound") won't produce any results because left hand side of the query doesn't have this label: "foo"`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "one to one matching with on() - right missing",
			content:     "- record: foo\n  expr: foo_with_notfound / on(notfound) bar\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
			problems: []checks.Problem{
				{
					Fragment: "foo_with_notfound / on(notfound) bar",
					Lines:    []int{2},
					Reporter: "promql/vector_matching",
					Text:     `using on("notfound") won't produce any results because right hand side of the query doesn't have this label: "bar"`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "nested query",
			content:     "- alert: foo\n  expr: (memory_bytes / ignoring(job) (memory_limit > 0)) * on(app_name) group_left(a,b,c) app_registry\n",
			checker:     checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
			problems: []checks.Problem{
				{
					Fragment: "(memory_bytes / ignoring(job) (memory_limit > 0)) * on(app_name) group_left(a,b,c) app_registry",
					Lines:    []int{2},
					Reporter: "promql/vector_matching",
					Text:     `using on("app_name") won't produce any results because left hand side of the query doesn't have this label: "(memory_bytes / ignoring(job) (memory_limit > 0))"`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "one to one matching with ignoring() - both present - {__name__=}",
			content: `
- record: foo
  expr: '{__name__="foo_with_notfound"} / ignoring(notfound) foo_with_notfound'
`,
			checker: checks.NewVectorMatchingCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*1)),
		},
	}
	runTests(t, testCases)
}
