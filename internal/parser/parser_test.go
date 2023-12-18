package parser_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/parser"

	"github.com/google/go-cmp/cmp"
	promparser "github.com/prometheus/prometheus/promql/parser"
)

func TestParse(t *testing.T) {
	type testCaseT struct {
		content     []byte
		output      []parser.Rule
		shouldError bool
	}

	testCases := []testCaseT{
		{
			content:     nil,
			output:      nil,
			shouldError: false,
		},
		{
			content:     []byte{},
			output:      nil,
			shouldError: false,
		},
		{
			content:     []byte(string("! !00 \xf6")),
			output:      nil,
			shouldError: true,
		},
		{
			content: []byte("- 0: 0\n  00000000: 000000\n  000000:00000000000: 00000000\n  00000000000:000000: 0000000000000000000000000000000000\n  000000: 0000000\n  expr: |"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 6},
					Error: parser.ParseError{Err: fmt.Errorf("incomplete rule, no alert or record key"), Line: 6},
				},
			},
		},
		{
			content: []byte("- record: |\n    multiline\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: fmt.Errorf("missing expr key"), Line: 2},
				},
			},
		},
		{
			content: []byte("- expr: foo\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: fmt.Errorf("incomplete rule, no alert or record key"), Line: 1},
				},
			},
		},
		{
			content: []byte("- alert: foo\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: fmt.Errorf("missing expr key"), Line: 1},
				},
			},
		},
		{
			content: []byte("- alert: foo\n  record: foo\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: fmt.Errorf("got both record and alert keys in a single rule"), Line: 1},
				},
			},
		},
		{
			content: []byte("- record: foo\n  labels:\n    foo: bar\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 3},
					Error: parser.ParseError{Err: fmt.Errorf("missing expr key"), Line: 1},
				},
			},
		},
		{
			content:     []byte("- record: - foo\n"),
			shouldError: true,
		},
		{
			content:     []byte("- record: foo  expr: sum(\n"),
			shouldError: true,
		},
		{
			content:     []byte("- record\n\texpr: foo\n"),
			shouldError: true,
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
					Error: parser.ParseError{Err: fmt.Errorf("duplicated expr key"), Line: 4},
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
					Error: parser.ParseError{Err: fmt.Errorf("duplicated record key"), Line: 4},
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
					Error: parser.ParseError{Err: fmt.Errorf("duplicated alert key"), Line: 3},
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
					Error: parser.ParseError{Err: fmt.Errorf("duplicated for key"), Line: 5},
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
					Error: parser.ParseError{Err: fmt.Errorf("duplicated keep_firing_for key"), Line: 5},
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
					Error: parser.ParseError{Err: fmt.Errorf("duplicated labels key"), Line: 5},
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
					Error: parser.ParseError{Err: fmt.Errorf("duplicated labels key"), Line: 5},
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
					Error: parser.ParseError{Err: fmt.Errorf("duplicated annotations key"), Line: 5},
				},
			},
		},
		{
			content: []byte("- record: foo\n  expr: foo\n  extra: true\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 3},
					Error: parser.ParseError{Err: fmt.Errorf("invalid key(s) found: extra"), Line: 3},
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
							Query: &parser.PromQLNode{
								Expr: "foo offset 10m",
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
							Query: &parser.PromQLNode{
								Expr: "foo offset -10m",
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
							Query: &parser.PromQLNode{
								Expr: "foo offset 10m",
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
							Query: &parser.PromQLNode{
								Expr: "foo[5m] offset 10m",
								Children: []*parser.PromQLNode{
									{Expr: "foo offset 10m"},
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
							Query: &parser.PromQLNode{
								Expr: "sum(foo)",
								Children: []*parser.PromQLNode{
									{Expr: "foo"},
								},
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
			shouldError: false,
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
							Query: &parser.PromQLNode{
								Expr: "sum(foo)",
								Children: []*parser.PromQLNode{
									{Expr: "foo"},
								},
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
			shouldError: false,
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

- record: >
    foo
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
							Query: &parser.PromQLNode{
								Expr: "up == 0\n",
								Children: []*parser.PromQLNode{
									{Expr: "up"},
									{Expr: "0"},
								},
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
					Lines: parser.LineRange{First: 11, Last: 17},
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{
							Lines: parser.LineRange{First: 11, Last: 12},
							Value: "foo\n",
						},
						Expr: parser.PromQLExpr{
							Value: &parser.YamlNode{
								Lines: parser.LineRange{First: 13, Last: 16},
								Value: "bar\n/\nbaz > 1",
							},
							Query: &parser.PromQLNode{
								Expr: "bar\n/\nbaz > 1",
								Children: []*parser.PromQLNode{
									{
										Expr: "bar / baz", Children: []*parser.PromQLNode{
											{Expr: "bar"},
											{Expr: "baz"},
										},
									},
									{Expr: "1"},
								},
							},
						},
						Labels: &parser.YamlMap{
							Lines: parser.LineRange{First: 17, Last: 17},
							Key: &parser.YamlNode{
								Lines: parser.LineRange{First: 17, Last: 17},
								Value: "labels",
							},
						},
					},
				},
			},
			shouldError: false,
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
							Query: &parser.PromQLNode{
								Expr: "( xxx - yyy ) * bar > 0 and on(instance, device) baz",
								Children: []*parser.PromQLNode{
									{
										Expr: "(xxx - yyy) * bar > 0",
										Children: []*parser.PromQLNode{
											{
												Expr: "(xxx - yyy) * bar",
												Children: []*parser.PromQLNode{
													{
														Expr: "(xxx - yyy)",
														Children: []*parser.PromQLNode{
															{
																Expr: "xxx - yyy",
																Children: []*parser.PromQLNode{
																	{Expr: "xxx"},
																	{Expr: "yyy"},
																},
															},
														},
													},
													{
														Expr: "bar",
													},
												},
											},
											{
												Expr: "0",
											},
										},
									},
									{Expr: "baz"},
								},
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
							Query: &parser.PromQLNode{
								Expr: `kube_deployment_status_replicas_available{namespace="example-app"} < 1`,
								Children: []*parser.PromQLNode{
									{Expr: `kube_deployment_status_replicas_available{namespace="example-app"}`},
									{Expr: "1"},
								},
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
							Query: &parser.PromQLNode{
								Expr: `sum(rate(kube_pod_container_status_restarts_total{namespace="example-app"}[5m])) > ( 3/60 )`,
								Children: []*parser.PromQLNode{
									{
										Expr: `sum(rate(kube_pod_container_status_restarts_total{namespace="example-app"}[5m]))`,
										Children: []*parser.PromQLNode{
											{
												Expr: `rate(kube_pod_container_status_restarts_total{namespace="example-app"}[5m])`,
												Children: []*parser.PromQLNode{
													{
														Expr: `kube_pod_container_status_restarts_total{namespace="example-app"}[5m]`,
														Children: []*parser.PromQLNode{
															{Expr: `kube_pod_container_status_restarts_total{namespace="example-app"}`},
														},
													},
												},
											},
										},
									},
									{
										Expr: "(3 / 60)",
										Children: []*parser.PromQLNode{
											{
												Expr: "3 / 60",
												Children: []*parser.PromQLNode{
													{Expr: "3"},
													{Expr: "60"},
												},
											},
										},
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
							Query: &parser.PromQLNode{
								Expr: "increase(haproxy_server_check_failures_total[15m]) > 100",
								Children: []*parser.PromQLNode{
									{
										Expr: "increase(haproxy_server_check_failures_total[15m])",
										Children: []*parser.PromQLNode{
											{
												Expr: "haproxy_server_check_failures_total[15m]",
												Children: []*parser.PromQLNode{
													{
														Expr: "haproxy_server_check_failures_total",
													},
												},
											},
										},
									},
									{Expr: "100"},
								},
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
							Query: &parser.PromQLNode{Expr: "expr1"},
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
							Query: &parser.PromQLNode{Expr: "expr2"},
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
							Query: &parser.PromQLNode{Expr: "expr1"},
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
							Query: &parser.PromQLNode{Expr: "expr1"},
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
							Query: &parser.PromQLNode{Expr: "expr2"},
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
					Error: parser.ParseError{Err: fmt.Errorf("alert value cannot be empty"), Line: 1},
				},
			},
		},
		{
			content: []byte("- alert: foo\n  expr:\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: fmt.Errorf("expr value cannot be empty"), Line: 2},
				},
			},
		},
		{
			content: []byte("- alert: foo\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: fmt.Errorf("missing expr key"), Line: 1},
				},
			},
		},
		{
			content: []byte("- record:\n  expr:\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: fmt.Errorf("record value cannot be empty"), Line: 1},
				},
			},
		},
		{
			content: []byte("- record: foo\n  expr:\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 2},
					Error: parser.ParseError{Err: fmt.Errorf("expr value cannot be empty"), Line: 2},
				},
			},
		},
		{
			content: []byte("- record: foo\n"),
			output: []parser.Rule{
				{
					Lines: parser.LineRange{First: 1, Last: 1},
					Error: parser.ParseError{Err: fmt.Errorf("missing expr key"), Line: 1},
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
							Query: &parser.PromQLNode{Expr: "up"},
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
							Query: &parser.PromQLNode{Expr: "up"},
						},
					},
				},
			},
			shouldError: false,
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
							Query: &parser.PromQLNode{
								Expr: "up == 0",
								Children: []*parser.PromQLNode{
									{Expr: "up"},
									{Expr: "0"},
								},
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
							Query: &parser.PromQLNode{
								Expr: "up == 0",
								Children: []*parser.PromQLNode{
									{Expr: "up"},
									{Expr: "0"},
								},
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
	}

	alwaysEqual := cmp.Comparer(func(_, _ interface{}) bool { return true })
	ignorePrometheusExpr := cmp.FilterValues(func(x, y interface{}) bool {
		_, xe := x.(promparser.Expr)
		_, ye := y.(promparser.Expr)
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
			p := parser.NewParser()
			output, err := p.Parse(tc.content)

			hadError := err != nil
			if hadError != tc.shouldError {
				t.Errorf("Parse() returned err=%v, expected=%v", err, tc.shouldError)
				return
			}

			if diff := cmp.Diff(tc.output, output, ignorePrometheusExpr, sameErrorText); diff != "" {
				t.Errorf("Parse() returned wrong output (-want +got):\n%s", diff)
				return
			}
		})
	}
}
