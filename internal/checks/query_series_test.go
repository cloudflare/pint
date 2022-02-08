package checks_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"

	"github.com/rs/zerolog"
)

func TestSeriesCheck(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match := r.URL.Query()[("match[]")]

		switch strings.Join(match, ", ") {
		case "notfound", `notfound{job="foo"}`, `notfound{job!="foo"}`, `{__name__="notfound",job="bar"}`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data": []
			}`))
		case "found_1", `{__name__="notfound"}`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data": [{"__name__":"single", "foo": "bar"}]
			}`))
		case "found_7":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data": [
					{"__name__":"seven", "id": "1"},
					{"__name__":"seven", "id": "2"},
					{"__name__":"seven", "id": "3"},
					{"__name__":"seven", "id": "4"},
					{"__name__":"seven", "id": "5"},
					{"__name__":"seven", "id": "6"},
					{"__name__":"seven", "id": "7"}
				]
			}`))
		case `node_filesystem_readonly{mountpoint!=""}`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data": [{"__name__":"single", "foo": "bar"}]
			}`))
		case `disk_info{interface_speed!="6.0 Gb/s",type="sat"}`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data": [{"__name__":"disk_info"}]
			}`))
		case `found{job="notfound"}`, `notfound{job="notfound"}`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data": []
			}`))
		case `found`:
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status":"success",
				"data": [{"__name__":"found"}]
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
			checker:     checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
		},
		{
			description: "bad response",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
			problems: []checks.Problem{
				{
					Fragment: "foo",
					Lines:    []int{2},
					Reporter: "query/series",
					Text:     "query using prom failed with: bad_data: unhandled query",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "simple query",
			content:     "- record: foo\n  expr: sum(notfound)\n",
			checker:     checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
			problems: []checks.Problem{
				{
					Fragment: "notfound",
					Lines:    []int{2},
					Reporter: "query/series",
					Text:     "query using prom completed without any results for notfound",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "complex query",
			content:     "- record: foo\n  expr: sum(found_7 * on (job) sum(sum(notfound))) / found_7\n",
			checker:     checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
			problems: []checks.Problem{
				{
					Fragment: "notfound",
					Lines:    []int{2},
					Reporter: "query/series",
					Text:     "query using prom completed without any results for notfound",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "complex query / bug",
			content:     "- record: foo\n  expr: sum(found_7 * on (job) sum(sum(notfound))) / found_7\n",
			checker:     checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
			problems: []checks.Problem{
				{
					Fragment: "notfound",
					Lines:    []int{2},
					Reporter: "query/series",
					Text:     "query using prom completed without any results for notfound",
					Severity: checks.Warning,
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
			checker: checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
		},
		{
			description: "offset",
			content:     "- record: foo\n  expr: node_filesystem_readonly{mountpoint!=\"\"} offset 5m\n",
			checker:     checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
		},
		{
			description: "negative offset",
			content:     "- record: foo\n  expr: node_filesystem_readonly{mountpoint!=\"\"} offset -15m\n",
			checker:     checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
		},
		{
			description: "series found, label missing",
			content:     "- record: foo\n  expr: found{job=\"notfound\"}\n",
			checker:     checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
			problems: []checks.Problem{
				{
					Fragment: `found{job="notfound"}`,
					Lines:    []int{2},
					Reporter: "query/series",
					Text:     `query using prom completed without any results for found{job="notfound"}`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "series missing, label missing",
			content:     "- record: foo\n  expr: notfound{job=\"notfound\"}\n",
			checker:     checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
			problems: []checks.Problem{
				{
					Fragment: "notfound",
					Lines:    []int{2},
					Reporter: "query/series",
					Text:     "query using prom completed without any results for notfound",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "series missing, {__name__=}",
			content: `
- record: foo
  expr: '{__name__="notfound", job="bar"}'
`,
			checker: checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
			problems: []checks.Problem{
				{
					Fragment: `{__name__="notfound",job="bar"}`,
					Lines:    []int{3},
					Reporter: "query/series",
					Text:     `query using prom completed without any results for {__name__="notfound",job="bar"}`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "series missing but check disabled",
			content: `
# pint disable query/series(notfound)
- record: foo
  expr: count(notfound) == 0
`,
			checker: checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
		},
		{
			description: "series missing but check disabled, labels",
			content: `
# pint disable query/series(notfound)
- record: foo
  expr: count(notfound{job="foo"}) == 0
`,
			checker: checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
		},
		{
			description: "series missing but check disabled, negative labels",
			content: `
# pint disable query/series(notfound)
- record: foo
  expr: count(notfound{job!="foo"}) == 0
`,
			checker: checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
		},
		{
			description: "series missing, disabled comment for labels",
			content: `
# pint disable query/series(notfound{job="foo"})
- record: foo
  expr: count(notfound) == 0
`,
			checker: checks.NewSeriesCheck(promapi.NewPrometheus("prom", srv.URL, time.Second*5)),
			problems: []checks.Problem{
				{
					Fragment: `notfound`,
					Lines:    []int{4},
					Reporter: "query/series",
					Text:     `query using prom completed without any results for notfound`,
					Severity: checks.Warning,
				},
			},
		},
	}
	runTests(t, testCases)
}
