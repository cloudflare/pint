package parser_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/parser"
)

func FuzzParse(f *testing.F) {
	testcases := []string{
		`# head comment
- record: foo # record comment
  expr: foo offset 10m # expr comment
  #  pre-labels comment
  labels:
    # pre-foo comment
    foo: bar
    # post-foo comment
    bob: alice
# foot comment
`,
		`
- alert: foo
  annotations: {}
  expr: bar
  annotations: {}
`,
		`- record: name
expr: sum(foo)
labels:
  foo: bar
  bob: alice
`,
		`
groups:
- name: custom_rules
  rules:
    - record: name
      expr: sum(foo)
      labels:
        foo: bar
        bob: alice
`,
		`- alert: Down
  expr: |
    up == 0
  for: |+
    11m
  labels:
    severity: critical
  annotations:
    uri: https://docs.example.com/down.html

- record: >
    foo
  expr: |-
    bar
    /
    baz > 1
  labels: {}
`,
		`- alert: Foo
expr:
  (
	xxx
	-
	yyy
  ) * bar > 0
  and on(instance, device) baz
for: 30m
`,
		`
# pint ignore/begin
{%- set foo = 1 %}
{% set bar = 2 -%}
{# comment #}
{#
  comment 
#}
# pint ignore/end

- record: colo_job:up:count
  expr: sum(foo) without(job)

- record: invalid
  expr: sum(foo) by ())

# pint ignore/begin
- record: colo_job:down:count
  expr: up == {{ foo }}
# pint ignore/end

- record: colo:multiline
  expr: |
    sum(
      multiline
    ) without(job, instance)

- record: colo:multiline:sum
  expr: |
    sum(sum) without(job)
    +
    sum(sum) without(job)

- record: colo:multiline2
  expr: >-
    sum(
      multiline2
    ) without(job, instance)

- record: colo_job:up:byinstance
  expr: sum(byinstance) by(instance)

- record: instance_mode:node_cpu:rate4m
  expr:  sum(rate(node_cpu_seconds_total[4m])) without (cpu)

- record: instance_mode:node_cpu:rate4m
  expr:  sum(rate(node_cpu_seconds_total[5m])) without (cpu)

- record: instance_mode:node_cpu:rate5min
  expr:  sum(irate(node_cpu_seconds_total[5m])) without (cpu)

- alert: Instance Is Down
  expr: up == 0
`,
		`
- record: colo_job:down:count
  expr: up{job=~"foo"} == 0

- record: colo_job:down:count
  expr: up{job!~"foo"} == 0
`,
		`
- record: colo_job:fl_cf_html_bytes_in:rate10m
  expr: sum(rate(fl_cf_html_bytes_in[10m])) WITHOUT (colo_id, instance, node_type, region, node_status, job, colo_name)
- record: colo_job:foo:rate1m
  expr: sum(rate(foo[1m])) WITHOUT (instance)
- record: colo_job:foo:irate3m
  expr: sum(irate(foo[3m])) WITHOUT (colo_id)
`,
		`
xxx:
  xxx:
  xxx:

- xx
- yyy
`,
		`
- record: "colo:test1"
  expr: topk(6, sum(rate(edgeworker_subrequest_errorCount{cordon="free"}[5m])) BY (zoneId,job))
- record: "colo:test2"
  expr: topk(6, sum(rate(edgeworker_subrequest_errorCount{cordon="free"}[10m])) without (instance))
`,
		`- alert: Always
  expr: up
- alert: AlwaysIgnored
  expr: up # pint disable alerts/comparison
  labels:
    severity: warning
  annotations:
    url: "https://wiki.example.com/page/ServiceIsDown.html"
- alert: ServiceIsDown
  expr: up == 0
- alert: ServiceIsDown
  expr: up == 0
  labels:
    severity: bad
  annotations:
    url: bad
- alert: ServiceIsDown
  expr: up == 0
  labels:
    severity: warning
  annotations:
    url: "https://wiki.example.com/page/ServiceIsDown.html"
`,
		`
- alert: Foo Is Down
  expr: up{job="foo"} == 0
  annotations:
    url: "https://wiki.example.com/page/ServiceIsDown.html"
    summary: 'Instance {{ $label.instance }} down'
    func: '{{ $valuexx | xxx }}'
  labels:
    severity: warning
    summary: 'Instance {{ $label.instance }} down'
    func: '{{ $value | xxx }}'
    bar: 'Some {{$value}} value'
    val: '{{ .Value|humanizeDuration }}'
    ignore: '$value is not a variable'
`,
		`groups:
- name: example
  rules:

  # Alert for any instance that is unreachable for >5 minutes.
  - alert: InstanceDown
    expr: up == 0
    for: 5m
    labels:
      severity: page
    annotations:
      summary: "Instance {{ $labels.instance }} down"
      description: "{{ $labels.instance }} of job {{ $labels.job }} has been down for more than 5 minutes."

  # Alert for any instance that has a median request latency >1s.
  - alert: APIHighRequestLatency
    expr: sum by (instance) (http_inprogress_requests) > 0
    for: 10m
    annotations:
      summary: "High request latency on {{ $labels.instance }}"
      description: "{{ $labels.instance }} has a median request latency above 1s (current value: {{ $value }}s)"
`,
		`- alert: Good
expr: up == 0
for: 2m
labels:
 component: foo

alert: Bad
expr: up == 0
for: 2m
labels:
 component: foo
`,
		`
- record: disabled
  expr: sum(errors_total) by ) # pint disable promql/syntax

- record: active
  expr: sum(errors_total) by )

- record: disabled
  # pint disable promql/aggregate(job:true)
  expr: sum(errors_total) without(job)

- record: disabled
  # pint disable promql/aggregate
  expr: sum(errors_total) without(job)

- record: active
  expr: sum(errors_total) without(job)

- alert: disabled
  expr: sum(errors_total) by ) # pint disable promql/syntax

- alert: active
  expr: sum(errors_total) by )

- alert: disabled
  # pint disable promql/aggregate(job:true)
  expr: sum(errors_total) without(job) > 0

- alert: disabled
  # pint disable promql/aggregate
  expr: sum(errors_total) without(job) > 0

- alert: active
  expr: sum(errors_total) without(job)
`,
		`groups:
- name: "haproxy.api_server.rules"
  rules:
    - alert: HaproxyServerHealthcheckFailure
      expr: increase(haproxy_server_check_failures_total[15m]) > 100
      for: 5m
      labels:
        severity: 24x7
      annotations:
        summary: "HAProxy server healthcheck failure (instance {{ $labels.instance }})"
        description: "Some server healthcheck are failing on {{ $labels.server }}\n  VALUE = {{ $value }}\n  LABELS: {{ $labels }}"

`,
	}
	for _, tc := range testcases {
		f.Add(tc)
	}
	p := parser.NewParser()
	f.Fuzz(func(t *testing.T, s string) {
		t.Logf("Parsing: [%s]\n", s)
		_, _ = p.Parse([]byte(s))
	})
}
