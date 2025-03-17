package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newTemplateCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewTemplateCheck()
}

func TestTemplateCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "skips recording rule",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
		},
		{
			description: "invalid syntax in annotations",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  annotations:\n    summary: 'Instance {{ $label.instance }} down'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "invalid function in annotations",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  annotations:\n    summary: '{{ $value | xxx }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "valid syntax in annotations",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  annotations:\n    summary: 'Instance {{ $labels.instance }} down'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
		},
		{
			description: "invalid syntax in labels",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  labels:\n    summary: 'Instance {{ $label.instance }} down'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "invalid function in annotations",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  labels:\n    summary: '{{ $value | xxx }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "valid syntax in labels",
			content:     "- alert: Foo Is Down\n  expr: up{job=\"foo\"} == 0\n  labels:\n    summary: 'Instance {{ $labels.instance }} down'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
		},
		{
			description: "{{$value}} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: '{{$value}}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "{{$value}} in multiple labels",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: '{{ .Value }}'\n    baz: '{{$value}}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "{{  $value  }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: |\n      foo is {{  $value | humanizePercentage }}%\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "{{  $value  }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: |\n      foo is {{$value|humanizePercentage}}%\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "{{ .Value }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: 'value {{ .Value }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "{{ .Value|humanize }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: '{{ .Value|humanize }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "{{ $foo := $value }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: '{{ $foo := $value }}{{ $foo }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "{{ $foo := .Value }} in label value",
			content:     "- alert: foo\n  expr: sum(foo)\n  labels:\n    foo: bar\n    baz: '{{ $foo := .Value }}{{ $foo }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "annotation label missing from metrics (by)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) > 0\n  annotations:\n    summary: '{{ $labels.job }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "annotation label missing from metrics (by)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) > 0\n  annotations:\n    summary: '{{ .Labels.job }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "annotation label missing from metrics (without)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) without(job) > 0\n  annotations:\n    summary: '{{ $labels.job }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "annotation label missing from metrics (without)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) without(job) > 0\n  annotations:\n    summary: '{{ .Labels.job }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "label missing from metrics (without)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) without(job) > 0\n  labels:\n    summary: '{{ $labels.job }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "annotation label missing from metrics (or)",
			content:     "- alert: Foo Is Down\n  expr: sum(foo) by(job) or sum(bar)\n  annotations:\n    summary: '{{ .Labels.job }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "annotation label missing from metrics (1+)",
			content:     "- alert: Foo Is Down\n  expr: 1 + sum(foo) by(notjob)\n  annotations:\n    summary: '{{ .Labels.job }}'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "annotation label missing from metrics (group_left)",
			content: `
- alert: Foo Is Down
  expr: count(build_info) by (instance, version) != ignoring(package) group_left(foo) count(package_installed) by (instance, version, package)
  annotations:
    summary: '{{ $labels.instance }} on {{ .Labels.foo }} is down'
    help: '{{ $labels.ixtance }}'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "don't trigger for label_replace() provided labels",
			content: `
- alert: label_replace_not_checked_correctly
  expr: |
    label_replace(
      sum by (pod) (pod_status) > 0
      ,"cluster", "$1", "pod", "(.*)"
    )
  annotations:
    summary: "Some error found in {{ $labels.cluster }}"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "annotation label present on metrics (absent)",
			content: `
- alert: Foo Is Missing
  expr: absent(foo{job="bar", instance="server1"})
  annotations:
    summary: '{{ $labels.instance }} on {{ .Labels.job }} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "annotation label missing from metrics (absent, and)",
			content: `
- alert: Foo Is Missing
  expr: absent(foo{job="bar"}) AND on(job) foo
  labels:
    instance: '{{ $labels.instance }}'
  annotations:
    summary: '{{ $labels.instance }} on {{ .Labels.foo }} is missing'
    help: '{{ $labels.xxx }}'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "annotation label present on metrics (absent(sum))",
			content: `
- alert: Foo Is Missing
  expr: absent(sum(foo) by(job, instance))
  annotations:
    summary: '{{ $labels.instance }} on {{ .Labels.job }} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "annotation label missing from metrics (absent(sum))",
			content: `
- alert: Foo Is Missing
  expr: absent(sum(foo) by(job))
  annotations:
    summary: '{{ $labels.instance }} on {{ .Labels.job }} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "annotation label missing from metrics (absent({job=~}))",
			content: `
- alert: Foo Is Missing
  expr: absent({job=~".+"})
  annotations:
    summary: '{{ .Labels.job }} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "annotation label missing from metrics (absent()) / multiple",
			content: `
- alert: Foo Is Missing
  expr: absent(foo) or absent(bar)
  annotations:
    summary: '{{ .Labels.job }} / {{$labels.job}} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "absent() * on() group_left(...) foo",
			content: `
- alert: Foo
  expr: absent(foo{job="xxx"}) * on() group_left(cluster, env) bar
  annotations:
    summary: '{{ .Labels.job }} in cluster {{$labels.cluster}}/{{ $labels.env }} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "absent() * on() group_left() bar",
			content: `
- alert: Foo
  expr: absent(foo{job="xxx"}) * on() group_left() bar
  annotations:
    summary: '{{ .Labels.job }} in cluster {{$labels.cluster}}/{{ $labels.env }} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "bar * on() group_right(...) absent()",
			content: `
- alert: Foo
  expr: bar * on() group_right(cluster, env) absent(foo{job="xxx"})
  annotations:
    summary: '{{ .Labels.job }} in cluster {{$labels.cluster}}/{{ $labels.env }} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "bar * on() group_right() absent()",
			content: `
- alert: Foo
  expr: bar * on() group_right() absent(foo{job="xxx"})
  annotations:
    summary: '{{ .Labels.job }} in cluster {{$labels.cluster}}/{{ $labels.env }} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "foo and on() absent(bar)",
			content: `
- alert: Foo
  expr: foo and on() absent(bar)
  annotations:
    summary: '{{ .Labels.job }} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "no humanize on rate()",
			content: `
- alert: Foo
  expr: rate(errors[2m]) > 0
  annotations:
    summary: "Seeing {{ $value }} errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "no humanize on rate() / alias",
			content: `
- alert: Foo
  expr: rate(errors[2m]) > 0
  annotations:
    summary: "{{ $foo := $value }}{{ $bar := $foo }} Seeing {{ $bar }} errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "no humanize on irate()",
			content: `
- alert: Foo
  expr: irate(errors[2m]) > 0
  annotations:
    summary: "Seeing {{ .Value }} errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "no humanize on irate()",
			content: `
- alert: Foo
  expr: deriv(errors[2m]) > 0
  annotations:
    summary: "Seeing {{ .Value }} errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "rate() but no $value",
			content: `
- alert: Foo
  expr: rate(errors[2m]) > 0
  annotations:
    summary: "Seeing errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "humanize passed to value",
			content: `
- alert: Foo
  expr: rate(errors[2m]) > 0
  annotations:
    summary: "Seeing {{ $value | humanize }} errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "humanizePercentage passed to value",
			content: `
- alert: Foo
  expr: (sum(rate(errors[2m])) / sum(rate(requests[2m]))) > 0.1
  annotations:
    summary: "Seeing {{ $value | humanizePercentage }} errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "humanizeDuration passed to value",
			content: `
- alert: Foo
  expr: (sum(rate(errors[2m])) / sum(rate(requests[2m]))) > 0.1
  annotations:
    summary: "Seeing {{ $value | humanizeDuration }} errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "humanize not needed on count()",
			content: `
- alert: Foo
  expr: count(rate(errors[2m]) > 0) > 0
  annotations:
    summary: "Seeing {{ $value }} instances with errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "humanize not needed on rate() used in RHS",
			content: `
- alert: Foo
  expr: foo > on() sum(rate(errors[2m])
  annotations:
    summary: "Seeing {{ $value }} instances with errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "humanize not needed on round(rate())",
			content: `
- alert: Foo
  expr: round(rate(errors_total[5m]), 1) > 0
  annotations:
    summary: "Seeing {{ $value }} instances with errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "humanize not needed on wjen using printf %.2f",
			content: `
- alert: Foo
  expr: rate(errors_total[5m]) > 0
  annotations:
    summary: Seeing {{ printf "%.2f" $value }} instances with errors
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "humanize not needed on wjen using printf %f",
			content: `
- alert: Foo
  expr: rate(errors_total[5m]) > 0
  annotations:
    summary: Seeing {{ printf "%f" $value }} instances with errors
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "humanize still needed for printf on another value",
			content: `
- alert: Foo
  expr: rate(errors_total[5m]) > 0
  annotations:
    summary: Seeing {{ printf "%f" 2 }}{{ $value }} instances with errors
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "toTime",
			content: `
- alert: Foo
  expr: up == 0
  annotations:
    summary: "{{ $value | toTime }}"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "template query with syntax error",
			content: `
- alert: Foo
  expr: up == 0
  annotations:
    summary: |
      {{ with printf "sum({job='%s'}) by(" .Labels.job | query }}
      {{ . | first | label "instance" }}
      {{ end }}
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "template query with bogus function",
			content: `
- alert: Foo
  expr: up == 0
  annotations:
    summary: |
      {{ with printf "suz({job='%s'})" .Labels.job | query }}
      {{ . | first | label "instance" }}
      {{ end }}
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "$value | first",
			content: `
- alert: Foo
  expr: rate(errors[2m])
  annotations:
    summary: "{{ $value | first }} errors"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "template query with bogus range",
			content: `
- alert: Foo
  expr: up == 0
  annotations:
    summary: |
      {{ range query "up xxx" }}
      {{ .Labels.instance }} {{ .Value }}
      {{ end }}
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "template query with valid expr",
			content: `
- alert: Foo
  expr: up{job="bar"} == 0
  annotations:
    summary: Instance {{ printf "up{job='bar', instance='%s'}" $labels.instance | query | first | value }} is down'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		/*
					TODO
					{
						description: "template query removes instance",
						content: `
			- alert: Foo
			  expr: up == 0
			  annotations:
			    summary: |
			      {{ with printf "sum({job='%s'})" .Labels.job | query }}
			      {{ . | first | label "instance" }}
			      {{ end }}
			`,
						checker:    newTemplateCheck,
						prometheus: noProm,
						problems: func(_ string) []checks.Problem {
							return []checks.Problem{
								{
												    {{ with printf "sum({job='%s'})" .Labels.job | query }}
			    {{ . | first | label "instance" }}`,
									Reporter: checks.TemplateCheckName,
									Text:     `"summary" annotation template sends a query that is using "instance" label but that query removes it`,
									Severity: checks.Bug,
								},
							}
						},
					},
		*/
		{
			description: "sub aggregation",
			content: `
- alert: Foo
  expr: |
    (
      sum(foo:sum > 0) without(notify)
      * on(job) group_left(notify)
      job:notify
    )
    and on(job)
    sum(foo:count) by(job) > 20
  labels:
    notify: "{{ $labels.notify }}"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "abs / scalar",
			content: `
- alert: ScyllaNonBalancedcqlTraffic
  expr: >
    abs(rate(scylla_cql_updates{conditional="no"}[1m]) - scalar(avg(rate(scylla_cql_updates{conditional="no"}[1m]))))
    /
    scalar(stddev(rate(scylla_cql_updates{conditional="no"}[1m])) + 100) > 2
  for: 10s
  labels:
    advisor: balanced
    dashboard: cql
    severity: moderate
    status: "1"
    team: team_devops
  annotations:
    description: CQL queries are not balanced among shards {{ $labels.instance }} shard {{ $labels.shard }}
    summary: CQL queries are not balanced
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "annotation label from vector(0)",
			content:     "- alert: DeadMansSwitch\n  expr: vector(1)\n  annotations:\n    summary: 'Deadmans switch on {{ $labels.instance }} is firing'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "labels label from vector(0)",
			content:     "- alert: DeadMansSwitch\n  expr: vector(1)\n  labels:\n    summary: 'Deadmans switch on {{ $labels.instance }} is firing'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "annotation label from number",
			content:     "- alert: DeadMansSwitch\n  expr: 1 > bool 0\n  annotations:\n    summary: 'Deadmans switch on {{ $labels.instance }} / {{ $labels.job }} is firing'\n",
			checker:     newTemplateCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "foo / on(...) bar",
			content: `- alert: Foo
  expr: container_file_descriptors / on (instance, app_name) container_ulimits_soft{ulimit="max_open_files"}
  annotations:
    summary: "{{ $labels.app_type }} is using {{ $value }} fds."
  labels:
    job: "{{ $labels.job_name }}"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "multiple or",
			content: `
- alert: Foo
  expr: >
    avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"})
    < 0.5 > 0
    or avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*multicolo.*"})
    < 0.4 > 0
    or sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*offpeak.*"})
    < 8 > 0
    or sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"})
    < on() group_left() count(colo_router_tier:disabled_pops:max{tier="1",router=~"edge.*"}) * 0.4 > 0
    or avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"})
    < 0.1 > 0
    or avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*brat.*",cidr_use_case!~".*tier1.*",plan=~".*(free|pro).*"})
    <  0.1 > 0
    or sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*utopia.*"})
    < 5 > 0
  annotations:
    dashboard: 'Prefix is {{ $labels.prefix }}'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "multiple or / missing group_left()",
			content: `
- alert: Foo
  expr: >
    avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"})
    < 0.5 > 0
    or avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*multicolo.*"})
    < 0.4 > 0
    or sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*offpeak.*"})
    < 8 > 0
    or sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"})
    < on() count(colo_router_tier:disabled_pops:max{tier="1",router=~"edge.*"}) * 0.4 > 0
    or avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"})
    < 0.1 > 0
    or avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*brat.*",cidr_use_case!~".*tier1.*",plan=~".*(free|pro).*"})
    <  0.1 > 0
    or sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*utopia.*"})
    < 5 > 0
  annotations:
    dashboard: 'Prefix is {{ $labels.prefix }}'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "time - metric",
			content: `
- alert: Foo
  expr: (time() - foo_timestamp_unix) > 5*3600
  labels:
    notify: "{{ $labels.notify }}"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
		{
			description: "bar * ignoring(job) foo",
			content: `
- alert: Foo
  expr: bar * ignoring(job) foo
  annotations:
    summary: '{{ .Labels.job }} in cluster {{$labels.cluster}}/{{ $labels.env }} is missing'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "metric or (metric or vector)",
			content: `
- alert: Foo
  expr: |
    max without (instance) (metric1{exported_job="abc"}) == 0 or (metric2 OR on() vector(0)) == 0
  for: 15m
  annotations:
    summary: 'Foo is down in {{ $labels.colo_name }}'
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "ignore dead code",
			content: `
- alert: Foo
  expr: sum by (region, target, colo_name) (sum_over_time(probe_success{job="abc"}[5m]) or vector(1)) == 0
  for: 5m
  annotations:
    summary: "Probe from {{ $labels.region }} to {{ $labels.target }} failed for the last 5m"
`,
			checker:    newTemplateCheck,
			prometheus: noProm,
		},
	}
	runTests(t, testCases)
}
