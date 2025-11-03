package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newImpossibleCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewImpossibleCheck()
}

func TestImpossibleCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newImpossibleCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "vector(0) > 0",
			content: `
- alert: Foo
  expr: ((( group(vector(0)) ))) > 0
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "0 > 0",
			content: `
- alert: Foo
  expr: 0 > bool 0
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "sum(foo or vector(0)) > 0",
			content: `
- alert: Foo
  expr: sum(foo or vector(0)) > 0
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "foo{job=bar} unless vector(0)",
			content: `
- alert: Foo
  expr: foo{job="bar"} unless vector(0)
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "foo{job=bar} unless sum(foo)",
			content: `
- alert: Foo
  expr: foo{job="bar"} unless sum(foo)
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "",
			content: `
  - alert: Device_IO_Errors
    expr: >-
      max without (source_instance) (
        increase(kernel_device_io_errors_total{device!~"loop.+"}[120m]) > 3 unless on(instance, device) (
          increase(kernel_device_io_soft_errors_total{device!~"loop.+"}[125m])*2 > increase(kernel_device_io_errors_total[120m])
        )
        and on(device, instance) absent(node_disk_info)
      ) unless on (instance,device) max(max_over_time(cloudchamber_snapshot_devices[1h])) by (instance,device)
    labels:
      priority: "4"
      component: disk
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "__name__ is stripped",
			content: `
- record: count:sum:foo
  expr: |
    {job="foo"} unless on(__name__) count(sum({job="foo"})) by(__name__)
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "or vector() labels are missing",
			content: `
- alert: Foo
  expr: |
    (
      max(job:writes_total:rate5m{region=~"wnam|weur", job="myjob", cluster=~"(a|b)"} or vector(0)) by(region)
      +
      max(job:skipps_total:rate5m{region=~"wnam|weur", job="myjob", cluster=~"(a|b)"} or vector(0)) by(region)
    ) / sum(rate(records_total{region=~"wnam|weur"}[5m])) by (region) < 0.90
  annotations:
    summary: Throughput in region {{ $labels.region }}
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "complex query with or vector()",
			content: `
- alert: Foo
  expr: |
    (avg(rate(foo_rejections[6h]) or vector(0)) by (colo_name) /
      (avg(rate(foo_total[6h]) or vector(1)) by (colo_name)))
    > 5 * (avg(rate(foo_rejections[6h] offset 1d) or vector(0)) by (colo_name) /
      avg(rate(foo_total[6h] offset 1d) or vector(1)) by (colo_name))
    # Multi-line comment
    # inside the query
    and on (colo_name)
      (colo_job:foo_total:rate2m or vector(0)) > 80
    and on (colo_name)
      (colo_job:foo_total:rate2m offset 1d or vector(0)) > 80
  annotations:
    summary: High rejectsion rate in {{ $labels.colo_name }}
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "group_by and on()",
			content: `
- record: foo
  expr: |
    group by (env, cluster) (
      up{env="prod", job="foo"} and on (instance) (services_enabled == 999)
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   false,
		},
		{
			description: "impossible group_by *",
			content: `
- record: foo
  expr: |
    group by (env, cluster, status, instance, dc, port) (
        up{env="prod", job="foo"} * on (instance) (services_enabled == 999)
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "impossible / on",
			content: `
- record: foo
  expr: foo / on(instance) sum(bar)
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "impossible / on group_left ",
			content: `
- record: foo
  expr: foo / on(instance) group_left(cluster) sum(bar)
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "impossible / on group_right",
			content: `
- record: foo
  expr: sum(bar) / on(instance) group_right(cluster) foo
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "impossible sum * on sum",
			content: `
- record: foo
  expr: sum(bar) * on(cluster) sum(foo)
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "impossible sum by sum by group_left",
			content: `
- record: foo
  expr: |
    sum by (cluster, err, gen, scope, is_dev, job, slice)
      ( sum by (instance, job) (rate(cycles_total[2m])) * on (instance)
      group_left (err, gen, scope, is_dev, slice) (instance_job:node_metadata)
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
	}

	runTests(t, testCases)
}
