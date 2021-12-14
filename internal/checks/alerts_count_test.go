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

func TestAlertsCheck(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	content := "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n"

	now := time.Now()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Fatal(err)
		}
		query := r.Form.Get("query")
		if query != `up{job="foo"} == 0` &&
			query != `{__name__="up", job="foo"} == 0` &&
			query != `{__name__=~"(up|foo)", job="foo"} == 0` {
			t.Fatalf("Prometheus got invalid query: %s", query)
		}

		switch r.URL.Path {
		case "/empty/api/v1/query_range":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		case "/alerts/api/v1/query_range":
			w.WriteHeader(200)
			out := fmt.Sprintf(`{
				"status":"success",
				"data":{
					"resultType":"matrix",
					"result":[
						{"metric":{"instance":"1"},"values":[
							[%d,"0"],
							[%d,"0"],
							[%d,"0"],
							[%d,"0"],
							[%d,"0"]
						]},
						{"metric":{"instance":"2"},"values":[
							[%d,"0"],
							[%d,"0"],
							[%d,"0"],
							[%d,"0"],
							[%d,"0"]
						]}
					]
				}
			}`,
				now.AddDate(0, 0, -1).Unix(),
				now.AddDate(0, 0, -1).Add(time.Minute).Unix(),
				now.AddDate(0, 0, -1).Add(time.Minute*2).Unix(),
				now.AddDate(0, 0, -1).Add(time.Minute*60).Unix(),
				now.AddDate(0, 0, -1).Add(time.Minute*61).Unix(),

				now.AddDate(0, 0, -1).Add(time.Minute*6).Unix(),
				now.AddDate(0, 0, -1).Add(time.Minute*12).Unix(),
				now.AddDate(0, 0, -1).Add(time.Minute*18).Unix(),
				now.AddDate(0, 0, -1).Add(time.Minute*24).Unix(),
				now.AddDate(0, 0, -1).Add(time.Minute*30).Unix(),
			)
			_, _ = w.Write([]byte(out))
		default:
			w.WriteHeader(400)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"unhandled path"}`))
		}
	}))
	defer srv.Close()

	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     checks.NewAlertsCheck("prom", "http://localhost", time.Second*5, time.Hour*24, time.Minute, time.Minute*5),
		},
		{
			description: "ignores rules with syntax errors",
			content:     "- alert: Foo Is Down\n  expr: sum(\n",
			checker:     checks.NewAlertsCheck("prom", "http://localhost", time.Second*5, time.Hour*24, time.Minute, time.Minute*5),
		},
		{
			description: "bad request",
			content:     content,
			checker:     checks.NewAlertsCheck("prom", srv.URL+"/400/", time.Second*5, time.Hour*24, time.Minute, time.Minute*5),
			problems: []checks.Problem{
				{
					Fragment: `up{job="foo"} == 0`,
					Lines:    []int{2},
					Reporter: "alerts/count",
					Text:     "query using prom failed with: bad_data: unhandled path",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "empty response",
			content:     content,
			checker:     checks.NewAlertsCheck("prom", srv.URL+"/empty/", time.Second*5, time.Hour*24, time.Minute, time.Minute*5),
			problems: []checks.Problem{
				{
					Fragment: `up{job="foo"} == 0`,
					Lines:    []int{2},
					Reporter: "alerts/count",
					Text:     "query using prom would trigger 0 alert(s) in the last 1d",
					Severity: checks.Information,
				},
			},
		},
		{
			description: "multiple alerts",
			content:     content,
			checker:     checks.NewAlertsCheck("prom", srv.URL+"/alerts/", time.Second*5, time.Hour*24, time.Minute, time.Minute*5),
			problems: []checks.Problem{
				{
					Fragment: `up{job="foo"} == 0`,
					Lines:    []int{2},
					Reporter: "alerts/count",
					Text:     "query using prom would trigger 7 alert(s) in the last 1d",
					Severity: checks.Information,
				},
			},
		},
		{
			description: "for: 10m",
			content:     "- alert: Foo Is Down\n  for: 10m\n  expr: up{job=\"foo\"} == 0\n",
			checker:     checks.NewAlertsCheck("prom", srv.URL+"/alerts/", time.Second*5, time.Hour*24, time.Minute*6, time.Minute*10),
			problems: []checks.Problem{
				{
					Fragment: `up{job="foo"} == 0`,
					Lines:    []int{2, 3},
					Reporter: "alerts/count",
					Text:     "query using prom would trigger 1 alert(s) in the last 1d",
					Severity: checks.Information,
				},
			},
		},
		{
			description: "{__name__=}",
			content: `
- alert: foo
  expr: '{__name__="up", job="foo"} == 0'
`,
			checker: checks.NewAlertsCheck("prom", srv.URL+"/alerts/", time.Second*5, time.Hour*24, time.Minute*6, time.Minute*10),
			problems: []checks.Problem{
				{
					Fragment: `{__name__="up", job="foo"} == 0`,
					Lines:    []int{3},
					Reporter: "alerts/count",
					Text:     "query using prom would trigger 3 alert(s) in the last 1d",
					Severity: checks.Information,
				},
			},
		},
		{
			description: "{__name__=~}",
			content: `
- alert: foo
  expr: '{__name__=~"(up|foo)", job="foo"} == 0'
`,
			checker: checks.NewAlertsCheck("prom", srv.URL+"/alerts/", time.Second*5, time.Hour*24, time.Minute*6, time.Minute*10),
			problems: []checks.Problem{
				{
					Fragment: `{__name__=~"(up|foo)", job="foo"} == 0`,
					Lines:    []int{3},
					Reporter: "alerts/count",
					Text:     "query using prom would trigger 3 alert(s) in the last 1d",
					Severity: checks.Information,
				},
			},
		},
	}

	runTests(t, testCases)
}
