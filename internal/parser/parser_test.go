package parser_test

import (
	"fmt"
	"strconv"
	"testing"

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
			content: []byte("- record: |\n    multiline\n"),
			output: []parser.Rule{
				{Error: parser.ParseError{Err: fmt.Errorf("missing expr key"), Line: 1}},
			},
		},
		{
			content: []byte("- expr: foo\n"),
			output: []parser.Rule{
				{Error: parser.ParseError{Err: fmt.Errorf("incomplete rule, no alert or record key"), Line: 1}},
			},
		},
		{
			content: []byte("- alert: foo\n"),
			output: []parser.Rule{
				{Error: parser.ParseError{Err: fmt.Errorf("missing expr key"), Line: 1}},
			},
		},
		{
			content: []byte("- alert: foo\n  record: foo\n"),
			output: []parser.Rule{
				{Error: parser.ParseError{Err: fmt.Errorf("got both record and alert keys in a single rule"), Line: 1}},
			},
		},
		{
			content: []byte("- record: foo\n  labels:\n    foo: bar\n"),
			output: []parser.Rule{
				{Error: parser.ParseError{Err: fmt.Errorf("missing expr key"), Line: 1}},
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
			output:      nil,
			shouldError: true,
		},
		{
			content: []byte("- record: foo\n  expr: foo\n  extra: true\n"),
			output: []parser.Rule{
				{Error: parser.ParseError{Err: fmt.Errorf("invalid key(s) found: extra"), Line: 3}},
			},
		},
		{
			content: []byte("- record: foo\n  expr: foo offset 10m\n"),
			output: []parser.Rule{
				{
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{1}},
								Value:    "record",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{1}},
								Value:    "foo",
							},
						},
						Expr: parser.PromQLExpr{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{2}},
								Value:    "expr",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{2}},
								Value:    "foo offset 10m",
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
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{1}},
								Value:    "record",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{1}},
								Value:    "foo",
							},
						},
						Expr: parser.PromQLExpr{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{2}},
								Value:    "expr",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{2}},
								Value:    "foo offset -10m",
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
# head comment
- record: foo # record comment
  expr: foo offset 10m # expr comment
  #  pre-labels comment
  labels:
    # pre-foo comment
    foo: bar
    # post-foo comment
    bob: alice
# foot comment
`),
			output: []parser.Rule{
				{
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{3}},
								Value:    "record",
								Comments: []string{"# head comment"},
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{3}},
								Value:    "foo",
								Comments: []string{"# record comment"},
							},
						},
						Expr: parser.PromQLExpr{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{4}},
								Value:    "expr",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{4}},
								Value:    "foo offset 10m",
								Comments: []string{"# expr comment"},
							},
							Query: &parser.PromQLNode{
								Expr: "foo offset 10m",
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{6}},
								Value:    "labels",
								Comments: []string{"#  pre-labels comment"},
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{8}},
										Value:    "foo",
										Comments: []string{"# pre-foo comment"},
									},
									Value: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{8}},
										Value:    "bar",
									},
								},
								{
									Key: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{10}},
										Value:    "bob",
										Comments: []string{"# post-foo comment"},
									},
									Value: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{10}},
										Value:    "alice",
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
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{1}},
								Value:    "record",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{1}},
								Value:    "foo",
							},
						},
						Expr: parser.PromQLExpr{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{2}},
								Value:    "expr",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{2}},
								Value:    "foo[5m] offset 10m",
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
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{2}},
								Value:    "record",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{2}},
								Value:    "name",
							},
						},
						Expr: parser.PromQLExpr{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{3}},
								Value:    "expr",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{3}},
								Value:    "sum(foo)",
							},
							Query: &parser.PromQLNode{
								Expr: "sum(foo)",
								Children: []*parser.PromQLNode{
									{Expr: "foo"},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{4}},
								Value:    "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{5}},
										Value:    "foo",
									},
									Value: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{5}},
										Value:    "bar",
									},
								},
								{
									Key: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{6}},
										Value:    "bob",
									},
									Value: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{6}},
										Value:    "alice",
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
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{5}},
								Value:    "record",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{5}},
								Value:    "name",
							},
						},
						Expr: parser.PromQLExpr{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{6}},
								Value:    "expr",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{6}},
								Value:    "sum(foo)",
							},
							Query: &parser.PromQLNode{
								Expr: "sum(foo)",
								Children: []*parser.PromQLNode{
									{Expr: "foo"},
								},
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{7}},
								Value:    "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{8}},
										Value:    "foo",
									},
									Value: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{8}},
										Value:    "bar",
									},
								},
								{
									Key: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{9}},
										Value:    "bob",
									},
									Value: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{9}},
										Value:    "alice",
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
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{1}},
								Value:    "alert",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{1}},
								Value:    "Down",
							},
						},
						Expr: parser.PromQLExpr{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{2}},
								Value:    "expr",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{3}},
								Value:    "up == 0\n",
							},
							Query: &parser.PromQLNode{
								Expr: "up == 0\n",
								Children: []*parser.PromQLNode{
									{Expr: "up"},
									{Expr: "0"},
								},
							},
						},
						For: &parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{4}},
								Value:    "for",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{5}},
								Value:    "11m\n",
							},
						},
						Labels: &parser.YamlMap{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{6}},
								Value:    "labels",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{7}},
										Value:    "severity",
									},
									Value: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{7}},
										Value:    "critical",
									},
								},
							},
						},
						Annotations: &parser.YamlMap{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{8}},
								Value:    "annotations",
							},
							Items: []*parser.YamlKeyValue{
								{
									Key: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{9}},
										Value:    "uri",
									},
									Value: &parser.YamlNode{
										Position: parser.FilePosition{Lines: []int{9}},
										Value:    "https://docs.example.com/down.html",
									},
								},
							},
						},
					},
				},
				{
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{11}},
								Value:    "record",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{12}},
								Value:    "foo\n",
							},
						},
						Expr: parser.PromQLExpr{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{13}},
								Value:    "expr",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{14, 15, 16}},
								Value:    "bar\n/\nbaz > 1",
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
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{17}},
								Value:    "labels",
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
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{1}},
								Value:    "alert",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{1}},
								Value:    "Foo",
							},
						},
						Expr: parser.PromQLExpr{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{2}},
								Value:    "expr",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{3, 4, 5, 6, 7, 8}},
								Value:    "( xxx - yyy ) * bar > 0 and on(instance, device) baz",
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
						For: &parser.YamlKeyValue{
							Key: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{9}},
								Value:    "for",
							},
							Value: &parser.YamlNode{
								Position: parser.FilePosition{Lines: []int{9}},
								Value:    "30m",
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
			output: nil,
		},
		/*
					FIXME https://github.com/cloudflare/pint/issues/20
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
								AlertingRule: &parser.AlertingRule{
									Alert: parser.YamlKeyValue{
										Key: &parser.YamlNode{
											Position: parser.FilePosition{Lines: []int{4}},
											Value:    "alert",
										},
										Value: &parser.YamlNode{
											Position: parser.FilePosition{Lines: []int{4}},
											Value:    "HaproxyServerHealthcheckFailure",
										},
									},
									Expr: parser.PromQLExpr{
										Key: &parser.YamlNode{
											Position: parser.FilePosition{Lines: []int{5}},
											Value:    "expr",
										},
										Value: &parser.YamlNode{
											Position: parser.FilePosition{Lines: []int{5}},
											Value:    "increase(haproxy_server_check_failures_total[15m]) > 100",
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
									For: &parser.YamlKeyValue{
										Key: &parser.YamlNode{
											Position: parser.FilePosition{Lines: []int{6}},
											Value:    "for",
										},
										Value: &parser.YamlNode{Position: parser.FilePosition{Lines: []int{6}},
											Value: "5m",
										},
									},
									Labels: &parser.YamlMap{
										Key: &parser.YamlNode{
											Position: parser.FilePosition{Lines: []int{7}},
											Value:    "labels",
										},
										Items: []*parser.YamlKeyValue{
											{
												Key: &parser.YamlNode{
													Position: parser.FilePosition{Lines: []int{8}},
													Value:    "severity",
												},
												Value: &parser.YamlNode{
													Position: parser.FilePosition{Lines: []int{8}},
													Value:    "24x7",
												},
											},
										},
									},
									Annotations: &parser.YamlMap{
										Key: &parser.YamlNode{
											Position: parser.FilePosition{Lines: []int{9}},
											Value:    "annotations",
										},
										Items: []*parser.YamlKeyValue{
											{
												Key: &parser.YamlNode{
													Position: parser.FilePosition{Lines: []int{10}},
													Value:    "summary",
												},
												Value: &parser.YamlNode{
													Position: parser.FilePosition{Lines: []int{10}},
													Value:    "HAProxy server healthcheck failure (instance {{ $labels.instance }})",
												},
											},
											{
												Key: &parser.YamlNode{
													Position: parser.FilePosition{Lines: []int{11}},
													Value:    "description",
												},
												Value: &parser.YamlNode{
													Position: parser.FilePosition{Lines: []int{11}},
													Value:    `Some server healthcheck are failing on {{ $labels.server }}\n  VALUE = {{ $value }}\n  LABELS: {{ $labels }}`,
												},
											},
										},
									},
								},
							},
						},
					},
		*/
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
