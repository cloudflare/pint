package parser_test

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

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
		input  []byte
		output parser.File
		strict bool
		schema parser.Schema
		names  model.ValidationScheme
	}

	testCases := []testCaseT{
		{
			input: nil,
			output: parser.File{
				IsRelaxed: true,
			},
		},
		{
			input: []byte{},
			output: parser.File{
				IsRelaxed: true,
			},
		},
		{
			input: []byte(""),
			output: parser.File{
				IsRelaxed: true,
			},
		},
		{
			input: []byte("\n"),
			output: parser.File{
				IsRelaxed: true,
			},
		},
		{
			input: []byte("\n\n\n"),
			output: parser.File{
				IsRelaxed: true,
			},
		},
		{
			input: []byte("---"),
			output: parser.File{
				IsRelaxed: true,
			},
		},
		{
			input: []byte("---\n"),
			output: parser.File{
				IsRelaxed: true,
			},
		},
		{
			input: []byte("\n---\n\n---\n"),
			output: parser.File{
				IsRelaxed: true,
			},
		},
		{
			input: []byte("\n---\n\n---\n---"),
			output: parser.File{
				IsRelaxed: true,
			},
		},
		{
			input: []byte(string("! !00 \xf6")),
			output: parser.File{
				IsRelaxed: true,
				Error: parser.ParseError{
					Err:  errors.New("yaml: incomplete UTF-8 octet sequence"),
					Line: 1,
				},
			},
		},
		{
			input: []byte("- 0: 0\n  00000000: 000000\n  000000:00000000000: 00000000\n  00000000000:000000: 0000000000000000000000000000000000\n  000000: 0000000\n  expr: |"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 6},
								Error: parser.ParseError{Err: errors.New("incomplete rule, no alert or record key"), Line: 6},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- record: |\n    multiline\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 2},
								Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 2},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`---
- expr: foo
  record: foo
---
- expr: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
						},
					},
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 5},
								Error: parser.ParseError{Err: errors.New("incomplete rule, no alert or record key"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- expr: foo\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 1},
								Error: parser.ParseError{Err: errors.New("incomplete rule, no alert or record key"), Line: 1},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- alert: foo\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 1},
								Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 1},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- alert: foo\n  record: foo\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 2},
								Error: parser.ParseError{Err: errors.New("got both record and alert keys in a single rule"), Line: 1},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- record: foo\n  labels:\n    foo: bar\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 3},
								Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 1},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- record: - foo\n"),
			output: parser.File{
				IsRelaxed: true,
				Error: parser.ParseError{
					Err:  errors.New("yaml: block sequence entries are not allowed in this context"),
					Line: 1,
				},
			},
		},
		{
			input: []byte("- record: foo  expr: sum(\n"),
			output: parser.File{
				IsRelaxed: true,
				Error: parser.ParseError{
					Err:  errors.New("yaml: mapping values are not allowed in this context"),
					Line: 1,
				},
			},
		},
		{
			input: []byte("- record\n\texpr: foo\n"),
			output: parser.File{
				IsRelaxed: true,
				Error: parser.ParseError{
					Err:  errors.New("found a tab character that violates indentation"),
					Line: 2,
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  expr: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("duplicated expr key"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  record: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("duplicated record key"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  alert: bar
  expr: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 3},
								Error: parser.ParseError{Err: errors.New("duplicated alert key"), Line: 3},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  for: 5m
  expr: bar
  for: 1m
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("duplicated for key"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  keep_firing_for: 5m
  expr: bar
  keep_firing_for: 1m
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("duplicated keep_firing_for key"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  labels: {}
  expr: bar
  labels: {}
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("duplicated labels key"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  labels: {}
  expr: bar
  labels: {}
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("duplicated labels key"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  annotations: {}
  expr: bar
  annotations: {}
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("duplicated annotations key"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- record: foo\n  expr: foo\n  extra: true\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 3},
								Error: parser.ParseError{Err: errors.New("invalid key(s) found: extra"), Line: 3},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`- record: foo
  expr: foo offset 10m
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte("- record: foo\n  expr: foo offset -10m\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte("- record: foo\n  expr: foo[5m] offset 10m\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
- record: name
  expr: sum(foo)
  labels:
    foo: bar
    bob: alice
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
groups:
- name: custom_rules
  rules:
    - record: name
      expr: sum(foo)
      labels:
        foo: bar
        bob: alice
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "custom_rules",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`- alert: Down
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
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`- alert: Foo
  expr:
    (
      xxx
      -
      yyy
    ) * bar > 0
    and on(instance, device) baz
  for: 30m
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`---
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
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "example-app-alerts",
						Rules: []parser.Rule{
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
						},
					},
					{
						Name: "other alerts",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 28, Last: 29},
								AlertingRule: &parser.AlertingRule{
									Alert: parser.YamlNode{
										Value: "Example_High_Restart_Rate",
										Pos: diags.PositionRanges{
											{Line: 28, FirstColumn: 20, LastColumn: 44},
										},
									},
									Expr: parser.PromQLExpr{
										Value: &parser.YamlNode{
											Value: "1",
											Pos: diags.PositionRanges{
												{Line: 29, FirstColumn: 20, LastColumn: 20},
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
			input: []byte(`---
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
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "example-app-alerts",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`groups:
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
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "haproxy.api_server.rules",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`groups:
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
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "certmanager",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`groups:
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
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "certmanager",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte("- alert:\n  expr: vector(1)\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 2},
								Error: parser.ParseError{Err: errors.New("alert value cannot be empty"), Line: 1},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- alert: foo\n  expr:\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 2},
								Error: parser.ParseError{Err: errors.New("expr value cannot be empty"), Line: 2},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- alert: foo\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 1},
								Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 1},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- record:\n  expr:\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 2},
								Error: parser.ParseError{Err: errors.New("record value cannot be empty"), Line: 1},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- record: foo\n  expr:\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 2},
								Error: parser.ParseError{Err: errors.New("expr value cannot be empty"), Line: 2},
							},
						},
					},
				},
			},
		},
		{
			input: []byte("- record: foo\n"),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 1, Last: 1},
								Error: parser.ParseError{Err: errors.New("missing expr key"), Line: 1},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(string(`
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
			output: parser.File{
				IsRelaxed: true,
				Comments: []comments.Comment{
					{
						Type: comments.FileOwnerType,
						Value: comments.Owner{
							Name: "bob",
							Line: 2,
						},
					},
					{
						Type: comments.FileOwnerType,
						Value: comments.Owner{
							Name: "alice",
							Line: 10,
						},
					},
				},
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(string(`
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
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
- record: invalid metric name
  expr: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 3},
								Error: parser.ParseError{Err: errors.New("invalid recording rule name: invalid metric name"), Line: 2},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: utf-8 enabled name
  expr: bar
  labels:
    "a b c": bar
`),
			names: model.UTF8Validation,
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  labels:
    "foo bar": yes
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("invalid label name: foo bar"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  labels:
    "foo bar": yes
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("invalid label name: foo bar"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  labels:
    "{{ $value }}": yes
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("invalid label name: {{ $value }}"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  annotations:
    "foo bar": yes
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("invalid annotation name: foo bar"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  labels:
    foo: ` + string("\xed\xbf\xbf")),
			// Label values are invalid only if they aren't valid UTF-8 strings
			// which also makes them unparsable by YAML.
			output: parser.File{
				IsRelaxed: true,
				Error: parser.ParseError{
					Err:  errors.New("yaml: invalid Unicode character"),
					Line: 1,
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  annotations:
    "{{ $value }}": yes
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("invalid annotation name: {{ $value }}"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  keep_firing_for: 5m
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("invalid field 'keep_firing_for' in recording rule"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  for: 5m
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("invalid field 'for' in recording rule"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  annotations:
    foo: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("invalid field 'annotations' in recording rule"), Line: 4},
							},
						},
					},
				},
			},
		},
		// Tag tests
		{
			input: []byte(`
- record: 5
  expr: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 3},
								Error: parser.ParseError{Err: errors.New("record value must be a string, got integer instead"), Line: 2},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: 5
  expr: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 3},
								Error: parser.ParseError{Err: errors.New("alert value must be a string, got integer instead"), Line: 2},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: 5
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 3},
								Error: parser.ParseError{Err: errors.New("expr value must be a string, got integer instead"), Line: 3},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  for: 5
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("for value must be a string, got integer instead"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  keep_firing_for: 5
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("keep_firing_for value must be a string, got integer instead"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  labels: []
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("labels value must be a mapping, got list instead"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  labels: {}
  annotations: []
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 5},
								Error: parser.ParseError{Err: errors.New("annotations value must be a mapping, got list instead"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  labels:
    foo: 3
  annotations:
    bar: "5"
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 7},
								Error: parser.ParseError{Err: errors.New("labels foo value must be a string, got integer instead"), Line: 5},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  labels: {}
  annotations:
    foo: "3"
    bar: 5
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 7},
								Error: parser.ParseError{Err: errors.New("annotations bar value must be a string, got integer instead"), Line: 7},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  labels: 4
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("labels value must be a mapping, got integer instead"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  labels: true
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("labels value must be a mapping, got bool instead"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  labels: null
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
- record: true
  expr: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 3},
								Error: parser.ParseError{Err: errors.New("record value must be a string, got bool instead"), Line: 2},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record:
    query: foo
  expr: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("record value must be a string, got mapping instead"), Line: 3},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  labels: some
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("labels value must be a mapping, got string instead"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  labels: !!binary "SGVsbG8sIFdvcmxkIQ=="
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{
									Err:  errors.New("labels value must be a mapping, got binary data instead"),
									Line: 4,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: foo
  expr: bar
  for: 1.23
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{
									Err:  errors.New("for value must be a string, got float instead"),
									Line: 4,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  labels: !!garbage "SGVsbG8sIFdvcmxkIQ=="
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{Err: errors.New("labels value must be a mapping, got garbage instead"), Line: 4},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: foo
  expr: bar
  labels: !! "SGVsbG8sIFdvcmxkIQ=="
`),
			output: parser.File{
				IsRelaxed: true,
				Error: parser.ParseError{
					Err:  errors.New("did not find expected tag URI"),
					Line: 4,
				},
			},
		},
		{
			input: []byte(`
- record: &foo foo
  expr: bar
  labels: *foo
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 2, Last: 4},
								Error: parser.ParseError{
									Err:  errors.New("labels value must be a mapping, got string instead"),
									Line: 4,
								},
							},
						},
					},
				},
			},
		},
		// Multi-document tests
		{
			input: []byte(`---
- expr: foo
  record: foo
---
- expr: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
						},
					},
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 5},
								Error: parser.ParseError{
									Err:  errors.New("incomplete rule, no alert or record key"),
									Line: 5,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`---
- expr: foo
  record: foo
---
- expr: bar
  record: bar
  expr: bar
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
						},
					},
					{
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 7},
								Error: parser.ParseError{Err: errors.New("duplicated expr key"), Line: 7},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`---
- expr: foo
  record: foo
---
- expr: bar
  alert: foo
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
						},
					},
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`---
groups:
- name: v1
  rules:
  - record: up:count
    expr: count(up)
    labels:
      foo:
        bar: foo
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "v1",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 9},
								Error: parser.ParseError{
									Err:  errors.New("labels foo value must be a string, got mapping instead"),
									Line: 9,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: v1
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "v1",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
groups:
- name: v1
  rules:
  - record: up:count
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "v1",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 5},
								Error: parser.ParseError{
									Err:  errors.New("missing expr key"),
									Line: 5,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- record: up:count
  expr: count(up)
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("top level field must be a groups key, got list"),
					Line: 2,
				},
			},
		},
		{
			input: []byte(`
rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("unexpected key rules"),
					Line: 2,
				},
			},
		},
		{
			input: []byte(`
groups:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Error: parser.ParseError{
							Err:  errors.New("invalid group key record"),
							Line: 3,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Error: parser.ParseError{
							Err:  errors.New("incomplete group definition, name is required and must be set"),
							Line: 3,
						},
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 4, Last: 5},
								RecordingRule: &parser.RecordingRule{
									Record: parser.YamlNode{
										Value: "up:count",
									},
									Expr: parser.PromQLExpr{
										Value: &parser.YamlNode{
											Value: "count(up)",
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
			input: []byte(`
groups:
- name: foo
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
					},
				},
			},
		},
		{
			input: []byte(`
groups: {}
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("groups value must be a list, got mapping"),
					Line: 2,
				},
			},
		},
		{
			input: []byte(`
groups:
- name: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Error: parser.ParseError{
							Err:  errors.New("group name must be a string, got list"),
							Line: 3,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  name: bar
  name: bob
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "bar",
						Error: parser.ParseError{
							Err:  errors.New("duplicated key name"),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: v1
  rules:
    rules:
      - record: up:count
        expr: count(up)
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "v1",
						Error: parser.ParseError{
							Err:  errors.New("rules must be a list, got mapping"),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: v1
  rules:
    - rules:
      - record: up:count
        expr: count(up)
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "v1",
						Rules: []parser.Rule{
							{
								Error: parser.ParseError{
									Err:  errors.New("invalid rule key rules"),
									Line: 5,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: v1
  rules:
    - rules:
      - record: up:count
		expr: count(up)
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("found a tab character that violates indentation"),
					Line: 6,
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "v1",
						Rules: []parser.Rule{
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
					},
					{
						Name: "v1",
						Rules: []parser.Rule{
							{
								Error: parser.ParseError{
									Err:  errors.New("invalid rule key rules"),
									Line: 12,
								},
							},
						},
					},
				},
				Error: parser.ParseError{
					Line: 8,
					Err:  errors.New("multi-document YAML files are not allowed"),
				},
			},
		},
		{
			input: []byte(`
---
groups: []
---
groups:
- name: foo
  rules:
    - labels: !!binary "SGVsbG8sIFdvcmxkIQ=="
      record: foo
      expr: foo
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 8, Last: 10},
								Error: parser.ParseError{
									Line: 8,
									Err:  errors.New("labels value must be a mapping, got binary data instead"),
								},
							},
						},
					},
				},
				Error: parser.ParseError{
					Line: 4,
					Err:  errors.New("multi-document YAML files are not allowed"),
				},
			},
		},
		{
			input:  []byte("[]"),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("top level field must be a groups key, got list"),
					Line: 1,
				},
			},
		},
		{
			input:  []byte("\n\n[]"),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("top level field must be a groups key, got list"),
					Line: 3,
				},
			},
		},
		{
			input:  []byte("groups: {}"),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("groups value must be a list, got mapping"),
					Line: 1,
				},
			},
		},
		{
			input:  []byte("groups: []"),
			strict: true,
		},
		{
			input:  []byte("xgroups: {}"),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("unexpected key xgroups"),
					Line: 1,
				},
			},
		},
		{
			input:  []byte("\nbob\n"),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("top level field must be a groups key, got string"),
					Line: 2,
				},
			},
		},
		{
			input: []byte(`groups: []

rules: []
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("unexpected key rules"),
					Line: 3,
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: 
  rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Error: parser.ParseError{
							Err:  errors.New("group name must be a string, got null"),
							Line: 3,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up)
    labels:
      job: foo
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Error: parser.ParseError{
									Err:  errors.New("invalid rule key xxx"),
									Line: 7,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Error: parser.ParseError{
							Err:  errors.New("rules must be a list, got mapping"),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 9},
								Error: parser.ParseError{
									Line: 9,
									Err:  errors.New("labels job value must be a string, got mapping instead"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr:
      sum: sum(up)
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 7},
								Error: parser.ParseError{
									Line: 7,
									Err:  errors.New("expr value must be a string, got mapping instead"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules: []
- name: foo
  rules: []
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("duplicated group name"),
					Line: 5,
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 8, Last: 9},
								Error: parser.ParseError{
									Line: 9,
									Err:  errors.New("duplicated labels key foo"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: v2
  rules:
  - record: up:count
    expr: count(up)
    expr: sum(up)`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "v2",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 7},
								Error: parser.ParseError{
									Line: 7,
									Err:  errors.New("duplicated expr key"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: v2
  rules:
  - record: up:count
    expr: count(up)
bogus: 1
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("unexpected key bogus"),
					Line: 7,
				},
			},
		},
		{
			input: []byte(`
groups:
- name: v2
  rules:
  - record: up:count
    expr: count(up)
    bogus: 1
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "v2",
						Rules: []parser.Rule{
							{
								Error: parser.ParseError{
									Err:  errors.New("invalid rule key bogus"),
									Line: 7,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "v2",
						Rules: []parser.Rule{
							{
								Error: parser.ParseError{
									Err:  errors.New("invalid rule key bogus"),
									Line: 11,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:

- name: CloudflareKafkaZookeeperExporter

  rules:
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "CloudflareKafkaZookeeperExporter",
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    expr: 1
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Error: parser.ParseError{
							Err:  errors.New("rules must be a list, got mapping"),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - expr: 1
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 5},
								Error: parser.ParseError{
									Line: 5,
									Err:  errors.New("incomplete rule, no alert or record key"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - expr: null
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 5},
								Error: parser.ParseError{
									Line: 5,
									Err:  errors.New("incomplete rule, no alert or record key"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - 1: null
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Error: parser.ParseError{
									Err:  errors.New("invalid rule key 1"),
									Line: 5,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - true: !!binary "SGVsbG8sIFdvcmxkIQ=="
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Error: parser.ParseError{
									Err:  errors.New("invalid rule key true"),
									Line: 5,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - expr: !!binary "SGVsbG8sIFdvcmxkIQ=="
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 5},
								Error: parser.ParseError{
									Line: 5,
									Err:  errors.New("incomplete rule, no alert or record key"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - expr: !!binary "SGVsbG8sIFdvcmxkIQ=="
      record: foo
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 6},
								Error: parser.ParseError{
									Line: 5,
									Err:  errors.New("expr value must be a string, got binary data instead"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - labels: !!binary "SGVsbG8sIFdvcmxkIQ=="
      record: foo
      expr: foo
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 7},
								Error: parser.ParseError{
									Line: 5,
									Err:  errors.New("labels value must be a mapping, got binary data instead"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Error: parser.ParseError{
					Line: 8,
					Err:  errors.New("multi-document YAML files are not allowed"),
				},
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
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
						},
					},
					{
						Name: "foo",
						Rules: []parser.Rule{
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
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - record: foo
      expr: foo
      expr: foo
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 7},
								Error: parser.ParseError{
									Line: 7,
									Err:  errors.New("duplicated expr key"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - record: foo
      keep_firing_for: 1m
      keep_firing_for: 2m
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 7},
								Error: parser.ParseError{
									Line: 7,
									Err:  errors.New("duplicated keep_firing_for key"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - record: foo
      keep_firing_for: 1m
      record: 2m
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 7},
								Error: parser.ParseError{
									Line: 7,
									Err:  errors.New("duplicated record key"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
    - []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
							{
								Error: parser.ParseError{
									Err:  errors.New("rule definion must be a mapping, got list"),
									Line: 5,
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
1: 0
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("groups key must be a string, got a integer"),
					Line: 2,
				},
			},
		},
		{
			input: []byte(`
true: 0
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("groups key must be a string, got a bool"),
					Line: 2,
				},
			},
		},
		{
			input: []byte(`
groups: !!binary "SGVsbG8sIFdvcmxkIQ=="
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("groups value must be a list, got binary data"),
					Line: 2,
				},
			},
		},
		{
			input: []byte(`
groups:
  - true: null"
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Error: parser.ParseError{
							Err:  errors.New("invalid group key true"),
							Line: 3,
						},
					},
				},
			},
		},
		{
			input: []byte(`
!!!binary "groups": true"
`),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("groups key must be a string, got a binary"),
					Line: 2,
				},
			},
		},
		{
			input:  []byte("[]"),
			strict: true,
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("top level field must be a groups key, got list"),
					Line: 1,
				},
			},
		},
		{
			input: []byte(`
groups:
  - true
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Error: parser.ParseError{
							Err:  errors.New("group must be a mapping, got bool"),
							Line: 3,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
  - name:
    rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Error: parser.ParseError{
							Err:  errors.New("group name must be a string, got null"),
							Line: 3,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
  - name: ""
    rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Error: parser.ParseError{
							Err:  errors.New("group name cannot be empty"),
							Line: 3,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
  - name: 1
    rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Error: parser.ParseError{
							Err:  errors.New("group name must be a string, got integer"),
							Line: 3,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
  - name: foo
    interval: 1
    rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Error: parser.ParseError{
							Err:  errors.New("group interval must be a string, got integer"),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
  - name: foo
    interval: xxx
    rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Error: parser.ParseError{
							Err:  errors.New("invalid interval value: not a valid duration string: \"xxx\""),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
  - name: foo
    query_offset: 1
    rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Error: parser.ParseError{
							Err:  errors.New("group query_offset must be a string, got integer"),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
  - name: foo
    query_offset: xxx
    rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "foo",
						Error: parser.ParseError{
							Err:  errors.New("invalid query_offset value: not a valid duration string: \"xxx\""),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
  - name: foo
    query_offset: 1m
    limit: abc
    rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name:        "foo",
						QueryOffset: time.Minute,
						Error: parser.ParseError{
							Err:  errors.New("group limit must be a integer, got string"),
							Line: 5,
						},
					},
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Error: parser.ParseError{
					Err:  errors.New("did not find expected alphabetic or numeric character"),
					Line: 7,
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "v2",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 5, Last: 10},
								Error: parser.ParseError{
									Line: 6,
									Err:  errors.New("for value must be a string, got integer instead"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Groups: []parser.Group{
					{
						Name:  "v2",
						Limit: 1,
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 6, Last: 10},
								Error: parser.ParseError{
									Line: 7,
									Err:  errors.New("keep_firing_for value must be a string, got integer instead"),
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: "{{ source }}"
  rules:
# pint ignore/begin
    
# pint ignore/end
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "{{ source }}",
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: |
      {"up"}
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: |
      {'up'}
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
groups:
- name: mygroup
  partial_response_strategy: bob
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "mygroup",
						Error: parser.ParseError{
							Err:  errors.New("partial_response_strategy is only valid when parser is configured to use the Thanos rule schema"),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: mygroup
  partial_response_strategy: warn
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			schema: parser.ThanosSchema,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "mygroup",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
groups:
- name: mygroup
  partial_response_strategy: abort
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			schema: parser.ThanosSchema,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "mygroup",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
groups:
- name: mygroup
  partial_response_strategy: abort
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: false,
			schema: parser.PrometheusSchema,
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "mygroup",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
groups:
- name: mygroup
  partial_response_strategy: bob
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			schema: parser.ThanosSchema,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "mygroup",
						Error: parser.ParseError{
							Err:  errors.New("invalid partial_response_strategy value: bob"),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: mygroup
  partial_response_strategy: 1
  rules:
  - record: up:count
    expr: count(up)
`),
			strict: true,
			schema: parser.ThanosSchema,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "mygroup",
						Error: parser.ParseError{
							Err:  errors.New("partial_response_strategy must be a string, got integer"),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
- alert: Multi Line
  expr: foo
          AND ON (instance)
          bar
`),
			strict: false,
			schema: parser.PrometheusSchema,
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
  - alert: FooBar
    expr: >-
      aaaaaaaaaaaaaaaaaaaaaaaa
      AND ON (colo_id) bbbbbbbbbbb
      > 2
    for: 1m
`),
			strict: false,
			schema: parser.PrometheusSchema,
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
  - alert: FooBar
    expr: 'aaaaaaaaaaaaaaaaaaaaaaaa
          AND ON (colo_id) bbbbbbbbbbb
          > 2'
    for: 1m
`),
			strict: false,
			schema: parser.PrometheusSchema,
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "foo",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "{{ source }}",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
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
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "{{ source }}",
						Rules: []parser.Rule{
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
				},
			},
		},
		{
			input: []byte(`
groups:
- name: xxx
  interval: 3m
  rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name:     "xxx",
						Interval: time.Minute * 3,
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: xxx
  interval: 3m
  rules: []
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name:     "xxx",
						Interval: time.Minute * 3,
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: xxx
  interval: 3m
  rules: []
---
groups:
- name: yyy
  interval: 2m
  rules: []
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name:     "xxx",
						Interval: time.Minute * 3,
					},
					{
						Name:     "yyy",
						Interval: time.Minute * 2,
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: xxx
  interval: 3m
  labels:
    foo: bar
  rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name:     "xxx",
						Interval: time.Minute * 3,
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
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: xxx
  labels:
    - foo: bar
  rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "xxx",
						Error: parser.ParseError{
							Err:  errors.New("group labels must be a mapping, got list"),
							Line: 4,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: xxx
  labels:
    foo: 1
  rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "xxx",
						Error: parser.ParseError{
							Err:  errors.New("labels foo value must be a string, got integer instead"),
							Line: 5,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: xxx
  labels:
    foo: bar
    bob: foo
    foo: bob
  rules: []
`),
			strict: true,
			output: parser.File{
				Groups: []parser.Group{
					{
						Name: "xxx",
						Error: parser.ParseError{
							Err:  errors.New("duplicated labels key foo"),
							Line: 7,
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: xxx
  interval: 3m
  query_offset: 1s
  limit: 5
  labels:
    foo: bar
  rules: []
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name:        "xxx",
						Interval:    time.Minute * 3,
						QueryOffset: time.Second,
						Limit:       5,
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
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: xxx
  labels:
    - foo: bar
  rules: []
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "xxx",
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
			input: []byte(`
groups:
- name: xxx
  labels:
    foo: 1
  rules: []
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "xxx",
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
										Value: "1",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
groups:
- name: xxx
  labels:
    foo: bar
    bob: foo
    foo: bob
  rules: []
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "xxx",
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
										Value: "foo",
									},
								},
								{
									Key: &parser.YamlNode{
										Value: "foo",
									},
									Value: &parser.YamlNode{
										Value: "bob",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			input: []byte(`
- name: xxx
  interval: 3m
  query_offset: 1s
  limit: 5
  labels:
    foo: bar
  rules:
  - record: up:count
    expr: count(up)
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name:        "xxx",
						Interval:    time.Minute * 3,
						QueryOffset: time.Second,
						Limit:       5,
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
							},
						},
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 9, Last: 10},
								RecordingRule: &parser.RecordingRule{
									Record: parser.YamlNode{
										Value: "up:count",
										Pos:   diags.PositionRanges{{Line: 9, FirstColumn: 13, LastColumn: 20}},
									},
									Expr: parser.PromQLExpr{
										Value: &parser.YamlNode{
											Value: "count(up)",
											Pos:   diags.PositionRanges{{Line: 10, FirstColumn: 11, LastColumn: 19}},
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
			input: []byte(`

- interval: 3m
  query_offset: 1s
  limit: 5
  labels:
    foo: bar
  rules:
  - record: up:count
    expr: count(up)
`),
			output: parser.File{
				IsRelaxed: true,
				Groups: []parser.Group{
					{
						Name: "",
						Rules: []parser.Rule{
							{
								Lines: diags.LineRange{First: 9, Last: 10},
								RecordingRule: &parser.RecordingRule{
									Record: parser.YamlNode{
										Value: "up:count",
										Pos:   diags.PositionRanges{{Line: 9, FirstColumn: 13, LastColumn: 20}},
									},
									Expr: parser.PromQLExpr{
										Value: &parser.YamlNode{
											Value: "count(up)",
											Pos:   diags.PositionRanges{{Line: 10, FirstColumn: 11, LastColumn: 19}},
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
			input: []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    shard: "0"
    total-shards: "1"
  name: shard-jobs
  namespace: foo
data:
  jobs.yaml: |
    jobs:
      - name: foo
        interval: 1m
        queries:
          - name: xxx
            help: ''
            labels:
              - account_id
              - bucket
              - cache_status
        mtls_identity:
          cert_path: /etc/identity/tls.crt
          key_path: /etc/identity/tls.key
`),
			output: parser.File{
				Groups:    nil,
				IsRelaxed: true,
			},
		},
	}

	alwaysEqual := cmp.Comparer(func(_, _ any) bool { return true })
	ignorePrometheusExpr := cmp.FilterValues(func(x, y any) bool {
		_, xe := x.(*parser.PromQLNode)
		_, ye := y.(*parser.PromQLNode)
		return xe || ye
	}, alwaysEqual)

	cmpErrorText := cmp.Comparer(func(x, y any) bool {
		xe := x.(error)
		ye := y.(error)
		return xe.Error() == ye.Error()
	})
	sameErrorText := cmp.FilterValues(func(x, y any) bool {
		_, xe := x.(error)
		_, ye := y.(error)
		return xe && ye
	}, cmpErrorText)

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			t.Logf("\n--- Content ---%s--- END ---", tc.input)

			s := bufio.NewScanner(bytes.NewReader(tc.input))
			for s.Scan() {
				tc.output.TotalLines++
			}

			p := parser.NewParser(tc.strict, tc.schema, tc.names)
			file := p.Parse(bytes.NewReader(tc.input))

			if diff := cmp.Diff(tc.output, file,
				ignorePrometheusExpr,
				sameErrorText,
				cmpopts.IgnoreFields(parser.YamlNode{}, "Pos"), // FIXME remove?
				cmpopts.IgnoreUnexported(parser.PromQLExpr{}),
			); diff != "" {
				t.Errorf("Parse() returned wrong output (-want +got):\n%s", diff)
				return
			}
		})
	}
}

func TestPromQLExprSyntaxError(t *testing.T) {
	type testCaseT struct {
		name          string
		expectedError string
		input         []byte
		schema        parser.Schema
		names         model.ValidationScheme
	}

	testCases := []testCaseT{
		{
			name: "invalid label matching operator",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: |
      {'up' == 1}
`),
			expectedError: `1:8: parse error: unexpected "=" in label matching, expected string`,
			schema:        parser.PrometheusSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "unclosed parenthesis",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up
`),
			expectedError: `1:7: parse error: unclosed left parenthesis`,
			schema:        parser.PrometheusSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "unclosed brace",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: up{job="test"
`),
			expectedError: `1:14: parse error: unexpected end of input inside braces`,
			schema:        parser.PrometheusSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "invalid aggregation without grouping",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: sum without (up)
`),
			expectedError: `1:17: parse error: unexpected end of input in aggregation`,
			schema:        parser.PrometheusSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "bad regex syntax",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: up{job=~"["}
`),
			expectedError: `1:4: parse error: error parsing regexp: missing closing ]: ` + "`[`",
			schema:        parser.PrometheusSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "invalid binary operator position",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: up +
`),
			expectedError: `1:5: parse error: unexpected end of input`,
			schema:        parser.PrometheusSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "multiple binary operators",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: up + * down
`),
			expectedError: `1:6: parse error: unexpected <op:*>`,
			schema:        parser.PrometheusSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "invalid duration syntax",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: rate(up[5x])
`),
			expectedError: `1:9: parse error: bad number or duration syntax: "5"`,
			schema:        parser.PrometheusSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "missing range selector",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: rate(up)
`),
			expectedError: `1:6: parse error: expected type range vector in call to function "rate", got instant vector`,
			schema:        parser.PrometheusSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "invalid label name in matcher",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: up{123="test"}
`),
			expectedError: `1:4: parse error: unexpected character inside braces: '1'`,
			schema:        parser.PrometheusSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "unclosed parenthesis with Thanos schema",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: sum(up
`),
			expectedError: `1:7: parse error: unclosed left parenthesis`,
			schema:        parser.ThanosSchema,
			names:         model.LegacyValidation,
		},
		{
			name: "invalid binary operator with UTF8 validation",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: up +
`),
			expectedError: `1:5: parse error: unexpected end of input`,
			schema:        parser.PrometheusSchema,
			names:         model.UTF8Validation,
		},
		{
			name: "bad regex with Thanos schema and UTF8 validation",
			input: []byte(`
groups:
- name: foo
  rules:
  - record: foo
    expr: up{job=~"["}
`),
			expectedError: `1:4: parse error: error parsing regexp: missing closing ]: ` + "`[`",
			schema:        parser.ThanosSchema,
			names:         model.UTF8Validation,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := parser.NewParser(false, tc.schema, tc.names)
			r := bytes.NewReader(tc.input)
			file := p.Parse(r)

			require.NoError(t, file.Error.Err)
			require.NotEmpty(t, file.Groups)
			for _, group := range file.Groups {
				for _, rule := range group.Rules {
					require.NoError(t, rule.Error.Err)
					expr := rule.Expr()
					err := expr.SyntaxError()
					require.Error(t, err)
					require.Equal(t, tc.expectedError, err.Error())
				}
			}
		})
	}
}

func BenchmarkParse(b *testing.B) {
	data, err := os.ReadFile("testrules.yml")
	require.NoError(b, err)

	p := parser.NewParser(true, parser.PrometheusSchema, model.LegacyValidation)
	for b.Loop() {
		b.StopTimer()
		r := bytes.NewReader(data)
		b.StartTimer()

		f := p.Parse(r)

		b.StopTimer()
		require.Len(b, f.Groups, 90)
		require.NoError(b, f.Error.Err)
		require.Equal(b, 5501, f.TotalLines)
		b.StartTimer()
	}
}
