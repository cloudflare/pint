package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestSelector(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores recording rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp(".+"), nil, "job", "comment", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "ignores alerting rules with syntax errors",
			content:     "- alert: foo\n  expr: sum(foo) without(\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp(".+"), nil, "job", "comment", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "label present",
			content: `
- alert: foo
  expr: errors{job="foo"} > 0
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp(".+"), nil, "job", "comment", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "label missing",
			content: `
- alert: foo
  expr: errors > 0
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp(".+"), nil, "job", "comment", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "label missing / __name__",
			content: `
- alert: foo
  expr: |
    {__name__="errors"} > 0
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp(".+"), nil, "job", "comment", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "label missing / __name__ regexp",
			content: `
- alert: foo
  expr: |
    {__name__=~"errors"} > 0
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp(".+"), nil, "job", "comment", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "label missing / matchers",
			content: `
- alert: foo
  expr: errors{cluster="dev", instance="foo"} > 0
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp(".+"), nil, "job", "comment", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "label missing / name mismatch",
			content: `
- alert: foo
  expr: errors{cluster="dev"} > 0
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp("foo"), nil, "job", "comment", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "label missing / regexp name mismatch",
			content: `
- alert: foo
  expr: |
    {__name__=~"errors|foo", cluster="dev"} > 0
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp("foo"), nil, "job", "comment", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "label present inside call",
			content: `
- alert: foo
  expr: absent(errors{job="foo"})
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp(".+"), checks.MustTemplatedRegexp(".+"), "job", "comment", checks.Warning)
			},
			prometheus: noProm,
		},
		{
			description: "label missing inside call",
			content: `
- alert: foo
  expr: absent(errors{xjob="foo"})
`,
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewSelectorCheck(checks.MustTemplatedRegexp(".+"), checks.MustTemplatedRegexp(".+"), "job", "comment", checks.Warning)
			},
			prometheus: noProm,
			problems:   true,
		},
	}
	runTests(t, testCases)
}
