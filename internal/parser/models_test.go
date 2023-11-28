package parser_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
)

func newMustRule(content string) parser.Rule {
	p := parser.NewParser()
	rules, err := p.Parse([]byte(content))
	if err != nil {
		panic(err)
	}
	for _, rule := range rules {
		return rule
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
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			equal := tc.a.IsIdentical(tc.b)
			require.Equal(t, tc.equal, equal)
		})
	}
}
