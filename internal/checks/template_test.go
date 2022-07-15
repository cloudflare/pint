package checks_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
)

func TestTemplatedRegexpExpand(t *testing.T) {
	type testCaseT struct {
		input  string
		rule   parser.Rule
		output string
		err    string
	}

	testCases := []testCaseT{
		{
			input: "{{ xxx }}",
			rule:  parser.Rule{},
			err:   `template: regexp:1: function "xxx" not defined`,
		},
		{
			input: "{{ $xxx := yyy }}",
			rule:  parser.Rule{},
			err:   `template: regexp:1: function "yyy" not defined`,
		},
		{
			input: "{{nil}}",
			rule:  parser.Rule{},
			err:   `template: regexp:1:125: executing "regexp" at <nil>: nil is not a command`,
		},
		{
			input:  "",
			rule:   parser.Rule{},
			output: "^$",
		},
		{
			input:  "foo",
			rule:   parser.Rule{},
			output: "^foo$",
		},
		{
			input:  "foo [a-z]+ bar",
			rule:   parser.Rule{},
			output: "^foo [a-z]+ bar$",
		},
		{
			input:  "for is {{ $for }}",
			rule:   newMustRule("- alert: foo\n  expr: foo\n  for: 5m\n"),
			output: "^for is 5m$",
		},
		{
			input:  "record is {{ $record }}!",
			rule:   newMustRule("- record: foo\n  expr: foo\n"),
			output: "^record is foo!$",
		},
		{
			input:  "label foo is {{ $labels.foo }}!",
			rule:   newMustRule("- record: foo\n  expr: foo\n  labels:\n    foo: bar\n"),
			output: "^label foo is bar!$",
		},
		{
			input:  "alert is {{ $alert }}",
			rule:   newMustRule("- record: foo\n  expr: foo\n  labels:\n    foo: bar\n"),
			output: "^alert is $",
		},
		{
			input:  "label foo is {{ $labels.bar }}!",
			rule:   newMustRule("- record: foo\n  expr: foo\n  labels:\n    foo: bar\n"),
			output: "^label foo is !$",
		},
		{
			input:  "label foo is {{ $labels.foo }}!",
			rule:   newMustRule("- alert: foo\n  expr: foo\n  labels:\n    foo: bar\n"),
			output: "^label foo is bar!$",
		},
		{
			input:  "annotation foo is {{ $labels.foo }}!",
			rule:   newMustRule("- alert: foo\n  expr: foo\n  annotations:\n    foo: bar\n"),
			output: "^annotation foo is bar!$",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			tr, err := checks.NewTemplatedRegexp(tc.input)
			if err != nil {
				require.EqualError(t, err, tc.err)
				return
			}

			re, err := tr.Expand(tc.rule)
			if err != nil {
				require.EqualError(t, err, tc.err)
				return
			}
			require.Empty(t, tc.err)
			require.Equal(t, tc.output, re.String())
		})
	}
}

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
