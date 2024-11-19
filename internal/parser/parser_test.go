package parser_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/parser"

	"github.com/google/go-cmp/cmp"
)

func TestParse(t *testing.T) {
	type testCaseT struct {
		err     string
		content []byte
		output  []parser.Rule
		strict  bool
	}

	testCases := []testCaseT{
		{
			content: nil,
			output:  nil,
		},
		{
			content: []byte{},
			output:  nil,
		},
		{
			content: []byte(string("! !00 \xf6")),
			output:  nil,
			err:     "error at line 1: yaml: incomplete UTF-8 octet sequence",
		},
		{
			content: []byte("- 0: 0\n  00000000: 000000\n  000000:00000000000: 00000000\n  00000000000:000000: 0000000000000000000000000000000000\n  000000: 0000000\n  expr: |"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 6},
					Error: parser.ParseError{Err: errors.New("incomplete rule, no alert or record key"), Line: 6},
				},
			},
		},
		{
			content: []byte("- record: |\n    multiline\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 2},
				},
			},
		},
		{
			content: []byte("---\n- expr: foo\n  record: foo\n---\n- expr: bar\n"),
			output: []parser.Rule{
				{
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 3, Last: 3},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 2, Last: 2},
								Value: "foo",
							},
						},
					},
					Lines: parser.LineRange{First: 2, Last: 3},
				},
				{
					Lines: parser.LineRange{First: 5, Last: 5},
					Error: parser.ParseError{Err: errors.New("incomplete rule, no alert or record key"), Line: 5},
				},
			},
		},
		{
			content: []byte("- expr: foo\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: errors.New("incomplete rule, no alert or record key"), Line: 1},
				},
			},
		},
		{
			content: []byte("- alert: foo\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 1},
				},
			},
		},
		{
			content: []byte("- alert: foo\n  record: foo\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("got both record and alert keys in a single rule"), Line: 1},
				},
			},
		},
		{
			content: []byte("- record: foo\n  labels:\n    foo: bar\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 3},
					Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 1},
				},
			},
		},
		{
			content: []byte("- record: - foo\n"),
			err:     "error at line 1: yaml: block sequence entries are not allowed in this context",
		},
		{
			content: []byte("- record: foo  expr: sum(\n"),
			err:     "error at line 1: yaml: mapping values are not allowed in this context",
		},
		{
			content: []byte("- record\n\texpr: foo\n"),
			err:     "error at line 2: found a tab character that violates indentation",
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  expr: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("duplicated expr key"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  record: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("duplicated record key"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  alert: bar
  expr: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 3},
					Error: parser.ParseError{Err: errors.New("duplicated alert key"), Line: 3},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  for: 5m
  expr: bar
  for: 1m
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("duplicated for key"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  keep_firing_for: 5m
  expr: bar
  keep_firing_for: 1m
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("duplicated keep_firing_for key"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  labels: {}
  expr: bar
  labels: {}
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("duplicated labels key"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  labels: {}
  expr: bar
  labels: {}
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("duplicated labels key"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  annotations: {}
  expr: bar
  annotations: {}
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("duplicated annotations key"), Line: 5},
				},
			},
		},
		{
			content: []byte("- record: foo\n  expr: foo\n  extra: true\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 3},
					Error: parser.ParseError{Err: errors.New("invalid key(s) found: extra"), Line: 3},
				},
			},
		},
		{
			content: []byte("- record: foo\n  expr: foo offset 10m\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 1, Last: 1},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 2, Last: 2},
								Value: "foo offset 10m",
							},
						},
					},
				},
			},
		},
		{
			content: []byte("- record: foo\n  expr: foo offset -10m\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 1, Last: 1},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 2, Last: 2},
								Value: "foo offset -10m",
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
# pint disable head comment
- record: foo # pint disable record comment
  expr: foo offset 10m # pint disable expr comment
  #  pint disable pre-labels comment
  labels:
    # pint disable pre-foo comment
    foo: bar
    # pint disable post-foo comment
    bob: alice
  # pint disable foot comment
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 3, Last: 10},
					Comments: []comments.Comment{
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "head comment"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "record comment"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "expr comment"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "pre-labels comment"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "foot comment"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "pre-foo comment"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "post-foo comment"},
						},
					},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 3, Last: 3},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 4, Last: 4},
								Value: "foo offset 10m",
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 6, Last: 10},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 6, Last: 6},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "foo",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "bar",
									},
								},
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 10, Last: 10},
										Value: "bob",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 10, Last: 10},
										Value: "alice",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			content: []byte("- record: foo\n  expr: foo[5m] offset 10m\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 1, Last: 1},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 2, Last: 2},
								Value: "foo[5m] offset 10m",
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
- record: name
  expr: sum(foo)
  labels:
    foo: bar
    bob: alice
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 6},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 2, Last: 2},
							Value: "name",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 3, Last: 3},
								Value: "sum(foo)",
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 4, Last: 6},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 4, Last: 4},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 5, Last: 5},
										Value: "foo",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 5, Last: 5},
										Value: "bar",
									},
								},
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 6, Last: 6},
										Value: "bob",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 6, Last: 6},
										Value: "alice",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: custom_rules
  rules:
    - record: name
      expr: sum(foo)
      labels:
        foo: bar
        bob: alice
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 9},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 5, Last: 5},
							Value: "name",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 6, Last: 6},
								Value: "sum(foo)",
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 7, Last: 9},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 7, Last: 7},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "foo",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "bar",
									},
								},
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 9, Last: 9},
										Value: "bob",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 9, Last: 9},
										Value: "alice",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`- alert: Down
  expr: |
    up == 0
  for: |+
    11m
  labels:
    severity: critical
  annotations:
    uri: https://docs.example.com/down.html

- record: foo
  expr: |-
    bar
    /
    baz > 1
  labels: {}
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 9},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Lines: parser.LineRange{First: 1, Last: 1},
							Value: "Down",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 2, Last: 3},
								Value: "up == 0\n",
							},
						},
						For: &parser.YamlNode{
							Lines: parser.LineRange{First: 4, Last: 5},
							Value: "11m\n",
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 6, Last: 7},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 6, Last: 6},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 7, Last: 7},
										Value: "severity",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 7, Last: 7},
										Value: "critical",
									},
								},
							},
						},
						Annotations: &parser.YamlMap{
							Lines: parser.LineRange{First: 8, Last: 9},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 8, Last: 8},
								Value: "annotations",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 9, Last: 9},
										Value: "uri",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 9, Last: 9},
										Value: "https://docs.example.com/down.html",
									},
								},
							},
						},
					},
				},
				{
					Lines: parser.LineRange{First: 11, Last: 16},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 11, Last: 11},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 12, Last: 15},
								Value: "bar\n/\nbaz > 1",
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 16, Last: 16},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 16, Last: 16},
								Value: "labels",
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`- alert: Foo
  expr:
    (
      xxx
      -
      yyy
    ) * bar > 0
    and on(instance, device) baz
  for: 30m
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 9},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Lines: parser.LineRange{First: 1, Last: 1},
							Value: "Foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 2, Last: 8},
								Value: "( xxx - yyy ) * bar > 0 and on(instance, device) baz",
							},
						},
						For: &parser.YamlNode{
							Lines: parser.LineRange{First: 9, Last: 9},
							Value: "30m",
						},
					},
				},
			},
		},
		{
			content: []byte(`---
kind: ConfigMap
apiVersion: v1
metadata:
  name: example-app-alerts
  labels:
    app: example-app
data:
  alerts: |
    groups:
      - name: example-app-alerts
        rules:
          - alert: Example_High_Restart_Rate
            expr: sum(rate(kube_pod_container_status_restarts_total{namespace="example-app"}[5m])) > ( 3/60 )
---
kind: ConfigMap
apiVersion: v1
metadata:
  name: other
  labels:
  app: other
data:
  alerts: |
    groups:
      - name: other alerts
        rules:
          - alert: Example_High_Restart_Rate
            expr: "1"

`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 13, Last: 14},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Lines: parser.LineRange{First: 13, Last: 13},
							Value: "Example_High_Restart_Rate",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 14, Last: 14},
								Value: `sum(rate(kube_pod_container_status_restarts_total{namespace="example-app"}[5m])) > ( 3/60 )`,
							},
						},
					},
				},
				{
					Lines: parser.LineRange{First: 27, Last: 28},
					AlertingRule: &parser.AlertingRule{
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{Value: "1", Lines: parser.LineRange{First: 28, Last: 28}},
						},
						Alert: parser.YamlNode{Value: "Example_High_Restart_Rate", Lines: parser.LineRange{First: 27, Last: 27}},
					},
				},
			},
		},
		{
			content: []byte(`---
kind: ConfigMap
apiVersion: v1
metadata:
  name: example-app-alerts
  labels:
  app: example-app
data:
  alerts: |
    groups:
      - name: example-app-alerts
        rules:
          - alert: Example_Is_Down
            expr: kube_deployment_status_replicas_available{namespace="example-app"} < 1
            for: 5m
            labels:
              priority: "2"
              environment: production
            annotations:
              summary: "No replicas for Example have been running for 5 minutes"

          - alert: Example_High_Restart_Rate
            expr: sum(rate(kube_pod_container_status_restarts_total{namespace="example-app"}[5m])) > ( 3/60 )
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 13, Last: 20},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Lines: parser.LineRange{First: 13, Last: 13},
							Value: "Example_Is_Down",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 14, Last: 14},
								Value: `kube_deployment_status_replicas_available{namespace="example-app"} < 1`,
							},
						},
						For: &parser.YamlNode{
							Lines: parser.LineRange{First: 15, Last: 15},
							Value: "5m",
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 16, Last: 18},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 16, Last: 16},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 17, Last: 17},
										Value: "priority",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 17, Last: 17},
										Value: "2",
									},
								},
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 18, Last: 18},
										Value: "environment",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 18, Last: 18},
										Value: "production",
									},
								},
							},
						},
						Annotations: &parser.YamlMap{
							Lines: parser.LineRange{First: 19, Last: 20},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 19, Last: 19},
								Value: "annotations",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 20, Last: 20},
										Value: "summary",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 20, Last: 20},
										Value: "No replicas for Example have been running for 5 minutes",
									},
								},
							},
						},
					},
				},
				{
					Lines: parser.LineRange{First: 22, Last: 23},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Lines: parser.LineRange{First: 22, Last: 22},
							Value: "Example_High_Restart_Rate",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 23, Last: 23},
								Value: `sum(rate(kube_pod_container_status_restarts_total{namespace="example-app"}[5m])) > ( 3/60 )`,
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`groups:
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
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 4, Last: 13},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Lines: parser.LineRange{First: 4, Last: 4},
							Value: "HaproxyServerHealthcheckFailure",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 5, Last: 5},
								Value: "increase(haproxy_server_check_failures_total[15m]) > 100",
							},
						},
						For: &parser.YamlNode{
							Lines: parser.LineRange{First: 6, Last: 6},
							Value: "5m",
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 7, Last: 8},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 7, Last: 7},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "severity",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "24x7",
									},
								},
							},
						},
						Annotations: &parser.YamlMap{
							Lines: parser.LineRange{First: 9, Last: 13},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 9, Last: 9},
								Value: "annotations",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 10, Last: 10},
										Value: "summary",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 10, Last: 10},
										Value: "HAProxy server healthcheck failure (instance {{ $labels.instance }})",
									},
								},
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 11, Last: 11},
										Value: "description",
									},
									Value: &parser.YamlNode{
										// FIXME https://github.com/cloudflare/pint/issues/20
										// Should be Lines: [11]
										Lines: parser.LineRange{First: 11, Last: 13},
										// Should be `Some ...` since \n should be escaped
										Value: "Some server healthcheck are failing on {{ $labels.server }}\n  VALUE = {{ $value }}\n  LABELS: {{ $labels }}",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`groups:
- name: certmanager
  rules:
  # pint disable before recordAnchor
  - &recordAnchor # pint disable recordAnchor
    record: name1 # pint disable name1
    expr: expr1 # pint disable expr1
    # pint disable after expr1
  - <<: *recordAnchor
    expr: expr2
  - <<: *recordAnchor
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 6, Last: 7},
					Comments: []comments.Comment{
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "before recordAnchor"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "recordAnchor"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "name1"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "after expr1"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "expr1"},
						},
					},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 6, Last: 6},
							Value: "name1",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 7, Last: 7},
								Value: "expr1",
							},
						},
					},
				},
				{
					Comments: []comments.Comment{
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "before recordAnchor"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "recordAnchor"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "name1"},
						},
					},
					Lines: parser.LineRange{First: 6, Last: 10},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 6, Last: 6},
							Value: "name1",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 10, Last: 10},
								Value: "expr2",
							},
						},
					},
				},
				{
					Comments: []comments.Comment{
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "before recordAnchor"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "recordAnchor"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "name1"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "after expr1"},
						},
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "expr1"},
						},
					},
					Lines: parser.LineRange{First: 6, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 6, Last: 6},
							Value: "name1",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 7, Last: 7},
								Value: "expr1",
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`groups:
- name: certmanager
  rules:
  - record: name1
    expr: expr1
    labels: &labelsAnchor
      label1: val1
      label2: val2
  - record: name2
    expr: expr2
    labels: *labelsAnchor
    # pint disable foot comment
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 4, Last: 8},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 4, Last: 4},
							Value: "name1",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 5, Last: 5},
								Value: "expr1",
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 6, Last: 8},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 6, Last: 6},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 7, Last: 7},
										Value: "label1",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 7, Last: 7},
										Value: "val1",
									},
								},
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "label2",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "val2",
									},
								},
							},
						},
					},
				},
				{
					Comments: []comments.Comment{
						{
							Type:  comments.DisableType,
							Value: comments.Disable{Match: "foot comment"},
						},
					},
					Lines: parser.LineRange{First: 9, Last: 11},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 9, Last: 9},
							Value: "name2",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 10, Last: 10},
								Value: "expr2",
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 11, Last: 11},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 11, Last: 11},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 7, Last: 7},
										Value: "label1",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 7, Last: 7},
										Value: "val1",
									},
								},
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "label2",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "val2",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			content: []byte("- alert:\n  expr: vector(1)\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("alert value cannot be empty"), Line: 1},
				},
			},
		},
		{
			content: []byte("- alert: foo\n  expr:\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("expr value cannot be empty"), Line: 2},
				},
			},
		},
		{
			content: []byte("- alert: foo\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 1},
				},
			},
		},
		{
			content: []byte("- record:\n  expr:\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("record value cannot be empty"), Line: 1},
				},
			},
		},
		{
			content: []byte("- record: foo\n  expr:\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("expr value cannot be empty"), Line: 2},
				},
			},
		},
		{
			content: []byte("- record: foo\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 1},
				},
			},
		},
		{
			content: []byte(string(`
# pint file/owner bob
# pint ignore/begin
# pint ignore/end
# pint disable up

- record: foo
  expr: up

# pint file/owner alice

- record: foo
  expr: up

# pint ignore/next-line
`)),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 7, Last: 8},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 7, Last: 7},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 8, Last: 8},
								Value: "up",
							},
						},
					},
				},
				{
					Lines: parser.LineRange{First: 12, Last: 13},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 12, Last: 12},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 13, Last: 13},
								Value: "up",
							},
						},
					},
				},
			},
		},
		{
			content: []byte(string(`
- alert: Template
  expr: &expr up == 0
  labels:
    notify: &maybe_escalate_notify chat-alerts
- alert: Service Down
  expr: *expr
  labels:
    notify: *maybe_escalate_notify
    summary: foo
`)),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Lines: parser.LineRange{First: 2, Last: 2},
							Value: "Template",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 3, Last: 3},
								Value: "up == 0",
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 4, Last: 5},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 4, Last: 4},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 5, Last: 5},
										Value: "notify",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 5, Last: 5},
										Value: "chat-alerts",
									},
								},
							},
						},
					},
				},
				{
					Lines: parser.LineRange{First: 6, Last: 10},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Lines: parser.LineRange{First: 6, Last: 6},
							Value: "Service Down",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 7, Last: 7},
								Value: "up == 0",
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 8, Last: 10},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 8, Last: 8},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 9, Last: 9},
										Value: "notify",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 9, Last: 9},
										Value: "chat-alerts",
									},
								},
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 10, Last: 10},
										Value: "summary",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 10, Last: 10},
										Value: "foo",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
- record: invalid metric name
  expr: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 3},
					Error: parser.ParseError{Err: errors.New("invalid recording rule name: invalid metric name"), Line: 2},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  labels:
    "foo bar": yes
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("invalid label name: foo bar"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  labels:
    "foo bar": yes
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("invalid label name: foo bar"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  labels:
    "{{ $value }}": yes
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("invalid label name: {{ $value }}"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  annotations:
    "foo bar": yes
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("invalid annotation name: foo bar"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  labels:
    foo: ` + string("\xed\xbf\xbf")),
			// Label values are invalid only if they aren't valid UTF-8 strings
			// which also makes them unparsable by YAML.
			err: "error at line 1: yaml: invalid Unicode character",
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  annotations:
    "{{ $value }}": yes
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("invalid annotation name: {{ $value }}"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  keep_firing_for: 5m
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("invalid field 'keep_firing_for' in recording rule"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  for: 5m
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("invalid field 'for' in recording rule"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  annotations:
    foo: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("invalid field 'annotations' in recording rule"), Line: 4},
				},
			},
		},
		// Tag tests
		{
			content: []byte(`
- record: 5
  expr: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 3},
					Error: parser.ParseError{Err: errors.New("record value must be a string, got integer instead"), Line: 2},
				},
			},
		},
		{
			content: []byte(`
- alert: 5
  expr: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 3},
					Error: parser.ParseError{Err: errors.New("alert value must be a string, got integer instead"), Line: 2},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: 5
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 3},
					Error: parser.ParseError{Err: errors.New("expr value must be a string, got integer instead"), Line: 3},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  for: 5
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("for value must be a string, got integer instead"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  keep_firing_for: 5
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("keep_firing_for value must be a string, got integer instead"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  labels: []
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("labels value must be a mapping, got list instead"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  labels: {}
  annotations: []
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("annotations value must be a mapping, got list instead"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  labels:
    foo: 3
  annotations:
    bar: "5"
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 7},
					Error: parser.ParseError{Err: errors.New("labels foo value must be a string, got integer instead"), Line: 5},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  labels: {}
  annotations:
    foo: "3"
    bar: 5
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 7},
					Error: parser.ParseError{Err: errors.New("annotations bar value must be a string, got integer instead"), Line: 7},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  labels: 4
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("labels value must be a mapping, got integer instead"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  labels: true
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("labels value must be a mapping, got bool instead"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  labels: null
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 2, Last: 2},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 3, Last: 3},
								Value: "bar",
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 4, Last: 4},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 4, Last: 4},
								Value: "labels",
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
- record: true
  expr: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 3},
					Error: parser.ParseError{Err: errors.New("record value must be a string, got bool instead"), Line: 2},
				},
			},
		},
		{
			content: []byte(`
- record:
    query: foo
  expr: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("record value must be a string, got mapping instead"), Line: 3},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  labels: some
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("labels value must be a mapping, got string instead"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  labels: !!binary "SGVsbG8sIFdvcmxkIQ=="
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{
						Err:  errors.New("labels value must be a mapping, got binary data instead"),
						Line: 4,
					},
				},
			},
		},
		{
			content: []byte(`
- alert: foo
  expr: bar
  for: 1.23
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{
						Err:  errors.New("for value must be a string, got float instead"),
						Line: 4,
					},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  labels: !!garbage "SGVsbG8sIFdvcmxkIQ=="
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{Err: errors.New("labels value must be a mapping, got garbage instead"), Line: 4},
				},
			},
		},
		{
			content: []byte(`
- record: foo
  expr: bar
  labels: !! "SGVsbG8sIFdvcmxkIQ=="
`),
			err: "error at line 4: did not find expected tag URI",
		},
		{
			content: []byte(`
- record: &foo foo
  expr: bar
  labels: *foo
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 4},
					Error: parser.ParseError{
						Err:  errors.New("labels value must be a mapping, got string instead"),
						Line: 4,
					},
				},
			},
		},
		// Multi-document tests
		{
			content: []byte(`---
- expr: foo
  record: foo
---
- expr: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 3},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 3, Last: 3},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 2, Last: 2},
								Value: "foo",
							},
						},
					},
				},
				{
					Lines: parser.LineRange{First: 5, Last: 5},
					Error: parser.ParseError{
						Err:  errors.New("incomplete rule, no alert or record key"),
						Line: 5,
					},
				},
			},
		},
		{
			content: []byte(`---
- expr: foo
  record: foo
---
- expr: bar
  record: bar
  expr: bar
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 3},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 3, Last: 3},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 2, Last: 2},
								Value: "foo",
							},
						},
					},
				},
				{
					Lines: parser.LineRange{First: 5, Last: 7},
					Error: parser.ParseError{Err: errors.New("duplicated expr key"), Line: 7},
				},
			},
		},
		{
			content: []byte(`---
- expr: foo
  record: foo
---
- expr: bar
  alert: foo
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 2, Last: 3},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 3, Last: 3},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 2, Last: 2},
								Value: "foo",
							},
						},
					},
				},
				{
					Lines: parser.LineRange{First: 5, Last: 6},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Lines: parser.LineRange{First: 6, Last: 6},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 5, Last: 5},
								Value: "bar",
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`---
groups:
- name: v1
  rules:
  - record: up:count
    expr: count(up)
    labels:
      foo:
        bar: foo
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 9},
					Error: parser.ParseError{
						Err:  errors.New("labels foo value must be a string, got mapping instead"),
						Line: 9,
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: v1
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 6},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 5, Last: 5},
							Value: "up:count",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 6, Last: 6},
								Value: "count(up)",
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: v1
  rules:
  - record: up:count
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 5},
					Error: parser.ParseError{
						Err:  errors.New("missing expr key"),
						Line: 5,
					},
				},
			},
		},
		{
			content: []byte(`
- record: up:count
  expr: count(up)
`),
			strict: true,
			err:    "error at line 2: top level field must be a groups key, got list",
		},
		{
			content: []byte(`
rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			err:    "error at line 2: unexpected key rules",
		},
		{
			content: []byte(`
groups:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			err:    "error at line 3: invalid group key record",
		},
		{
			content: []byte(`
groups:
- rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			err:    "error at line 3: incomplete group definition, name is required and must be set",
		},
		{
			content: []byte(`
groups:
- name: foo
`),
			strict: true,
		},
		{
			content: []byte(`
groups: {}
`),
			strict: true,
			err:    "error at line 2: groups value must be a list, got mapping",
		},
		{
			content: []byte(`
groups:
- name: []
`),
			strict: true,
			err:    "error at line 3: group name must be a string, got list",
		},
		{
			content: []byte(`
groups:
- name: foo
  name: bar
  name: bob
`),
			strict: true,
			err:    "error at line 4: duplicated key name",
		},
		{
			content: []byte(`
groups:
- name: v1
  rules:
    rules:
      - record: up:count
        expr: count(up)
`),
			strict: true,
			err:    "error at line 4: rules must be a list, got mapping",
		},
		{
			content: []byte(`
groups:
- name: v1
  rules:
    - rules:
      - record: up:count
        expr: count(up)
`),
			strict: true,
			err:    "error at line 5: invalid rule key rules",
		},
		{
			content: []byte(`
groups:
- name: v1
  rules:
    - rules:
      - record: up:count
		expr: count(up)
`),
			strict: true,
			err:    "error at line 6: found a tab character that violates indentation",
		},
		{
			content: []byte(`
---
groups:
- name: v1
  rules:
    - record: up:count
      expr: count(up)
---
groups:
- name: v1
  rules:
    - rules:
      - record: up:count
        expr: count(up)
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 6, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 6, Last: 6},
							Value: "up:count",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 7, Last: 7},
								Value: "count(up)",
							},
						},
					},
				},
			},
			err: "error at line 12: invalid rule key rules",
		},
		{
			content: []byte(`
---
groups: []
---
groups:
- name: foo
  rules:
    - labels: !!binary "SGVsbG8sIFdvcmxkIQ=="
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 8, Last: 8},
					Error: parser.ParseError{
						Line: 8,
						Err:  errors.New("labels value must be a mapping, got binary data instead"),
					},
				},
				{
					Lines: parser.LineRange{First: 4, Last: 4},
					Error: parser.ParseError{
						Line: 4,
						Err:  errors.New("multi-document YAML files are not allowed"),
					},
				},
			},
		},
		{
			content: []byte("[]"),
			strict:  true,
			err:     "error at line 1: top level field must be a groups key, got list",
		},
		{
			content: []byte("\n\n[]"),
			strict:  true,
			err:     "error at line 3: top level field must be a groups key, got list",
		},
		{
			content: []byte("groups: {}"),
			strict:  true,
			err:     "error at line 1: groups value must be a list, got mapping",
		},
		{
			content: []byte("groups: []"),
			strict:  true,
		},
		{
			content: []byte("xgroups: {}"),
			strict:  true,
			err:     "error at line 1: unexpected key xgroups",
		},
		{
			content: []byte("\nbob\n"),
			strict:  true,
			err:     "error at line 2: top level field must be a groups key, got string",
		},
		{
			content: []byte(`groups: []

rules: []
`),
			strict: true,
			err:    "error at line 3: unexpected key rules",
		},
		{
			content: []byte(`
groups:
- name: foo
  rules: []
`),
			strict: true,
		},
		{
			content: []byte(`
groups:
- name: 
  rules: []
`),
			strict: true,
			err:    "error at line 3: group name must be a string, got null",
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up)
    labels:
      job: foo
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 8},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 5, Last: 5},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 6, Last: 6},
								Value: "sum(up)",
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 7, Last: 8},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 7, Last: 7},
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "job",
									},
									Value: &parser.YamlNode{
										Lines: parser.LineRange{First: 8, Last: 8},
										Value: "foo",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up)
    xxx: 1
    labels:
      job: foo
`),
			strict: true,
			err:    "error at line 7: invalid rule key xxx",
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    record: foo
    expr: sum(up)
    xxx: 1
    labels:
      job: foo
`),
			strict: true,
			err:    "error at line 4: rules must be a list, got mapping",
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up)
    labels:
      job:
        foo: bar
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 9},
					Error: parser.ParseError{
						Line: 9,
						Err:  errors.New("labels job value must be a string, got mapping instead"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr:
      sum: sum(up)
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 7},
					Error: parser.ParseError{
						Line: 7,
						Err:  errors.New("expr value must be a string, got mapping instead"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules: []
- name: foo
  rules: []
`),
			strict: true,
			err:    "error at line 5: duplicated group name",
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up)
    labels:
      foo: bob
      foo: bar
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 8, Last: 9},
					Error: parser.ParseError{
						Line: 9,
						Err:  errors.New("duplicated labels key foo"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: v2
  rules:
  - record: up:count
    expr: count(up)
    expr: sum(up)`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 7},
					Error: parser.ParseError{
						Line: 7,
						Err:  errors.New("duplicated expr key"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: v2
  rules:
  - record: up:count
    expr: count(up)
bogus: 1
`),
			strict: true,
			err:    "error at line 7: unexpected key bogus",
		},
		{
			content: []byte(`
groups:
- name: v2
  rules:
  - record: up:count
    expr: count(up)
    bogus: 1
`),
			strict: true,
			err:    "error at line 7: invalid rule key bogus",
		},
		{
			content: []byte(`
groups:
- name: v2
  rules:
  - alert: up:count
    for: 5m
    keep_firing_for: 5m
    expr: count(up)
    labels: {}
    annotations: {}
    bogus: 1
`),
			strict: true,
			err:    "error at line 11: invalid rule key bogus",
		},
		{
			content: []byte(`
groups:

- name: CloudflareKafkaZookeeperExporter

  rules:
`),
			strict: true,
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    expr: 1
`),
			strict: true,
			err:    "error at line 4: rules must be a list, got mapping",
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - expr: 1
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 5},
					Error: parser.ParseError{
						Line: 5,
						Err:  errors.New("incomplete rule, no alert or record key"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - expr: null
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 5},
					Error: parser.ParseError{
						Line: 5,
						Err:  errors.New("incomplete rule, no alert or record key"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - 1: null
`),
			strict: true,
			err:    "error at line 5: invalid rule key 1",
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - true: !!binary "SGVsbG8sIFdvcmxkIQ=="
`),
			strict: true,
			err:    "error at line 5: invalid rule key true",
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - expr: !!binary "SGVsbG8sIFdvcmxkIQ=="
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 5},
					Error: parser.ParseError{
						Line: 5,
						Err:  errors.New("incomplete rule, no alert or record key"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - expr: !!binary "SGVsbG8sIFdvcmxkIQ=="
      record: foo
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 6},
					Error: parser.ParseError{
						Line: 5,
						Err:  errors.New("expr value must be a string, got binary data instead"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - labels: !!binary "SGVsbG8sIFdvcmxkIQ=="
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 5},
					Error: parser.ParseError{
						Line: 5,
						Err:  errors.New("labels value must be a mapping, got binary data instead"),
					},
				},
			},
		},
		{
			content: []byte(`
---
groups:
- name: foo
  rules:
    - record: foo
      expr: bar
---
groups:
- name: foo
  rules:
    - record: foo
      expr: bar
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 6, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 6, Last: 6},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 7, Last: 7},
								Value: "bar",
							},
						},
					},
				},
				{
					Lines: parser.LineRange{First: 12, Last: 13},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 12, Last: 12},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 13, Last: 13},
								Value: "bar",
							},
						},
					},
				},
				{
					Lines: parser.LineRange{First: 8, Last: 8},
					Error: parser.ParseError{
						Line: 8,
						Err:  errors.New("multi-document YAML files are not allowed"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - record: foo
      expr: foo
      expr: foo
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 7},
					Error: parser.ParseError{
						Line: 7,
						Err:  errors.New("duplicated expr key"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - record: foo
      keep_firing_for: 1m
      keep_firing_for: 2m
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 7},
					Error: parser.ParseError{
						Line: 7,
						Err:  errors.New("duplicated keep_firing_for key"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - record: foo
      keep_firing_for: 1m
      record: 2m
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 7},
					Error: parser.ParseError{
						Line: 7,
						Err:  errors.New("duplicated record key"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
    - []
`),
			strict: true,
			err:    "error at line 5: rule definion must be a mapping, got list",
		},
		{
			content: []byte(`
1: 0
`),
			strict: true,
			err:    "error at line 2: groups key must be a string, got a integer",
		},
		{
			content: []byte(`
true: 0
`),
			strict: true,
			err:    "error at line 2: groups key must be a string, got a bool",
		},
		{
			content: []byte(`
groups: !!binary "SGVsbG8sIFdvcmxkIQ=="
`),
			strict: true,
			err:    "error at line 2: groups value must be a list, got binary data",
		},
		{
			content: []byte(`
groups:
  - true: null"
`),
			strict: true,
			err:    "error at line 3: invalid group key true",
		},
		{
			content: []byte(`
!!!binary "groups": true"
`),
			strict: true,
			err:    "error at line 2: groups key must be a string, got a binary",
		},
		{
			content: []byte("[]"),
			strict:  true,
			err:     "error at line 1: top level field must be a groups key, got list",
		},
		{
			content: []byte(`
groups:
  - true
`),
			strict: true,
			err:    "error at line 3: group must be a mapping, got bool",
		},
		{
			content: []byte(`
groups:
  - name:
    rules: []
`),
			strict: true,
			err:    "error at line 3: group name must be a string, got null",
		},
		{
			content: []byte(`
groups:
  - name: ""
    rules: []
`),
			strict: true,
			err:    "error at line 3: group name cannot be empty",
		},
		{
			content: []byte(`
groups:
  - name: 1
    rules: []
`),
			strict: true,
			err:    "error at line 3: group name must be a string, got integer",
		},
		{
			content: []byte(`
groups:
  - name: foo
    interval: 1
    rules: []
`),
			strict: true,
			err:    "error at line 4: group interval must be a string, got integer",
		},
		{
			content: []byte(`
groups:
  - name: foo
    interval: xxx
    rules: []
`),
			strict: true,
			err:    `error at line 4: invalid interval value: not a valid duration string: "xxx"`,
		},
		{
			content: []byte(`
groups:
  - name: foo
    query_offset: 1
    rules: []
`),
			strict: true,
			err:    "error at line 4: group query_offset must be a string, got integer",
		},
		{
			content: []byte(`
groups:
  - name: foo
    query_offset: xxx
    rules: []
`),
			strict: true,
			err:    `error at line 4: invalid query_offset value: not a valid duration string: "xxx"`,
		},
		{
			content: []byte(`
groups:
  - name: foo
    query_offset: 1m
    limit: abc
    rules: []
`),
			strict: true,
			err:    "error at line 5: group limit must be a integer, got string",
		},
		{
			content: []byte(`
groups:
- name: v2
  rules:
  - alert: up:count
    for: 5m &timeout
    keep_firing_for: **timeout
    expr: count(up)
    labels: {}
    annotations: {}
    bogus: 1
`),
			strict: true,
			err:    "error at line 7: did not find expected alphabetic or numeric character",
		},
		{
			content: []byte(`
groups:
- name: v2
  rules:
  - alert: up:count
    for: &for 1
    keep_firing_for: *for
    expr: count(up)
    labels: {}
    annotations: {}
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 10},
					Error: parser.ParseError{
						Line: 6,
						Err:  errors.New("for value must be a string, got integer instead"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: v2
  limit: &for 1
  rules:
  - alert: up:count
    keep_firing_for: *for
    expr: count(up)
    labels: {}
    annotations: {}
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 6, Last: 10},
					Error: parser.ParseError{
						Line: 7,
						Err:  errors.New("keep_firing_for value must be a string, got integer instead"),
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: "{{ source }}"
  rules:
# pint ignore/begin
    
# pint ignore/end
`),
			strict: true,
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: |
      {"up"}
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 5, Last: 5},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 6, Last: 7},
								Value: "{\"up\"}\n",
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: |
      {'up'}
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 5, Last: 5},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 6, Last: 7},
								Value: "{'up'}\n",
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: |
      {'up' == 1}
`),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 5, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 5, Last: 5},
							Value: "foo",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 6, Last: 7},
								Value: "{'up' == 1}\n",
							},
							SyntaxError: errors.New("unexpected character inside braces: '1'"),
						},
					},
				},
			},
		},
	}

	alwaysEqual := cmp.Comparer(func(_, _ interface{}) bool { return true })
	ignorePrometheusExpr := cmp.FilterValues(func(x, y interface{}) bool {
		_, xe := x.(*parser.PromQLNode)
		_, ye := y.(*parser.PromQLNode)
		return xe || ye
	}, alwaysEqual)

	cmpErrorText := cmp.Comparer(func(x, y interface{}) bool {
		xe := x.(error)
		ye := y.(error)
		return xe.Error() == ye.Error()
	})
	sameErrorText := cmp.FilterValues(func(x, y interface{}) bool {
		_, xe := x.(error)
		_, ye := y.(error)
		return xe && ye
	}, cmpErrorText)

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			t.Logf("\n--- Content ---%s--- END ---", tc.content)

			p := parser.NewParser(tc.strict)
			output, err := p.Parse(tc.content)

			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}

			if diff := cmp.Diff(tc.output, output, ignorePrometheusExpr, sameErrorText); diff != "" {
				t.Errorf("Parse() returned wrong output (-want +got):\n%s", diff)
				return
			}
		})
	}
}
