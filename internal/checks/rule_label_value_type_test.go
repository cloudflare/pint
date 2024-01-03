package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func TestRuleLabelValueTypeCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "label is not a string in recording rule / required",
			content:     "- record: foo\n  expr: rate(foo[1m])\n  labels:\n    foo: true\n",
			checker: func(_ *promapi.FailoverGroup) checks.RuleChecker {
				return checks.NewRuleLabelValueTypeCheck()
			},
			prometheus: noProm,
			problems: func(uri string) []checks.Problem {
				return []checks.Problem{{
					Lines: parser.LineRange{
						First: 4,
						Last:  4,
					},
					Reporter: checks.RuleLabelValueTypeName,
					Text:     "`foo` label value must be a string, got `!!bool`.",
					Severity: checks.Bug,
				}}
			},
		},
	}
	runTests(t, testCases)
}
