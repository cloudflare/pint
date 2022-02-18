package checks_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/cloudflare/pint/internal/checks"
)

func TestRateCheck(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.FatalLevel)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/30s/api/v1/status/config":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 30s\n"}}`))
		case "/1m/api/v1/status/config":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 1m\n"}}`))
		case "/default/api/v1/status/config":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  {}\n"}}`))
		case "/error/api/v1/status/config":
			w.WriteHeader(500)
			_, _ = w.Write([]byte("fake error\n"))
		case "/badYaml/api/v1/status/config":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"invalid yaml"}}`))
		default:
			w.WriteHeader(400)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"unhandled path"}`))
		}
	}))
	defer srv.Close()

	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL, time.Second, true)),
		},
		{
			description: "rate < 2x scrape_interval",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: "rate(foo[1m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     "duration for rate() must be at least 2 x scrape_interval, prom is using 1m scrape_interval",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "rate < 4x scrape_interval",
			content:     "- record: foo\n  expr: rate(foo[3m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: "rate(foo[3m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     "duration for rate() is recommended to be at least 4 x scrape_interval, prom is using 1m scrape_interval",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "rate == 4x scrape interval",
			content:     "- record: foo\n  expr: rate(foo[2m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/30s/", time.Second, true)),
		},
		{
			description: "irate < 2x scrape_interval",
			content:     "- record: foo\n  expr: irate(foo[1m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: "irate(foo[1m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     "duration for irate() must be at least 2 x scrape_interval, prom is using 1m scrape_interval",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "irate < 3x scrape_interval",
			content:     "- record: foo\n  expr: irate(foo[2m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: "irate(foo[2m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     "duration for irate() is recommended to be at least 3 x scrape_interval, prom is using 1m scrape_interval",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "irate{__name__} > 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__="foo"}[5m])
`,
			checker: checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
		},
		{
			description: "irate{__name__=~} > 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__=~"(foo|bar)_total"}[5m])
`,
			checker: checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
		},
		{
			description: "irate{__name__} < 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__="foo"}[2m])
`,
			checker: checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: `irate({__name__="foo"}[2m])`,
					Lines:    []int{3},
					Reporter: "promql/rate",
					Text:     "duration for irate() is recommended to be at least 3 x scrape_interval, prom is using 1m scrape_interval",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "irate{__name__=~} < 3x scrape_interval",
			content: `
- record: foo
  expr: irate({__name__=~"(foo|bar)_total"}[2m])
`,
			checker: checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: `irate({__name__=~"(foo|bar)_total"}[2m])`,
					Lines:    []int{3},
					Reporter: "promql/rate",
					Text:     "duration for irate() is recommended to be at least 3 x scrape_interval, prom is using 1m scrape_interval",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "irate == 3x scrape interval",
			content:     "- record: foo\n  expr: irate(foo[3m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
		},
		{
			description: "valid range selector",
			content:     "- record: foo\n  expr: foo[1m]\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
		},
		{
			description: "nested invalid rate",
			content:     "- record: foo\n  expr: sum(rate(foo[3m])) / sum(rate(bar[1m]))\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/1m/", time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: "rate(foo[3m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     "duration for rate() is recommended to be at least 4 x scrape_interval, prom is using 1m scrape_interval",
					Severity: checks.Warning,
				},
				{
					Fragment: "rate(bar[1m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     "duration for rate() must be at least 2 x scrape_interval, prom is using 1m scrape_interval",
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "500 error from Prometheus API",
			content:     "- record: foo\n  expr: rate(foo[5m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/error/", time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: "rate(foo[5m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     `cound't run "promql/rate" checks due to "prom" prometheus connection error: failed to query Prometheus config: server_error: server error: 500`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "invalid status",
			content:     "- record: foo\n  expr: rate(foo[5m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL, time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: "rate(foo[5m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     `query using prom failed with: failed to query Prometheus config: bad_data: unhandled path`,
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "invalid YAML",
			content:     "- record: foo\n  expr: rate(foo[5m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/badYaml/", time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: "rate(foo[5m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     fmt.Sprintf("cound't run \"promql/rate\" checks due to \"prom\" prometheus connection error: failed to decode config data in %s/badYaml/ response: yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `invalid...` into promapi.PrometheusConfig", srv.URL),
					Severity: checks.Bug,
				},
			},
		},
		{
			description: "connection refused",
			content:     "- record: foo\n  expr: rate(foo[5m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", "http://", time.Second, false)),
			problems: []checks.Problem{
				{
					Fragment: "rate(foo[5m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     `cound't run "promql/rate" checks due to "prom" prometheus connection error: failed to query Prometheus config: Get "http:///api/v1/status/config": http: no Host in request URL`,
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "irate == 3 x default 1m",
			content:     "- record: foo\n  expr: irate(foo[3m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/default/", time.Second, true)),
		},
		{
			description: "irate < 3 x default 1m",
			content:     "- record: foo\n  expr: irate(foo[2m])\n",
			checker:     checks.NewRateCheck(simpleProm("prom", srv.URL+"/default/", time.Second, true)),
			problems: []checks.Problem{
				{
					Fragment: "irate(foo[2m])",
					Lines:    []int{2},
					Reporter: "promql/rate",
					Text:     "duration for irate() is recommended to be at least 3 x scrape_interval, prom is using 1m scrape_interval",
					Severity: checks.Warning,
				},
			},
		},
	}
	runTests(t, testCases)
}
