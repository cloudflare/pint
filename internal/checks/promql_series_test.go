package checks_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
)

func TestSeriesCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Fatal(err)
		}
		query := r.Form.Get("query")

		switch query {
		case `count(test_metric)`, `count(test_metric_c) by (step)`:
			w.WriteHeader(400)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"error",
				"errorType":"execution",
				"error":"query failed"
			}`))
		case "count(notfound)",
			`count(notfound{job="bar"}`,
			`count(notfound{job="foo"})`,
			`count(notfound{job!="foo"})`,
			`count({__name__="notfound",job="bar"})`,
			`count(ALERTS{alertname="foo"})`,
			`count(ALERTS{notfound="foo"})`,
			`count(test_metric{step="2"})`,
			`count(test_metric_c{step="3"})`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/v1/query_range" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[]
				}
			}`))
		case "count(found_1)", `count({__name__="notfound"})`, `count(ALERTS) by (notfound)`, `count(test_metric_c)`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/v1/query_range" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"status":"success",
					"data":{
						"resultType":"matrix",
						"result":[
							{
								"metric":{},"values":[
									[1614859502.068,"1"]
								]
							},
							{
								"metric":{},"values":[
									[1614869502.068,"1"]
								]
							}
						]
					}
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
				}
			}`))

		case "count(found_7)", `count(ALERTS)`, `count(ALERTS) by (alertname)`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/v1/query_range" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"status":"success",
					"data":{
						"resultType":"matrix",
						"result":[
							{
								"metric":{"alertname": "xxx"},"values":[
									[1614859502.068,"1"]
								]
							},
							{
								"metric":{"alertname": "yyy"},"values":[
									[1614859502.068,"1"]
								]
							}
						]
					}
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[
						{"metric":{"alertname": "xxx"},"value":[1614859502.068,"7"]},
						{"metric":{"alertname": "yyy"},"value":[1614859502.068,"7"]}
					]
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
		case `count(found{job="notfound"})`, `count(notfound{job="notfound"})`, `count(notfound{job="bar"})`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/v1/query_range" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
				return
			}
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
			if r.URL.Path == "/api/v1/query_range" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"status":"success",
					"data":{
						"resultType":"matrix",
						"result":[
							{
								"metric":{},"values":[
									[1614859502.068,"1"]
								]
							},
							{
								"metric":{},"values":[
									[1614869502.068,"1"]
								]
							}
						]
					}
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[{"metric":{},"value":[1614859502.068,"1"]}]
				}
			}`))
		case `count(found) by (job)`, `count({__name__="notfound"}) by (job)`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api/v1/query_range" {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"status":"success",
					"data":{
						"resultType":"matrix",
						"result":[
							{
								"metric":{"job": "xxx"},"values":[
									[1614859502.068,"1"]
								]
							},
							{
								"metric":{"job": "yyy"},"values":[
									[1614859502.068,"1"]
								]
							}
						]
					}
				}`))
				return
			}
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data":{
					"resultType":"vector",
					"result":[
						{"metric":{"job": "xxx"},"value":[1614859502.068,"1"]},
						{"metric":{"job": "yyy"},"value":[1614859502.068,"1"]}
					]
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
					Text:     fmt.Sprintf(`prometheus "prom" at %s failed with: bad_data: unhandled query`, srv.URL),
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
					Text:     `cound't run "promql/series" checks due to prometheus "prom" at http:// connection error: Post "http:///api/v1/query": http: no Host in request URL`,
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
					Text:     fmt.Sprintf(`prometheus "prom" at %s didn't have any series for "notfound" metric in the last 1w`, srv.URL),
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
					Text:     fmt.Sprintf(`prometheus "prom" at %s didn't have any series for "notfound" metric in the last 1w`, srv.URL),
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
					Text:     fmt.Sprintf(`prometheus "prom" at %s didn't have any series for "notfound" metric in the last 1w`, srv.URL),
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
					Text:     fmt.Sprintf(`prometheus "prom" at %s has "found" metric but there are no series matching {job="notfound"} in the last 1w, "job" looks like a high churn label`, srv.URL),
					Severity: checks.Warning,
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
					Text:     fmt.Sprintf(`prometheus "prom" at %s didn't have any series for "notfound" metric in the last 1w`, srv.URL),
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
					Text:     fmt.Sprintf(`prometheus "prom" at %s has "{__name__=\"notfound\"}" metric but there are no series matching {job="bar"} in the last 1w, "job" looks like a high churn label`, srv.URL),
					Severity: checks.Warning,
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
					Text:     fmt.Sprintf(`prometheus "prom" at %s didn't have any series for "notfound" metric in the last 1w`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "ALERTS{notfound=...}",
			content:     "- alert: foo\n  expr: count(ALERTS{notfound=\"foo\"}) >= 10\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: `ALERTS{notfound="foo"}`,
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`prometheus "prom" at %s has "ALERTS" metric but there are no series with "notfound" label in the last 1w`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "ALERTS{alertname=...}",
			content:     "- alert: foo\n  expr: count(ALERTS{alertname=\"foo\"}) >= 10\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: `ALERTS{alertname="foo"}`,
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`prometheus "prom" at %s has "ALERTS" metric but there are no series matching {alertname="foo"} in the last 1w, "alertname" looks like a high churn label`, srv.URL),
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "step 2 error",
			content:     "- alert: foo\n  expr: test_metric{step=\"2\"}\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: "test_metric",
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`cound't run "promql/series" checks due to prometheus "prom" at %s connection error: no more retries possible`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "step 3 error",
			content:     "- alert: foo\n  expr: test_metric_c{step=\"3\"}\n",
			checker:     checks.NewSeriesCheck(simpleProm("prom", srv.URL, time.Second*5, true)),
			problems: []checks.Problem{
				{
					Fragment: "test_metric_c",
					Lines:    []int{2},
					Reporter: "promql/series",
					Text:     fmt.Sprintf(`cound't run "promql/series" checks due to prometheus "prom" at %s connection error: no more retries possible`, srv.URL),
					Severity: checks.Bug,
				},
			},
		},
	}
	runTests(t, testCases)
}
