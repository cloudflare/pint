package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newSyntaxCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewSyntaxCheck()
}

func TestSyntaxCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "valid recording rule",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newSyntaxCheck,
			prometheus:  noProm,
		},
		{
			description: "valid alerting rule",
			content:     "- alert: foo\n  expr: sum(foo)\n",
			checker:     newSyntaxCheck,
			prometheus:  noProm,
		},
		/* FIXME this test rendomly fails because promql error has empty position.
		{
			description: "no arguments for aggregate expression provided",
			content:     "- record: foo\n  expr: sum(\n",
			checker:     newSyntaxCheck,
			prometheus:  noProm,
			problems: true,
		},
		*/
		{
			description: "unclosed left parenthesis",
			content:     "- record: foo\n  expr: sum(foo) by(",
			checker:     newSyntaxCheck,
			prometheus:  noProm,
			problems:    true,
		},
	}
	runTests(t, testCases)
}
