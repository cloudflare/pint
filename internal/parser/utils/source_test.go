package utils_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

func TestMain(t *testing.M) {
	v := t.Run()
	if _, err := snaps.Clean(t, snaps.CleanOpts{Sort: true}); err != nil {
		fmt.Printf("snaps.Clean() returned an error: %s", err)
		os.Exit(100)
	}
	os.Exit(v)
}

func TestLabelsSource(t *testing.T) {
	testCases := []string{
		"1",
		"1 / 5",
		"(2 ^ 5) == bool 5",
		"(2 ^ 5 + 11) % 5 <= bool 2",
		"(2 ^ 5 + 11) % 5 >= bool 20",
		"(2 ^ 5 + 11) % 5 <= bool 3",
		"(2 ^ 5 + 11) % 5 < bool 1",
		"20 - 15 < bool 1",
		"2 * 5",
		"(foo or bar) * 5",
		"(foo or vector(2)) * 5",
		"(foo or vector(5)) * (vector(2) or bar)",
		`1 > bool 0`,
		`20 > bool 10`,
		`"test"`,
		"foo",
		"(foo > 1) > bool 1",
		"foo > bool 5",
		"foo > bool 5 == 1",
		"foo > bool bar",
		"(foo > bool bar) == 0",
		"foo > bool on(instance) bar",
		"(foo > bool on(instance) bar) == 1",
		"foo > bool on(instance) group_left(version) bar",
		"bar > bool on(instance) group_right(version) foo",
		"foo and bar > bool 0",
		"foo offset 5m",
		`foo{job="bar"}`,
		`foo{job=""}`,
		`foo{job="bar"} or bar{job="foo"}`,
		`foo{a="bar"} or bar{b="foo"}`,
		"foo[5m]",
		"prometheus_build_info[2m:1m]",
		"deriv(rate(distance_covered_meters_total[1m])[5m:1m])",
		"foo - 1",
		"foo / 5",
		"-foo",
		`sum(foo{job="myjob"})`,
		`sum(count(foo{job="myjob"}) by(instance))`,
		`sum(foo{job="myjob"}) > 20`,
		`sum(foo{job="myjob"}) without(job)`,
		`sum(foo) by(job)`,
		`sum(foo{job="myjob"}) by(job)`,
		`abs(foo{job="myjob"} offset 5m)`,
		`abs(foo{job="myjob"} or bar{cluster="dev"})`,
		`sum(foo{job="myjob"} or bar{cluster="dev"}) without(instance)`,
		`sum(foo{job="myjob"}) without(instance)`,
		`min(foo{job="myjob"}) / max(foo{job="myjob"})`,
		`max(foo{job="myjob"}) / min(foo{job="myjob"})`,
		`avg(foo{job="myjob"}) by(job)`,
		`group(foo) by(job)`,
		`stddev(rate(foo[5m]))`,
		`stdvar(rate(foo[5m]))`,
		`stddev_over_time(foo[5m])`,
		`stdvar_over_time(foo[5m])`,
		`quantile(0.9, rate(foo[5m]))`,
		`count_values("version", build_version)`,
		`count_values("version", build_version) without(job)`,
		`count_values("version", build_version{job="foo"}) without(job)`,
		`count_values("version", build_version) by(job)`,
		`topk(10, foo{job="myjob"}) > 10`,
		`topk(10, foo or bar)`,
		`rate(foo[10m])`,
		`sum(rate(foo[10m])) without(instance)`,
		`foo{job="foo"} / bar`,
		`foo{job="foo"} * on(instance) bar`,
		`foo{job="foo"} * on(instance) group_left(bar) bar`,
		`foo{job="foo"} * on(instance) group_left(cluster) bar{cluster="bar", ignored="true"}`,
		`foo{job="foo", ignored="true"} * on(instance) group_right(job) bar{cluster="bar"}`,
		`count(foo / bar)`,
		`count(up{job="a"} / on () up{job="b"})`,
		`count(up{job="a"} / on (env) up{job="b"})`,
		`foo{job="foo", instance="1"} and bar`,
		`foo{job="foo", instance="1"} and on(cluster) bar`,
		`topk(10, foo)`,
		`topk(10, foo) without(cluster)`,
		`topk(10, foo) by(cluster)`,
		`bottomk(10, sum(rate(foo[5m])) without(job))`,
		`foo or bar`,
		`foo or bar or baz`,
		`(foo or bar) or baz`,
		`foo unless bar`,
		`foo unless bar > 5`,
		`foo unless bar unless baz`,
		`count(sum(up{job="foo", cluster="dev"}) by(job, cluster) == 0) without(job, cluster)`,
		"year()",
		"year(foo)",
		`label_join(up{job="api-server",src1="a",src2="b",src3="c"}, "foo", ",", "src1", "src2", "src3")`,
		`
(
	sum(foo:sum > 0) without(notify)
	* on(job) group_left(notify)
	job:notify
)
and on(job)
sum(foo:count) by(job) > 20`,
		`container_file_descriptors / on (instance, app_name) container_ulimits_soft{ulimit="max_open_files"}`,
		`container_file_descriptors / on (instance, app_name) group_left() container_ulimits_soft{ulimit="max_open_files"}`,
		`absent(foo{job="bar"})`,
		`absent(foo{job="bar", cluster!="dev", instance=~".+", env="prod"})`,
		`absent(sum(foo) by(job, instance))`,
		`absent(foo{job="prometheus", xxx="1"}) AND on(job) prometheus_build_info`,
		`1 + sum(foo) by(notjob)`,
		`count(node_exporter_build_info) by (instance, version) != ignoring(package,version) group_left(foo) count(deb_package_version) by (instance, version, package)`,
		`absent(foo) or absent(bar)`,
		`absent_over_time(foo[5m]) or absent(bar)`,
		`bar * on() group_right(cluster, env) absent(foo{job="xxx"})`,
		`bar * on() group_right() absent(foo{job="xxx"})`,
		"vector(1)",
		"vector(scalar(foo))",
		"vector(0.0  >= bool 0.5) == 1",
		`sum_over_time(foo{job="myjob"}[5m])`,
		`days_in_month()`,
		`days_in_month(foo{job="foo"})`,
		`label_replace(up{job="api-server",service="a:c"}, "foo", "$1", "service", "(.*):.*")`,
		`label_replace(sum by (pod) (pod_status) > 0, "cluster", "$1", "pod", "(.*)")`,
		`(time() - my_metric) > 5*3600`,
		`up{instance="a", job="prometheus"} * ignoring(job) up{instance="a", job="pint"}`,
		`
avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"})
< 0.5 > 0
or sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"})
< on() count(colo_router_tier:disabled_pops:max{tier="1",router=~"edge.*"}) * 0.4 > 0
or avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"})
< 0.1 > 0
`,
		`label_replace(sum(foo) without(instance), "instance", "none", "", "")`,
		`
sum by (region, target, colo_name) (
    sum_over_time(probe_success{job="abc"}[5m])
	or
	vector(1)
) == 0`,
		`vector(1) or foo`,
		`vector(0) > 0`,
		`vector(0) > vector(1)`,
		`sum(foo or vector(0)) > 0`,
		`(sum(foo or vector(1)) > 0) == 2`,
		`(sum(foo or vector(1)) > 0) != 2`,
		`(sum(foo or vector(2)) > 0) != 2`,
		`(sum(sometimes{foo!="bar"} or vector(0)))
or
((bob > 10) or sum(foo) or vector(1))`,
		`
(
	sum(sometimes{foo!="bar"})
	or
	vector(1)
) and (
	((bob > 10) or sum(bar))
	or
	notfound > 0
)`,
		"foo offset 5m > 5",
		`
(rate(metric2[5m]) or vector(0)) +
(rate(metric1[5m]) or vector(1)) +
(rate(metric3{log_name="samplerd"}[5m]) or vector(2)) > 0
`,
		`label_replace(vector(1), "nexthop_tag", "$1", "nexthop", "(.+)")`,
		`(sum(foo{job="myjob"}))`,
		`(-foo{job="myjob"})`,
		"\n((( group(vector(0)) ))) > 0",
		"1 > bool 5",
		`prometheus_ready{job="prometheus"} unless vector(0)`,
		`prometheus_ready{job="prometheus"} unless on() vector(0)`,
		`prometheus_ready{job="prometheus"} unless on(job) vector(0)`,
		`
max by (instance, cluster) (cf_node_role{kubernetes_role="master",role="kubernetes"})
unless
	sum by (instance, cluster) (time() - node_systemd_timer_last_trigger_seconds{name=~"etcd-defrag-.*.timer"})
  	* on (instance) group_left (cluster)
    cf_node_role{kubernetes_role="master",role="kubernetes"}
`,
		`foo{a="1"} * on() bar{b="2"}`,
		`foo{a="1"} * on(instance) group_left(c,d) bar{b="2"}`,
		`foo{a="1"} * on(instance) group_right(c,d) bar{b="2"}`,
		`foo{a="1"} * on(instance) sum(bar{b="2"})`,
		`foo{a="1"} * on(instance) group_left(c,d) sum(bar{b="2"})`,
		`sum(foo{a="1"}) * on(instance) group_right(c,d) bar{b="2"}`,
		`foo{a="1"} * on(instance) group_left(c,d) sum(bar{b="2"}) without(instance)`,
		`sum(foo{a="1"}) without(instance) * on(instance) group_right(c,d) bar{b="2"}`,
		`
 max without (source_instance) (
   increase(kernel_device_io_errors_total{device!~"loop.+"}[120m]) > 3 unless on(instance, device) (
     increase(kernel_device_io_soft_errors_total{device!~"loop.+"}[125m])*2 > increase(kernel_device_io_errors_total[120m])
   )
   and on(device, instance) absent(node_disk_info)
 ) * on(instance) group_left(group) label_replace(salt_highstate_runner_configured_minions, "instance", "$1", "minion", "(.+)")
`,
		`sum(foo{a="1"}) by(job) * on() bar{b="2"}`,
		`sum(sum(foo) without(job)) by(job)`,
		`
prometheus:scrape_series_added:since_gc:sum
* on(prometheus) group_left()
label_replace(
  max(max_over_time(go_memstats_alloc_bytes{job="prometheus"}[2h])) by(instance)
  /
  max(max_over_time(prometheus_tsdb_head_series[2h])) by(instance),
  "prometheus", "$1",
  "instance", "(.+)"
)
`,
		`(day_of_week() == 6 and hour() < 1) or vector(1)`,
		`
sum by (foo, bar) (
    rate(errors_total[5m])
  * on (instance) group_left (bob, alice)
    server_errors_total
)`,
		`1 - (foo or vector(0)) < 0.999`,
		`
(
  vector(1) and month() == 2
) or vector(0)
`,
		`count by (region) (stddev by (colo_name, region) (error_total))`,
		`
(
  avg(
    rate(foo_rejections[6h])
    or
    vector(0)
  ) by (colo_name)
  /
  (
    avg(
      rate(foo_total[6h])
	  or
	  vector(1)
    ) by (colo_name)
  )
) > 5
*
(
  avg(
    rate(foo_rejections[6h] offset 1d)
	or
	vector(0)
  ) by (colo_name)
  /
  avg(
    rate(foo_total[6h] offset 1d)
	or
	vector(1)
  ) by (colo_name)
) and on (colo_name) (colo_job:foo_total:rate2m or vector(0)) > 80
  and on (colo_name) (colo_job:foo_total:rate2m offset 1d or vector(0)) > 80
`,
		`sum(selector) / sum(selector offset 30m) > 5`,
		`
count by (dc) (
  max(0 < (token_expiration - time()) < (6*60*60)) by (instance)
  * on (instance) group_right label_replace(
    configured_minions, "instance", "$1", "minion", "(.+)")
  ) > 5`,
		`topk(10, prometheus_build_info*prometheus_ready)`,
		`bottomk(10, prometheus_build_info*prometheus_ready)`,
	}

	type Snapshot struct {
		Expr   string
		Output []utils.Source
	}

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "can't get caller function")
	file = strings.TrimSuffix(filepath.Base(file), ".go")

	done := map[string]struct{}{}
	for i, expr := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			if _, ok := done[expr]; ok {
				t.Fatalf("Duplicated query: %s", expr)
			}
			done[expr] = struct{}{}

			n, err := parser.DecodeExpr(expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			output := utils.LabelsSource(expr, n.Expr)

			for _, src := range output {
				src.WalkSources(func(s utils.Source, _ *utils.Join, _ *utils.Unless) {
					require.Positive(t, s.Position.End, "empty position %+v", s)
					if s.DeadInfo != nil {
						require.Positive(t, s.DeadInfo.Fragment.End, "empty dead position %+v", s)
					}
				})
			}

			snap := Snapshot{
				Expr:   expr,
				Output: output,
			}
			d, err := yaml.Marshal(snap)
			require.NoError(t, err, "failed to YAML encode snapshots")
			snaps.WithConfig(snaps.Dir("."), snaps.Filename(file)).MatchSnapshot(t, string(d))
		})
	}
}

func TestLabelsSourceCallCoverage(t *testing.T) {
	for name, def := range promParser.Functions {
		t.Run(name, func(t *testing.T) {
			if def.Experimental {
				t.SkipNow()
			}

			var b strings.Builder
			b.WriteString(name)
			b.WriteRune('(')
			for i, at := range def.ArgTypes {
				if i > 0 {
					b.WriteString(", ")
				}
				switch at {
				case promParser.ValueTypeNone:
				case promParser.ValueTypeScalar:
					b.WriteRune('1')
				case promParser.ValueTypeVector:
					b.WriteString("http_requests_total")
				case promParser.ValueTypeMatrix:
					b.WriteString("http_requests_total[2m]")
				case promParser.ValueTypeString:
					b.WriteString(`"foo"`)
				}
			}
			b.WriteRune(')')

			n, err := parser.DecodeExpr(b.String())
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			output := utils.LabelsSource(b.String(), n.Expr)
			require.Len(t, output, 1)
			require.NotEmpty(t, output[0].Operations)
			call, ok := utils.MostOuterOperation[*promParser.Call](output[0])
			require.True(t, ok, "no call found in operations for: %q ~> %+v", b.String(), output)
			require.NotNil(t, call, "no call detected in: %q ~> %+v", b.String(), output)
			require.Equal(t, name, output[0].Operation())
			require.Equal(t, def.ReturnType, output[0].Returns, "incorrect return type on Source{}")
		})
	}
}

func TestLabelsSourceCallCoverageFail(t *testing.T) {
	n := &parser.PromQLNode{
		Expr: &promParser.Call{
			Func: &promParser.Function{
				Name: "fake_call",
			},
		},
	}
	output := utils.LabelsSource("fake_call()", n.Expr)
	require.Len(t, output, 1)
	call, ok := utils.MostOuterOperation[*promParser.Call](output[0])
	require.False(t, ok, "no call should have been detected in fake function, got: %v", ok)
	require.Nil(t, call, "no call should have been detected in fake function, got: %+v", call)
}

func TestVectorOperation(t *testing.T) {
	n := &parser.PromQLNode{
		Expr: &promParser.NumberLiteral{
			Val: 1,
		},
	}
	output := utils.LabelsSource("1", n.Expr)
	require.Len(t, output, 1)
	require.Empty(t, output[0].Operation())
}
