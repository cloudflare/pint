package parser_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/parser"
)

func newMustRule(content string) parser.Rule {
	p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
	file := p.Parse(strings.NewReader(content))
	if file.Error.Err != nil {
		panic(file.Error.Err)
	}
	for _, group := range file.Groups {
		for _, rule := range group.Rules {
			return rule
		}
	}
	return parser.Rule{}
}

func TestRuleIsIdentical(t *testing.T) {
	type testCaseT struct {
		a     parser.Rule
		b     parser.Rule
		equal bool
	}

	testCases := []testCaseT{
		{
			a:     parser.Rule{},
			b:     parser.Rule{},
			equal: true,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n"),
			equal: true,
		},
		{
			a:     newMustRule("- record: bob\n  expr: bob\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n  labels: {}\n"),
			equal: true,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n  labels: {}\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n"),
			equal: true,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n"),
			b:     newMustRule("- expr: bob\n  record: foo\n"),
			equal: true,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    bar: bar\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n    extra: foo\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n    extra: foo\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n    extra: bar\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n    abc: foo\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n    cba: foo\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n    extra: foo\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n    extra: foo\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n  labels:\n    foo: bar\n    extra: bar\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n"),
			b:     newMustRule("- record: foo\n  xxx: bob\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: foo\n  expr: bob\n"),
			b:     newMustRule("- record: foo\n  expr: bob == 0\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n  for: 4m\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n"),
			b:     newMustRule("- record: foo\n  expr: bob\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n"),
			equal: true,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n"),
			b:     newMustRule("- alert: foo\n  expr: bob(\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n"),
			b:     newMustRule("- alert: foo1\n  expr: bob\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n"),
			b:     newMustRule("- alert: foo\n  expr: bob1\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n  for: 4m\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n  for: 5m\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n  for: 4m\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n  keep_firing_for: 4m\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n  keep_firing_for: 4m\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n  for: 4m\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n  keep_firing_for: 3m\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n  keep_firing_for: 4m\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n  labels:\n    foo: bar\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n  labels:\n    bar: bar\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n  labels:\n    foo: bar\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n  annotations:\n    foo: bar\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n  annotations:\n    bar: bar\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n  annotations:\n    foo: bar\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  # pint disable promql/series\n  expr: bob\n"),
			b:     newMustRule("- alert: foo\n  expr: bob\n"),
			equal: false,
		},
		{
			a:     newMustRule("- alert: foo\n  expr: bob\n"),
			b:     newMustRule("- alert: foo\n  # pint disable promql/series\n  expr: bob\n"),
			equal: false,
		},
		{
			a:     newMustRule("- record: colo:labels:equal\n  expr: sum(foo) without(job)\n  labels:\n    same: yes\n"),
			b:     newMustRule("- record: colo:labels:equal\n  expr: sum(foo) without(job)\n  labels:\n    same: yes\n"),
			equal: true,
		},
		{
			a:     newMustRule("- record: foo\n  expr: sum(foo)\n  labels:\n    same: yes\n    a: b\n    d: c\n"),
			b:     newMustRule("- record: foo\n  expr: sum(foo)\n  labels:\n    same: yes\n    a: b\n    c: d\n"),
			equal: false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			equal := tc.a.IsIdentical(tc.b)
			require.Equal(t, tc.equal, equal, tc.a)
		})
	}
}

func TestRuleIsSame(t *testing.T) {
	type testCaseT struct {
		a    parser.Rule
		b    parser.Rule
		same bool
	}

	testCases := []testCaseT{
		// Both empty rules
		{
			a:    parser.Rule{},
			b:    parser.Rule{},
			same: true,
		},
		// Different rule types (alerting vs recording)
		{
			a:    newMustRule("- alert: foo\n  expr: bob\n"),
			b:    newMustRule("- record: foo\n  expr: bob\n"),
			same: false,
		},
		// Different rule types (recording vs alerting)
		{
			a:    newMustRule("- record: foo\n  expr: bob\n"),
			b:    newMustRule("- alert: foo\n  expr: bob\n"),
			same: false,
		},
		// One has RecordingRule, other has neither (AlertingRule matches as nil)
		{
			a: parser.Rule{
				RecordingRule: &parser.RecordingRule{},
			},
			b:    parser.Rule{},
			same: false,
		},
		// Same alerting rules
		{
			a:    newMustRule("- alert: foo\n  expr: bob\n"),
			b:    newMustRule("- alert: foo\n  expr: bob\n"),
			same: true,
		},
		// Same recording rules
		{
			a:    newMustRule("- record: foo\n  expr: bob\n"),
			b:    newMustRule("- record: foo\n  expr: bob\n"),
			same: true,
		},
		// Different line ranges - First line differs
		{
			a: parser.Rule{
				Lines: diags.LineRange{First: 1, Last: 3},
			},
			b: parser.Rule{
				Lines: diags.LineRange{First: 2, Last: 3},
			},
			same: false,
		},
		// Different line ranges - Last line differs
		{
			a: parser.Rule{
				Lines: diags.LineRange{First: 1, Last: 3},
			},
			b: parser.Rule{
				Lines: diags.LineRange{First: 1, Last: 4},
			},
			same: false,
		},
		// Different errors
		{
			a: parser.Rule{
				Error: parser.ParseError{Line: 1, Err: nil},
			},
			b: parser.Rule{
				Error: parser.ParseError{Line: 2, Err: nil},
			},
			same: false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			same := tc.a.IsSame(tc.b)
			require.Equal(t, tc.same, same)
		})
	}
}

func TestRuleLastKey(t *testing.T) {
	type testCaseT struct {
		expectedKey  string
		rule         parser.Rule
		expectedLine int
	}

	testCases := []testCaseT{
		// Recording rule - record is last key
		{
			rule:         newMustRule("- record: foo\n  expr: bob\n"),
			expectedKey:  "bob",
			expectedLine: 2,
		},
		// Recording rule - expr is last (comes after record in YAML)
		{
			rule:         newMustRule("- expr: bob\n  record: foo\n"),
			expectedKey:  "foo",
			expectedLine: 2,
		},
		// Recording rule with labels - label key is last
		{
			rule:         newMustRule("- record: foo\n  expr: bob\n  labels:\n    severity: critical\n"),
			expectedKey:  "severity",
			expectedLine: 4,
		},
		// Alerting rule - alert is last key
		{
			rule:         newMustRule("- alert: foo\n  expr: bob\n"),
			expectedKey:  "bob",
			expectedLine: 2,
		},
		// Alerting rule with for
		{
			rule:         newMustRule("- alert: foo\n  expr: bob\n  for: 5m\n"),
			expectedKey:  "5m",
			expectedLine: 3,
		},
		// Alerting rule with keep_firing_for
		{
			rule:         newMustRule("- alert: foo\n  expr: bob\n  keep_firing_for: 10m\n"),
			expectedKey:  "10m",
			expectedLine: 3,
		},
		// Alerting rule with labels
		{
			rule:         newMustRule("- alert: foo\n  expr: bob\n  labels:\n    severity: critical\n"),
			expectedKey:  "severity",
			expectedLine: 4,
		},
		// Alerting rule with annotations
		{
			rule:         newMustRule("- alert: foo\n  expr: bob\n  annotations:\n    summary: test\n"),
			expectedKey:  "summary",
			expectedLine: 4,
		},
		// Alerting rule with both labels and annotations - annotations last
		{
			rule:         newMustRule("- alert: foo\n  expr: bob\n  labels:\n    severity: critical\n  annotations:\n    summary: test\n"),
			expectedKey:  "summary",
			expectedLine: 6,
		},
		// Alerting rule with for, keep_firing_for, labels, annotations - annotations last
		{
			rule:         newMustRule("- alert: foo\n  expr: bob\n  for: 5m\n  keep_firing_for: 10m\n  labels:\n    severity: critical\n  annotations:\n    summary: test\n"),
			expectedKey:  "summary",
			expectedLine: 8,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			lastKey := tc.rule.LastKey()
			require.NotNil(t, lastKey)
			require.Equal(t, tc.expectedKey, lastKey.Value)
			require.Equal(t, tc.expectedLine, lastKey.Pos.Lines().Last)
		})
	}
}

func TestAlertingRuleIsIdenticalNilCases(t *testing.T) {
	// Both nil
	var nilA *parser.AlertingRule
	var nilB *parser.AlertingRule
	require.True(t, nilA.IsIdentical(nilB))

	// One nil, one not
	rule := newMustRule("- alert: foo\n  expr: bob\n")
	require.False(t, nilA.IsIdentical(rule.AlertingRule))
	require.False(t, rule.AlertingRule.IsIdentical(nilB))
}

func TestRecordingRuleIsIdenticalNilCases(t *testing.T) {
	// Both nil
	var nilA *parser.RecordingRule
	var nilB *parser.RecordingRule
	require.True(t, nilA.IsIdentical(nilB))

	// One nil, one not
	rule := newMustRule("- record: foo\n  expr: bob\n")
	require.False(t, nilA.IsIdentical(rule.RecordingRule))
	require.False(t, rule.RecordingRule.IsIdentical(nilB))
}

func TestMergeMaps(t *testing.T) {
	type testCaseT struct {
		a      *parser.YamlMap
		b      *parser.YamlMap
		output *parser.YamlMap
	}

	testCases := []testCaseT{
		{
			a:      nil,
			b:      nil,
			output: nil,
		},
		{
			a:      &parser.YamlMap{},
			b:      nil,
			output: &parser.YamlMap{},
		},
		{
			a:      nil,
			b:      &parser.YamlMap{},
			output: &parser.YamlMap{},
		},
		{
			a:      &parser.YamlMap{},
			b:      &parser.YamlMap{},
			output: &parser.YamlMap{},
		},
		{
			a: &parser.YamlMap{
				Key: &parser.YamlNode{Value: "a"},
				Items: []*parser.YamlKeyValue{
					{
						Key: &parser.YamlNode{Value: "1"},
					},
					{
						Key: &parser.YamlNode{Value: "2"},
					},
				},
			},
			b: &parser.YamlMap{
				Key: &parser.YamlNode{Value: "b"},
				Items: []*parser.YamlKeyValue{
					{
						Key: &parser.YamlNode{Value: "2"},
					},
					{
						Key: &parser.YamlNode{Value: "3"},
					},
				},
			},
			output: &parser.YamlMap{
				Key: &parser.YamlNode{Value: "a"},
				Items: []*parser.YamlKeyValue{
					{
						Key: &parser.YamlNode{Value: "1"},
					},
					{
						Key: &parser.YamlNode{Value: "2"},
					},
					{
						Key: &parser.YamlNode{Value: "3"},
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			output := parser.MergeMaps(tc.a, tc.b)
			require.Equal(t, tc.output, output)
		})
	}
}
