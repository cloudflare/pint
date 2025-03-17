package parser_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/parser"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParse(t *testing.T) {
	type testCaseT struct {
		err     string
		content []byte
		output  []parser.Rule
		strict  bool
		schema  parser.Schema
		names   model.ValidationScheme
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
					Lines: diags.LineRange{First: 1, Last: 6},
					Error: parser.ParseError{Err: errors.New("incomplete rule, no alert or record key"), Line: 6},
				},
			},
		},
		{
			content: []byte("- record: |\n    multiline\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 2},
				},
			},
		},
		{
			content: []byte(`---
- expr: foo
  record: foo
---
- expr: bar
`),
			output: []parser.Rule{
				{
					RecordingRule: &parser.RecordingRule{
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "foo",
								Pos: diags.PositionRanges{
									{Line: 2, FirstColumn: 9, LastColumn: 11},
								},
							},
						},
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 3, FirstColumn: 11, LastColumn: 13},
							},
						},
					},
					Lines: diags.LineRange{First: 2, Last: 3},
				},
				{
					Lines: diags.LineRange{First: 5, Last: 5},
					Error: parser.ParseError{Err: errors.New("incomplete rule, no alert or record key"), Line: 5},
				},
			},
		},
		{
			content: []byte("- expr: foo\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: errors.New("incomplete rule, no alert or record key"), Line: 1},
				},
			},
		},
		{
			content: []byte("- alert: foo\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 1},
				},
			},
		},
		{
			content: []byte("- alert: foo\n  record: foo\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("got both record and alert keys in a single rule"), Line: 1},
				},
			},
		},
		{
			content: []byte("- record: foo\n  labels:\n    foo: bar\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 3},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 3},
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
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 5},
					Error: parser.ParseError{Err: errors.New("duplicated annotations key"), Line: 5},
				},
			},
		},
		{
			content: []byte("- record: foo\n  expr: foo\n  extra: true\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 3},
					Error: parser.ParseError{Err: errors.New("invalid key(s) found: extra"), Line: 3},
				},
			},
		},
		{
			content: []byte(`- record: foo
  expr: foo offset 10m
`),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 2},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 1, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Pos: diags.PositionRanges{
									{Line: 2, FirstColumn: 9, LastColumn: 22},
								},
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
					Lines: diags.LineRange{First: 1, Last: 2},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 1, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "foo offset -10m",
								Pos: diags.PositionRanges{
									{Line: 2, FirstColumn: 9, LastColumn: 23},
								},
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
					Lines: diags.LineRange{First: 3, Last: 10},
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
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 3, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "foo offset 10m",
								Pos: diags.PositionRanges{
									{Line: 4, FirstColumn: 9, LastColumn: 22},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "foo",
									},
									Value: &parser.YamlNode{
										Value: "bar",
									},
								},
								{
									Key: &parser.YamlNode{
										Value: "bob",
									},
									Value: &parser.YamlNode{
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
					Lines: diags.LineRange{First: 1, Last: 2},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 1, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "foo[5m] offset 10m",
								Pos: diags.PositionRanges{
									{Line: 2, FirstColumn: 9, LastColumn: 26},
								},
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
					Lines: diags.LineRange{First: 2, Last: 6},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "name",
							Pos: diags.PositionRanges{
								{Line: 2, FirstColumn: 11, LastColumn: 14},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "sum(foo)",
								Pos: diags.PositionRanges{
									{Line: 3, FirstColumn: 9, LastColumn: 16},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "foo",
									},
									Value: &parser.YamlNode{
										Value: "bar",
									},
								},
								{
									Key: &parser.YamlNode{
										Value: "bob",
									},
									Value: &parser.YamlNode{
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
					Lines: diags.LineRange{First: 5, Last: 9},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "name",
							Pos: diags.PositionRanges{
								{Line: 5, FirstColumn: 15, LastColumn: 18},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "sum(foo)",
								Pos: diags.PositionRanges{
									{Line: 6, FirstColumn: 13, LastColumn: 20},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "foo",
									},
									Value: &parser.YamlNode{
										Value: "bar",
									},
								},
								{
									Key: &parser.YamlNode{
										Value: "bob",
									},
									Value: &parser.YamlNode{
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
					Lines: diags.LineRange{First: 1, Last: 9},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "Down",
							Pos: diags.PositionRanges{
								{Line: 1, FirstColumn: 10, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "up == 0\n",
								Pos: diags.PositionRanges{
									{Line: 3, FirstColumn: 5, LastColumn: 11},
								},
							},
						},
						For: &parser.YamlNode{
							Value: "11m\n",
							Pos: diags.PositionRanges{
								{Line: 5, FirstColumn: 5, LastColumn: 7},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "severity",
									},
									Value: &parser.YamlNode{
										Value: "critical",
									},
								},
							},
						},
						Annotations: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "annotations",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "uri",
									},
									Value: &parser.YamlNode{
										Value: "https://docs.example.com/down.html",
									},
								},
							},
						},
					},
				},
				{
					Lines: diags.LineRange{First: 11, Last: 16},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 11, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "bar\n/\nbaz > 1",
								Pos: diags.PositionRanges{
									{Line: 13, FirstColumn: 5, LastColumn: 8},
									{Line: 14, FirstColumn: 5, LastColumn: 6},
									{Line: 15, FirstColumn: 5, LastColumn: 11},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
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
					Lines: diags.LineRange{First: 1, Last: 9},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "Foo",
							Pos: diags.PositionRanges{
								{Line: 1, FirstColumn: 10, LastColumn: 12},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "( xxx - yyy ) * bar > 0 and on(instance, device) baz",
								Pos: diags.PositionRanges{
									{Line: 3, FirstColumn: 5, LastColumn: 6},
									{Line: 4, FirstColumn: 7, LastColumn: 10},
									{Line: 5, FirstColumn: 7, LastColumn: 8},
									{Line: 6, FirstColumn: 7, LastColumn: 10},
									{Line: 7, FirstColumn: 5, LastColumn: 16},
									{Line: 8, FirstColumn: 5, LastColumn: 32},
								},
							},
						},
						For: &parser.YamlNode{
							Value: "30m",
							Pos: diags.PositionRanges{
								{Line: 9, FirstColumn: 8, LastColumn: 10},
							},
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
					Lines: diags.LineRange{First: 13, Last: 14},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "Example_High_Restart_Rate",
							Pos: diags.PositionRanges{
								{Line: 13, FirstColumn: 20, LastColumn: 44},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: `sum(rate(kube_pod_container_status_restarts_total{namespace="example-app"}[5m])) > ( 3/60 )`,
								Pos: diags.PositionRanges{
									{Line: 14, FirstColumn: 19, LastColumn: 109},
								},
							},
						},
					},
				},
				{
					Lines: diags.LineRange{First: 27, Last: 28},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "Example_High_Restart_Rate",
							Pos: diags.PositionRanges{
								{Line: 27, FirstColumn: 20, LastColumn: 44},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "1",
								Pos: diags.PositionRanges{
									{Line: 28, FirstColumn: 20, LastColumn: 20},
								},
							},
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
					Lines: diags.LineRange{First: 13, Last: 20},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "Example_Is_Down",
							Pos: diags.PositionRanges{
								{Line: 13, FirstColumn: 20, LastColumn: 34},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: `kube_deployment_status_replicas_available{namespace="example-app"} < 1`,
								Pos: diags.PositionRanges{
									{Line: 14, FirstColumn: 19, LastColumn: 88},
								},
							},
						},
						For: &parser.YamlNode{
							Value: "5m",
							Pos: diags.PositionRanges{
								{Line: 15, FirstColumn: 18, LastColumn: 19},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "priority",
									},
									Value: &parser.YamlNode{
										Value: "2",
									},
								},
								{
									Key: &parser.YamlNode{
										Value: "environment",
									},
									Value: &parser.YamlNode{
										Value: "production",
									},
								},
							},
						},
						Annotations: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "annotations",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "summary",
									},
									Value: &parser.YamlNode{
										Value: "No replicas for Example have been running for 5 minutes",
									},
								},
							},
						},
					},
				},
				{
					Lines: diags.LineRange{First: 22, Last: 23},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "Example_High_Restart_Rate",
							Pos: diags.PositionRanges{
								{Line: 22, FirstColumn: 20, LastColumn: 44},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: `sum(rate(kube_pod_container_status_restarts_total{namespace="example-app"}[5m])) > ( 3/60 )`,
								Pos: diags.PositionRanges{
									{Line: 23, FirstColumn: 19, LastColumn: 109},
								},
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
					Lines: diags.LineRange{First: 4, Last: 11},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "HaproxyServerHealthcheckFailure",
							Pos: diags.PositionRanges{
								{Line: 4, FirstColumn: 12, LastColumn: 42},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "increase(haproxy_server_check_failures_total[15m]) > 100",
								Pos: diags.PositionRanges{
									{Line: 5, FirstColumn: 11, LastColumn: 66},
								},
							},
						},
						For: &parser.YamlNode{
							Value: "5m",
							Pos: diags.PositionRanges{
								{Line: 6, FirstColumn: 10, LastColumn: 11},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "severity",
									},
									Value: &parser.YamlNode{
										Value: "24x7",
									},
								},
							},
						},
						Annotations: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "annotations",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "summary",
									},
									Value: &parser.YamlNode{
										Value: "HAProxy server healthcheck failure (instance {{ $labels.instance }})",
									},
								},
								{
									Key: &parser.YamlNode{
										Value: "description",
									},
									Value: &parser.YamlNode{
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
					Lines: diags.LineRange{First: 6, Last: 7},
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
							Value: "name1",
							Pos: diags.PositionRanges{
								{Line: 6, FirstColumn: 13, LastColumn: 17},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "expr1",
								Pos: diags.PositionRanges{
									{Line: 7, FirstColumn: 11, LastColumn: 15},
								},
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
					Lines: diags.LineRange{First: 6, Last: 10},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "name1",
							Pos: diags.PositionRanges{
								{Line: 6, FirstColumn: 13, LastColumn: 17},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "expr2",
								Pos: diags.PositionRanges{
									{Line: 10, FirstColumn: 11, LastColumn: 15},
								},
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
					Lines: diags.LineRange{First: 6, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "name1",
							Pos: diags.PositionRanges{
								{Line: 6, FirstColumn: 13, LastColumn: 17},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "expr1",
								Pos: diags.PositionRanges{
									{Line: 7, FirstColumn: 11, LastColumn: 15},
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
					Lines: diags.LineRange{First: 4, Last: 8},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "name1",
							Pos: diags.PositionRanges{
								{Line: 4, FirstColumn: 13, LastColumn: 17},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "expr1",
								Pos: diags.PositionRanges{
									{Line: 5, FirstColumn: 11, LastColumn: 15},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "label1",
									},
									Value: &parser.YamlNode{
										Value: "val1",
									},
								},
								{
									Key: &parser.YamlNode{
										Value: "label2",
									},
									Value: &parser.YamlNode{
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
					Lines: diags.LineRange{First: 9, Last: 11},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "name2",
							Pos: diags.PositionRanges{
								{Line: 9, FirstColumn: 13, LastColumn: 17},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "expr2",
								Pos: diags.PositionRanges{
									{Line: 10, FirstColumn: 11, LastColumn: 15},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "label1",
									},
									Value: &parser.YamlNode{
										Value: "val1",
									},
								},
								{
									Key: &parser.YamlNode{
										Value: "label2",
									},
									Value: &parser.YamlNode{
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
					Lines: diags.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("alert value cannot be empty"), Line: 1},
				},
			},
		},
		{
			content: []byte("- alert: foo\n  expr:\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("expr value cannot be empty"), Line: 2},
				},
			},
		},
		{
			content: []byte("- alert: foo\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 1},
				},
			},
		},
		{
			content: []byte("- record:\n  expr:\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("record value cannot be empty"), Line: 1},
				},
			},
		},
		{
			content: []byte("- record: foo\n  expr:\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: errors.New("expr value cannot be empty"), Line: 2},
				},
			},
		},
		{
			content: []byte("- record: foo\n"),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 1, Last: 1},
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
					Lines: diags.LineRange{First: 7, Last: 8},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 7, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "up",
								Pos: diags.PositionRanges{
									{Line: 8, FirstColumn: 9, LastColumn: 10},
								},
							},
						},
					},
				},
				{
					Lines: diags.LineRange{First: 12, Last: 13},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 12, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "up",
								Pos: diags.PositionRanges{
									{Line: 13, FirstColumn: 9, LastColumn: 10},
								},
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
					Lines: diags.LineRange{First: 2, Last: 5},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "Template",
							Pos: diags.PositionRanges{
								{Line: 2, FirstColumn: 10, LastColumn: 17},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "up == 0",
								Pos: diags.PositionRanges{
									{Line: 3, FirstColumn: 15, LastColumn: 21},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "notify",
									},
									Value: &parser.YamlNode{
										Value: "chat-alerts",
									},
								},
							},
						},
					},
				},
				{
					Lines: diags.LineRange{First: 6, Last: 10},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "Service Down",
							Pos: diags.PositionRanges{
								{Line: 6, FirstColumn: 10, LastColumn: 21},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "up == 0",
								Pos: diags.PositionRanges{
									{Line: 7, FirstColumn: 10, LastColumn: 13}, // points at anchor
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "notify",
									},
									Value: &parser.YamlNode{
										Value: "chat-alerts",
									},
								},
								{
									Key: &parser.YamlNode{
										Value: "summary",
									},
									Value: &parser.YamlNode{
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
					Lines: diags.LineRange{First: 2, Last: 3},
					Error: parser.ParseError{Err: errors.New("invalid recording rule name: invalid metric name"), Line: 2},
				},
			},
		},
		{
			content: []byte(`
- record: utf-8 enabled name
  expr: bar
  labels:
    "a b c": bar
`),
			names: model.UTF8Validation,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 2, Last: 5},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "utf-8 enabled name",
							Pos: diags.PositionRanges{
								{Line: 2, FirstColumn: 11, LastColumn: 28},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "bar",
								Pos: diags.PositionRanges{
									{Line: 3, FirstColumn: 9, LastColumn: 11},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "a b c",
									},
									Value: &parser.YamlNode{
										Value: "bar",
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
- record: foo
  expr: bar
  labels:
    "foo bar": yes
`),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 3},
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
					Lines: diags.LineRange{First: 2, Last: 3},
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
					Lines: diags.LineRange{First: 2, Last: 3},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 7},
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
					Lines: diags.LineRange{First: 2, Last: 7},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 2, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "bar",
								Pos: diags.PositionRanges{
									{Line: 3, FirstColumn: 9, LastColumn: 11},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
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
					Lines: diags.LineRange{First: 2, Last: 3},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 4},
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
					Lines: diags.LineRange{First: 2, Last: 3},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 3, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "foo",
								Pos: diags.PositionRanges{
									{Line: 2, FirstColumn: 9, LastColumn: 11},
								},
							},
						},
					},
				},
				{
					Lines: diags.LineRange{First: 5, Last: 5},
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
					Lines: diags.LineRange{First: 2, Last: 3},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 3, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "foo",
								Pos: diags.PositionRanges{
									{Line: 2, FirstColumn: 9, LastColumn: 11},
								},
							},
						},
					},
				},
				{
					Lines: diags.LineRange{First: 5, Last: 7},
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
					Lines: diags.LineRange{First: 2, Last: 3},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 3, FirstColumn: 11, LastColumn: 13},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "foo",
								Pos: diags.PositionRanges{
									{Line: 2, FirstColumn: 9, LastColumn: 11},
								},
							},
						},
					},
				},
				{
					Lines: diags.LineRange{First: 5, Last: 6},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 6, FirstColumn: 10, LastColumn: 12},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "bar",
								Pos: diags.PositionRanges{
									{Line: 5, FirstColumn: 9, LastColumn: 11},
								},
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
					Lines: diags.LineRange{First: 5, Last: 9},
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
					Lines: diags.LineRange{First: 5, Last: 6},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "up:count",
							Pos: diags.PositionRanges{
								{Line: 5, FirstColumn: 13, LastColumn: 20},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "count(up)",
								Pos: diags.PositionRanges{
									{Line: 6, FirstColumn: 11, LastColumn: 19},
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
- name: v1
  rules:
  - record: up:count
`),
			strict: true,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 5, Last: 5},
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
					Lines: diags.LineRange{First: 6, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "up:count",
							Pos: diags.PositionRanges{
								{Line: 6, FirstColumn: 15, LastColumn: 22},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "count(up)",
								Pos: diags.PositionRanges{
									{Line: 7, FirstColumn: 13, LastColumn: 21},
								},
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
					Lines: diags.LineRange{First: 8, Last: 8},
					Error: parser.ParseError{
						Line: 8,
						Err:  errors.New("labels value must be a mapping, got binary data instead"),
					},
				},
				{
					Lines: diags.LineRange{First: 4, Last: 4},
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
					Lines: diags.LineRange{First: 5, Last: 8},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 5, FirstColumn: 13, LastColumn: 15},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "sum(up)",
								Pos: diags.PositionRanges{
									{Line: 6, FirstColumn: 11, LastColumn: 17},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "job",
									},
									Value: &parser.YamlNode{
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
					Lines: diags.LineRange{First: 5, Last: 9},
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
					Lines: diags.LineRange{First: 5, Last: 7},
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
					Lines: diags.LineRange{First: 8, Last: 9},
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
					Lines: diags.LineRange{First: 5, Last: 7},
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
					Lines: diags.LineRange{First: 5, Last: 5},
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
					Lines: diags.LineRange{First: 5, Last: 5},
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
					Lines: diags.LineRange{First: 5, Last: 5},
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
					Lines: diags.LineRange{First: 5, Last: 6},
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
					Lines: diags.LineRange{First: 5, Last: 5},
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
					Lines: diags.LineRange{First: 6, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 6, FirstColumn: 15, LastColumn: 17},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "bar",
								Pos: diags.PositionRanges{
									{Line: 7, FirstColumn: 13, LastColumn: 15},
								},
							},
						},
					},
				},
				{
					Lines: diags.LineRange{First: 12, Last: 13},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 12, FirstColumn: 15, LastColumn: 17},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "bar",
								Pos: diags.PositionRanges{
									{Line: 13, FirstColumn: 13, LastColumn: 15},
								},
							},
						},
					},
				},
				{
					Lines: diags.LineRange{First: 8, Last: 8},
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
					Lines: diags.LineRange{First: 5, Last: 7},
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
					Lines: diags.LineRange{First: 5, Last: 7},
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
					Lines: diags.LineRange{First: 5, Last: 7},
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
					Lines: diags.LineRange{First: 5, Last: 10},
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
					Lines: diags.LineRange{First: 6, Last: 10},
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
					Lines: diags.LineRange{First: 5, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 5, FirstColumn: 13, LastColumn: 15},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "{\"up\"}\n",
								Pos: diags.PositionRanges{
									{Line: 7, FirstColumn: 7, LastColumn: 12},
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
    expr: |
      {'up'}
`),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 5, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 5, FirstColumn: 13, LastColumn: 15},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "{'up'}\n",
								Pos: diags.PositionRanges{
									{Line: 7, FirstColumn: 7, LastColumn: 12},
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
    expr: |
      {'up' == 1}
`),
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 5, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "foo",
							Pos: diags.PositionRanges{
								{Line: 5, FirstColumn: 13, LastColumn: 15},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "{'up' == 1}\n",
								Pos: diags.PositionRanges{
									{Line: 7, FirstColumn: 7, LastColumn: 17},
								},
							},
							SyntaxError: errors.New(`1:8: parse error: unexpected "=" in label matching, expected string`),
						},
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: mygroup
  partial_response_strategy: bob
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			err:    "error at line 4: partial_response_strategy is only valid when parser is configured to use the Thanos rule schema",
		},
		{
			content: []byte(`
groups:
- name: mygroup
  partial_response_strategy: warn
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			schema: parser.ThanosSchema,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 6, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "up:count",
							Pos:   diags.PositionRanges{{Line: 6, FirstColumn: 13, LastColumn: 20}},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "count(up)",
								Pos:   diags.PositionRanges{{Line: 7, FirstColumn: 11, LastColumn: 19}},
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: mygroup
  partial_response_strategy: abort
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			schema: parser.ThanosSchema,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 6, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "up:count",
							Pos:   diags.PositionRanges{{Line: 6, FirstColumn: 13, LastColumn: 20}},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "count(up)",
								Pos:   diags.PositionRanges{{Line: 7, FirstColumn: 11, LastColumn: 19}},
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: mygroup
  partial_response_strategy: abort
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: false,
			schema: parser.PrometheusSchema,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 6, Last: 7},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "up:count",
							Pos:   diags.PositionRanges{{Line: 6, FirstColumn: 13, LastColumn: 20}},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "count(up)",
								Pos:   diags.PositionRanges{{Line: 7, FirstColumn: 11, LastColumn: 19}},
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
groups:
- name: mygroup
  partial_response_strategy: bob
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			schema: parser.ThanosSchema,
			err:    "error at line 4: invalid partial_response_strategy value: bob",
		},
		{
			content: []byte(`
groups:
- name: mygroup
  partial_response_strategy: 1
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			schema: parser.ThanosSchema,
			err:    "error at line 4: partial_response_strategy must be a string, got integer",
		},
		{
			content: []byte(`
- alert: Multi Line
  expr: foo
          AND ON (instance)
          bar
`),
			strict: false,
			schema: parser.PrometheusSchema,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 2, Last: 5},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "Multi Line",
							Pos:   diags.PositionRanges{{Line: 2, FirstColumn: 10, LastColumn: 19}},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "foo AND ON (instance) bar",
								Pos: diags.PositionRanges{
									{Line: 3, FirstColumn: 9, LastColumn: 12},
									{Line: 4, FirstColumn: 11, LastColumn: 28},
									{Line: 5, FirstColumn: 11, LastColumn: 13},
								},
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
  - alert: FooBar
    expr: >-
      count(
        foo
        or
        bar
      ) > 0
`),
			strict: false,
			schema: parser.PrometheusSchema,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 2, Last: 8},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "FooBar",
							Pos:   diags.PositionRanges{{Line: 2, FirstColumn: 12, LastColumn: 17}},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "count(\n  foo\n  or\n  bar\n) > 0",
								Pos: diags.PositionRanges{
									{Line: 4, FirstColumn: 7, LastColumn: 13},
									{Line: 5, FirstColumn: 7, LastColumn: 12},
									{Line: 6, FirstColumn: 7, LastColumn: 11},
									{Line: 7, FirstColumn: 7, LastColumn: 12},
									{Line: 8, FirstColumn: 7, LastColumn: 11},
								},
							},
						},
					},
				},
			},
		},
		{
			content: []byte(`
  - alert: FooBar
    expr: >-
      aaaaaaaaaaaaaaaaaaaaaaaa
      AND ON (colo_id) bbbbbbbbbbb
      > 2
    for: 1m
`),
			strict: false,
			schema: parser.PrometheusSchema,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 2, Last: 7},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "FooBar",
							Pos:   diags.PositionRanges{{Line: 2, FirstColumn: 12, LastColumn: 17}},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "aaaaaaaaaaaaaaaaaaaaaaaa AND ON (colo_id) bbbbbbbbbbb > 2",
								Pos: diags.PositionRanges{
									{Line: 4, FirstColumn: 7, LastColumn: 31},
									{Line: 5, FirstColumn: 7, LastColumn: 35},
									{Line: 6, FirstColumn: 7, LastColumn: 9},
								},
							},
						},
						For: &parser.YamlNode{
							Value: "1m",
							Pos:   diags.PositionRanges{{Line: 7, FirstColumn: 10, LastColumn: 11}},
						},
					},
				},
			},
		},
		{
			content: []byte(`
  - alert: FooBar
    expr: 'aaaaaaaaaaaaaaaaaaaaaaaa
          AND ON (colo_id) bbbbbbbbbbb
          > 2'
    for: 1m
`),
			strict: false,
			schema: parser.PrometheusSchema,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 2, Last: 6},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "FooBar",
							Pos:   diags.PositionRanges{{Line: 2, FirstColumn: 12, LastColumn: 17}},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "aaaaaaaaaaaaaaaaaaaaaaaa AND ON (colo_id) bbbbbbbbbbb > 2",
								Pos: diags.PositionRanges{
									{Line: 3, FirstColumn: 12, LastColumn: 36},
									{Line: 4, FirstColumn: 11, LastColumn: 39},
									{Line: 5, FirstColumn: 11, LastColumn: 13},
								},
							},
						},
						For: &parser.YamlNode{
							Value: "1m",
							Pos:   diags.PositionRanges{{Line: 6, FirstColumn: 10, LastColumn: 11}},
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
  - record: colo:foo:sum
    expr: sum without (instance) ( rate(my_metric[2m]) * on (instance)
      group_left (hardware_generation, hms_scope, sliver) (instance:metadata{})
      )
`),
			strict: false,
			schema: parser.PrometheusSchema,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 5, Last: 8},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "colo:foo:sum",
							Pos:   diags.PositionRanges{{Line: 5, FirstColumn: 13, LastColumn: 24}},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: "sum without (instance) ( rate(my_metric[2m]) * on (instance) group_left (hardware_generation, hms_scope, sliver) (instance:metadata{}) )",
								Pos: diags.PositionRanges{
									{Line: 6, FirstColumn: 11, LastColumn: 71},
									{Line: 7, FirstColumn: 7, LastColumn: 80},
									{Line: 8, FirstColumn: 7, LastColumn: 7},
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
- name: "{{ source }}"
  rules:
  # AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
  # BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
  - record: some_long_name
    labels:
      metricssource: receiver
    expr:
      clamp_max(
        sum(rate(my_metric_long_name_total{output="control:output:kafka:/:requests:http-b:http_requests_control_sample"}[5m])) /
        sum(rate(my_metric_long_name_total{output="control:output:kafka:/:requests:http-b:http_requests_control_sample"}[5m] offset 30m)),
        2
      )
`),
			strict: true,
			schema: parser.PrometheusSchema,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 7, Last: 15},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Value: "some_long_name",
							Pos:   diags.PositionRanges{{Line: 7, FirstColumn: 13, LastColumn: 26}},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Value: "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Value: "metricssource",
									},
									Value: &parser.YamlNode{
										Value: "receiver",
									},
								},
							},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: `clamp_max( sum(rate(my_metric_long_name_total{output="control:output:kafka:/:requests:http-b:http_requests_control_sample"}[5m])) / sum(rate(my_metric_long_name_total{output="control:output:kafka:/:requests:http-b:http_requests_control_sample"}[5m] offset 30m)), 2 )`,
								Pos: diags.PositionRanges{
									{Line: 11, FirstColumn: 7, LastColumn: 17},
									{Line: 12, FirstColumn: 9, LastColumn: 129},
									{Line: 13, FirstColumn: 9, LastColumn: 139},
									{Line: 14, FirstColumn: 9, LastColumn: 10},
									{Line: 15, FirstColumn: 7, LastColumn: 7},
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
- name: "{{ source }}"
  rules: 

  - alert: Director_Is_Not_Advertising_Any_Routes
    expr: |
        sum without (name) (
            bird_protocol_prefix_export_count{ip_version="4",name=~".*external.*",proto!="Kernel"}
          * on (instance) group_left (profile,cluster)
            cf_node_role{kubernetes_role="director",role="kubernetes"}
        ) <= 0
    for: 1m
`),
			strict: true,
			schema: parser.PrometheusSchema,
			output: []parser.Rule{
				{
					Lines: diags.LineRange{First: 6, Last: 13},
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{
							Value: "Director_Is_Not_Advertising_Any_Routes",
							Pos:   diags.PositionRanges{{Line: 6, FirstColumn: 12, LastColumn: 49}},
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Value: `sum without (name) (
    bird_protocol_prefix_export_count{ip_version="4",name=~".*external.*",proto!="Kernel"}
  * on (instance) group_left (profile,cluster)
    cf_node_role{kubernetes_role="director",role="kubernetes"}
) <= 0
`,
								Pos: diags.PositionRanges{
									{Line: 8, FirstColumn: 9, LastColumn: 29},
									{Line: 9, FirstColumn: 9, LastColumn: 99},
									{Line: 10, FirstColumn: 9, LastColumn: 55},
									{Line: 11, FirstColumn: 9, LastColumn: 71},
									{Line: 12, FirstColumn: 9, LastColumn: 14},
								},
							},
						},
						For: &parser.YamlNode{
							Value: "1m",
							Pos:   diags.PositionRanges{{Line: 13, FirstColumn: 10, LastColumn: 11}},
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

			p := parser.NewParser(tc.strict, tc.schema, tc.names)
			output, err := p.Parse(tc.content)

			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}

			if diff := cmp.Diff(tc.output, output,
				ignorePrometheusExpr,
				sameErrorText,
				cmpopts.IgnoreFields(parser.YamlNode{}, "Pos"), // FIXME remove?
			); diff != "" {
				t.Errorf("Parse() returned wrong output (-want +got):\n%s", diff)
				return
			}
		})
	}
}
