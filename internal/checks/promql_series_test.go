package checks_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"

	"github.com/rs/zerolog"
)

func TestSeriesCheck(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Fatal(err)
		}
		query := r.Form.Get("query")

		switch query {
		case "count(notfound)", `count(notfound{job="foo"})`, `count(notfound{job!="foo"})`, `count({__name__="notfound",job="bar"})`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[]
				}
			}`))
		case "count(found_1)", `count({__name__="notfound"})`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
				}
			}`))
		case "count(found_7)":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"7"]}]
				}
			}`))
		case `count(node_filesystem_readonly{mountpoint!=""})`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
				}
			}`))
		case `count(disk_info{interface_speed!="6.0 Gb/s",type="sat"})`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
				}
			}`))
		case `count(found{job="notfound"})`, `count(notfound{job="notfound"})`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[]
				}
			}`))
		case `count(found)`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
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
		}
	}))
	defer srv.Close()

	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
		},
		{
			description: "bad response",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: "foo",
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`query using "prom" on %s failed with: bad_data: unhandled query`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "bad uri",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", "http://", time.Second*5, false)),
			problems: []checks.Problem{
				{
					Fragment: "foo",
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     `cound't run "promql/series" checks due to "prom" on http:// connection error: Post "http:///api/v1/query": http: no Host in request URL`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "simple query",
			content:     "- record: foo\n  expr: sum(notfound)\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: "notfound",
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`query using "prom" on %s completed without any results for notfound`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "complex query",
			content:     "- record: foo\n  expr: sum(found_7 * on (job) sum(sum(notfound))) / found_7\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: "notfound",
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`query using "prom" on %s completed without any results for notfound`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "complex query / bug",
			content:     "- record: foo\n  expr: sum(found_7 * on (job) sum(sum(notfound))) / found_7\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: "notfound",
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`query using "prom" on %s completed without any results for notfound`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "label_replace()",
			content: `- alert: foo
  expr: |
    count(
      label_replace(
        node_filesystem_readonly{mountpoint!=""},
        "device",
        "$2",
        "device",
        "/dev/(mapper/luks-)?(sd[a-z])[0-9]"
      )
    ) by (device,instance) > 0
    and on (device, instance)
    label_replace(
      disk_info{type="sat",interface_speed!="6.0 Gb/s"},
      "device",
      "$1",
      "disk",
      "/dev/(sd[a-z])"
    )
  for: 5m
`,
			checker: checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
		},
		{
			description: "offset",
			content:     "- record: foo\n  expr: node_filesystem_readonly{mountpoint!=\"\"} offset 5m\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
		},
		{
			description: "negative offset",
			content:     "- record: foo\n  expr: node_filesystem_readonly{mountpoint!=\"\"} offset -15m\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
		},
		{
			description: "series found, label missing",
			content:     "- record: foo\n  expr: found{job=\"notfound\"}\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: `found{job="notfound"}`,
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`query using "prom" on %s completed without any results for found{job="notfound"}`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "series missing, label missing",
			content:     "- record: foo\n  expr: notfound{job=\"notfound\"}\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: "notfound",
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`query using "prom" on %s completed without any results for notfound`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "series missing, {__name__=}",
			content: `
- record: foo
  expr: '{__name__="notfound", job="bar"}'
`,
			checker: checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: `{__name__="notfound",job="bar"}`,
					Lines:    []int{3},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`query using "prom" on %s completed without any results for {__name__="notfound",job="bar"}`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "series missing but check disabled",
			content: `
# pint disable promql/series(notfound)
- record: foo
  expr: count(notfound) == 0
`,
			checker: checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
		},
		{
			description: "series missing but check disabled, labels",
			content: `
# pint disable promql/series(notfound)
- record: foo
  expr: count(notfound{job="foo"}) == 0
`,
			checker: checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
		},
		{
			description: "series missing but check disabled, negative labels",
			content: `
# pint disable promql/series(notfound)
- record: foo
  expr: count(notfound{job!="foo"}) == 0
`,
			checker: checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
		},
		{
			description: "series missing, disabled comment for labels",
			content: `
# pint disable promql/series(notfound{job="foo"})
- record: foo
  expr: count(notfound) == 0
`,
			checker: checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: `notfound`,
					Lines:    []int{4},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`query using "prom" on %s completed without any results for notfound`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
	}
	runTests(t, testCases)
}
