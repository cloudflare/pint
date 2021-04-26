package checks_test

import (
	"regexp"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
)

func TestAnnotationCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     checks.NewAnnotationCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
		},
		{
			description: "doesn't ignore rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker:     checks.NewAnnotationCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "alert: foo",
					Lines:    []int{1, 2},
					Reporter: "alerts/annotation",
					Text:     "severity annotation is required",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "no annotations / required",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker:     checks.NewAnnotationCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "alert: foo",
					Lines:    []int{1, 2},
					Reporter: "alerts/annotation",
					Text:     "severity annotation is required",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "no annotations / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker:     checks.NewAnnotationCheck("severity", regexp.MustCompile("^critical$"), false, checks.Warning),
		},
		{
			description: "missing annotation / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker:     checks.NewAnnotationCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "annotations:",
					Lines:    []int{3, 4},
					Reporter: "alerts/annotation",
					Text:     "severity annotation is required",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "missing annotation / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    foo: bar\n",
			checker:     checks.NewAnnotationCheck("severity", regexp.MustCompile("^critical$"), false, checks.Warning),
		},
		{
			description: "wrong annotation value / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: bar\n",
			checker:     checks.NewAnnotationCheck("severity", regexp.MustCompile("^critical$"), true, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "severity: bar",
					Lines:    []int{4},
					Reporter: "alerts/annotation",
					Text:     "severity annotation value must match regex: ^critical$",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "wrong annotation value / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: bar\n",
			checker:     checks.NewAnnotationCheck("severity", regexp.MustCompile("^critical$"), false, checks.Warning),
			problems: []checks.Problem{
				{
					Fragment: "severity: bar",
					Lines:    []int{4},
					Reporter: "alerts/annotation",
					Text:     "severity annotation value must match regex: ^critical$",
					Severity: checks.Warning,
				},
			},
		},
		{
			description: "valid annotation / required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: info\n",
			checker:     checks.NewAnnotationCheck("severity", regexp.MustCompile("^critical|info|debug$"), true, checks.Warning),
		},
		{
			description: "valid annotation / not required",
			content:     "- alert: foo\n  expr: sum(foo)\n  annotations:\n    severity: info\n",
			checker:     checks.NewAnnotationCheck("severity", regexp.MustCompile("^critical|info|debug$"), false, checks.Warning),
		},
	}
	runTests(t, testCases)
}
