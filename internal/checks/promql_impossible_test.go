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
		{
			description: "impossible group_by",
			content: `
- record: foo
  expr: |
    group by (colo_name, instance, tier, animal, brand, sliver, pop_name) (
      up{node_status="v", job="node_exporter"}
      and on (instance) (metal_services_enabled == 999)
      * on (colo_name) group_left(tier, animal, brand, pop_name) colo_metadata{colo_status="v"}
      * on (instance) group_left (sliver) sliver_metadata{node_status="v"}
      unless on (instance) label_join(
        max by (boring_instance) (boring_reboot_in_maintenance) == 1,
            "instance",
            "$1",
            "boring_instance"
          )
      unless on (colo_name) (colostat_disabled_pops == 1)
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "group_left labels promoted correctly",
			content: `
- record: foo
  expr: |
    up{node_status="v", job="node_exporter"}
    * on (colo_name) group_left(tier) colo_metadata{colo_status="v"}
    * on (instance) group_left (sliver) sliver_metadata{node_status="v"}
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "impossible match due to orphaned group_left",
			content: `
- record: foo
  expr: |
    up{node_status="v", job="node_exporter"}
    and on (instance, animal, brand) (metal_services_enabled == 999)
    * on (colo_name) group_left(tier, animal, brand, pop_name) colo_metadata{colo_status="v"}
    * on (instance) group_left (sliver) sliver_metadata{node_status="v"}
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "impossible group_left and",
			content: `
- record: foo
  expr: |
    up{node_status="v", job="node_exporter"}
    and on (colo_name) colo_metadata{colo_status="v"}
    * on (instance) group_left (sliver) sliver_metadata{node_status="v"}
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "possible group_left and",
			content: `
- record: foo
  expr: |
    (
      up{node_status="v", job="node_exporter"}
      and on (colo_name) colo_metadata{colo_status="v"}
    ) * on (instance) group_left (sliver) sliver_metadata{node_status="v"}
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "orphaned group_left labels",
			content: `
- record: foo
  expr: |
    up{node_status="v", job="node_exporter"}
    and on (instance) (metal_services_enabled == 999)
    * on (colo_name) group_left(tier, animal, brand, pop_name) colo_metadata{colo_status="v"}
    * on (instance) group_left (sliver) sliver_metadata{node_status="v"}
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "orphaned group_right labels (except animal)",
			content: `
- record: foo
  expr: |
    colo_metadata{colo_status="v"} * on (colo_name) group_right(tier, animal, brand, pop_name)
    sliver_metadata{node_status="v"} * on (instance) group_right (sliver, animal)
    (metal_services_enabled == 999) * on (instance)
    up{node_status="v", job="node_exporter"}
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "orphaned group_right labels (all)",
			content: `
- record: foo
  expr: |
    colo_metadata{colo_status="v"} * on (colo_name) group_right(tier, animal, brand, pop_name)
    sliver_metadata{node_status="v"} * on (instance) group_right (sliver)
    (metal_services_enabled == 999) * on (instance) group_right()
    up{node_status="v", job="node_exporter"}
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "orphaned group_right labels (except sliver)",
			content: `
- record: foo
  expr: |
    colo_metadata * on (colo_name) group_right(tier, animal, brand, pop_name)
    sliver_metadata * on (instance) group_right (sliver)
    metal_services_enabled
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "correctly passed group_right labels",
			content: `
- record: foo
  expr: |
    colo_metadata * on (colo_name) group_right(tier, animal, brand, pop_name)
    (
        sliver_metadata * on (instance) group_right (sliver)
        metal_services_enabled
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "match on group_right label",
			content: `
- record: foo
  expr: |
    colo_metadata * on (colo_name) group_right(animal)
    sliver_metadata * on (animal) group_right (sliver)
    metal_services_enabled
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "unused joined label",
			content: `
- record: foo
  expr: |
    (prometheus_ready * on(instance) group_left(version) prometheus_build_info)
    * on(instance) prometheus_config_last_reload_successful
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "joined label ignored via ignoring()",
			content: `
- record: foo
  expr: |
    (prometheus_ready * on(instance) group_left(version) prometheus_build_info)
    * ignoring(version) prometheus_config_last_reload_successful
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "joined label NOT ignored via ignoring()",
			content: `
- record: foo
  expr: |
    (prometheus_ready * on(instance) group_left(version) prometheus_build_info)
    * ignoring(env) prometheus_config_last_reload_successful
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "group_left on a label already guaranteed on the left",
			content: `
- record: foo
  expr: |
    up{node_status="v", job="node_exporter"}
    * on(instance) group_left(node_status) sliver_metadata
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "max by() unless",
			content: `
- record: foo
  expr: |
    max by (instance, cluster) (cf_node_role{kubernetes_role="master",role="kubernetes"})
    unless
       sum by (instance, cluster) (time() - node_systemd_timer_last_trigger_seconds{name=~"etcd-defrag-.*.timer"})
       * on (instance) group_left (cluster)
        cf_node_role{kubernetes_role="master",role="kubernetes"}
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "join on impossible label",
			content: `
- record: foo
  expr: |
    sum by (job) (
      up{env="prod", job="foo"} * on () group_left(job) services_enabled{job=""}
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "join on possible label",
			content: `
- record: foo
  expr: |
    sum by (job) (
      up{env="prod"} * on () group_left(job) services_enabled{job!=""}
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "join on possible but redundant label",
			content: `
- record: foo
  expr: |
    sum by (job) (
      up{env="prod", job="foo"} * on () group_left(job) services_enabled{job!=""}
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "group_left inside without",
			content: `
- record: foo
  expr: |
    group without(animal, brand) (
      up{node_status="v", job="node_exporter"}
      * on (colo_name) group_left(tier, animal, brand, pop_name) colo_metadata{colo_status="v"}
      * on (instance) group_left (sliver) sliver_metadata{node_status="v"}
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "group_left inside without but used for inner join",
			content: `
- record: foo
  expr: |
    group without(animal) (
      (
        up{node_status="v", job="node_exporter"}
        * on (colo_name) group_left(animal)
        colo_metadata{colo_status="v"}
      )
      * on(animal, instance)
      (
        up{node_status="v", job="blackholebird"}
        * on (colo_name) group_left(animal)
        colo_metadata{colo_status="v"}
      )
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "group_left inside without but used for inner join but not fully",
			content: `
- record: foo
  expr: |
    group without(animal) (
      (
        up{node_status="v", job="node_exporter"}
        * on (colo_name) group_left(animal)
        colo_metadata{colo_status="v"}
      )
      * on(animal, instance)
      (
        up{node_status="v", job="blackholebird"}
        * on (colo_name) group_left(animal, brand)
        colo_metadata{colo_status="v"}
      )
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "group_left inside without but used for inner without join",
			content: `
- record: foo
  expr: |
    group without(animal) (
      (
        up{node_status="v", job="node_exporter"}
        * on (colo_name) group_left(animal)
        colo_metadata{colo_status="v"}
      )
      * ignoring(colo_status)
      (
        up{node_status="v", job="blackholebird"}
        * on (colo_name) group_left(animal, brand)
        colo_metadata{colo_status="v"}
      )
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "group_left not included in by",
			content: `
- record: foo
  expr: |
    group by(brand) (
      up{node_status="v", job="node_exporter"}
      * on (colo_name) group_left(tier, animal, brand, pop_name) colo_metadata{colo_status="v"}
      * on (instance) group_left (sliver) sliver_metadata{node_status="v"}
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "group_left with empty by",
			content: `
- record: foo
  expr: |
    group (
      up{node_status="v", job="node_exporter"}
      * on (colo_name) group_left(tier, animal, brand, pop_name) colo_metadata{colo_status="v"}
      * on (instance) group_left (sliver) sliver_metadata{node_status="v"}
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "unused group_right labels",
			content: `
- record: foo
  expr: |
    colo_metadata * on (colo_name) group_right(tier, animal, brand, pop_name)
    group by (colo_name, animal) (
        sliver_metadata * on (instance) group_right (sliver)
        metal_services_enabled
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "unused & impossible group_right labels",
			content: `
- record: foo
  expr: |
    colo_metadata * on (colo_name) group_right(tier, animal, brand, pop_name)
    group by (animal) (
        sliver_metadata * on (instance) group_right (sliver)
        metal_services_enabled
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "group_left labels not used in outer join",
			content: `
- record: foo
  expr: |
    up{node_status="v", job="node_exporter"}
    * on(instance) metal_services_enabled
    * on(instance) (
      metal_services_enabled
      * on(instance) group_left (sliver)
      sliver_metadata{node_status="v"}
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "group_right labels not used in outer join",
			content: `
- record: foo
  expr: |
    up{node_status="v", job="node_exporter"}
    * on(instance) metal_services_enabled
    * on(instance) (
      sliver_metadata{node_status="v"}
      * on(instance) group_right (sliver)
      metal_services_enabled
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "group_left labels not used in outer join with another group_left",
			content: `
- record: foo
  expr: |
    up{node_status="v", job="node_exporter"}
    * on(instance) group_left metal_services_enabled
    * on(instance, colo_name) (
      metal_services_enabled
      * on(instance) group_left (sliver)
      sliver_metadata{node_status="v"}
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "group_left labels not used in outer join with another group_right",
			content: `
- record: foo
  expr: |
    up{node_status="v", job="node_exporter"}
    * on(instance) group_right metal_services_enabled
    * on(instance, colo_name) (
      metal_services_enabled
      * on(instance) group_left (sliver)
      sliver_metadata{node_status="v"}
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "label_join",
			content: `
- record: foo
  expr: |
    group by (cluster, namespace, workload, workload_type, pod) (
      label_join(
        label_join(
          group by (cluster, namespace, job_name, pod) (
            label_join(
              kube_pod_owner{job="kube-state-metrics", owner_kind="Job"}
            , "job_name", "", "owner_name")
          )
          * on (cluster, namespace, job_name) group_left(owner_kind, owner_name)
          group by (cluster, namespace, job_name, owner_kind, owner_name) (
            kube_job_owner{job="kube-state-metrics", owner_kind!="Pod", owner_kind!=""}
          )
        , "workload", "", "owner_name")
      , "workload_type", "", "owner_kind")
    )
`,
			checker:    newImpossibleCheck,
			prometheus: newSimpleProm,
		},
	}

	runTests(t, testCases)
}
