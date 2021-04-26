package checks_test

import (
	"regexp"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestLabelCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "doesn't ignore rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "record: foo",
					Lines:    []int{1, 2},
					Reporter: "rule/label",
					Text:     "severity label is required",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "no labels in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "record: foo",
					Lines:    []int{1, 2},
					Reporter: "rule/label",
					Text:     "severity label is required",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "no labels in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), false, checks.Warning),
		},
		{
			description: "missing label in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "labels:",
					Lines:    []int{3, 4},
					Reporter: "rule/label",
					Text:     "severity label is required",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "missing label in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), false, checks.Warning),
		},
		{
			description: "invalid value in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "severity: warning",
					Lines:    []int{4},
					Reporter: "rule/label",
					Text:     "severity label value must match regex: ^critical$",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "invalid value in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), false, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "severity: warning",
					Lines:    []int{4},
					Reporter: "rule/label",
					Text:     "severity label value must match regex: ^critical$",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "typo in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    priority: 2a\n",
			checker:     checks.NewLabelCheck("priority", regexp.MustCompile("^(1|2|3)$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "priority: 2a",
					Lines:    []int{4},
					Reporter: "rule/label",
					Text:     "priority label value must match regex: ^(1|2|3)$",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "typo in recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    priority: 2a\n",
			checker:     checks.NewLabelCheck("priority", regexp.MustCompile("^(1|2|3)$"), false, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "priority: 2a",
					Lines:    []int{4},
					Reporter: "rule/label",
					Text:     "priority label value must match regex: ^(1|2|3)$",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "no labels in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "alert: foo",
					Lines:    []int{1, 2},
					Reporter: "rule/label",
					Text:     "severity label is required",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "no labels in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), false, checks.Warning),
		},
		{
			description: "missing label in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "labels:",
					Lines:    []int{3, 4},
					Reporter: "rule/label",
					Text:     "severity label is required",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "missing label in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: bar\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical$"), false, checks.Warning),
		},
		{
			description: "invalid value in alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical|info$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "severity: warning",
					Lines:    []int{4},
					Reporter: "rule/label",
					Text:     "severity label value must match regex: ^critical|info$",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "invalid value in alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: warning\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical|info$"), false, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "severity: warning",
					Lines:    []int{4},
					Reporter: "rule/label",
					Text:     "severity label value must match regex: ^critical|info$",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "valid recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: critical\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical|info$"), true, checks.Warning),
		},
		{
			description: "valid recording rule / not required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: critical\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical|info$"), false, checks.Warning),
		},
		{
			description: "valid alerting rule / required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: info\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical|info$"), true, checks.Warning),
		},
		{
			description: "valid alerting rule / not required",
			content:     "- alert: foo\n  expr: rate(foo[1m])\n  labels:\n    severity: info\n",
			checker:     checks.NewLabelCheck("severity", regexp.MustCompile("^critical|info$"), false, checks.Warning),
		},
	}
	runTests(t, testCases)
}
