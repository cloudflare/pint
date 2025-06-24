package checks_test

import (
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newFragileCheck(_ *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewFragileCheck()
}

func TestFragileCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores syntax errors",
			content:     "- record: foo\n  expr: up ==\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
		},
		{
			description: "ignores recording rules",
			content:     "- record: foo\n  expr: up == 0\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
		},
		{
			description: "ignores other functions",
			content:     "- alert: foo\n  expr: sum(foo) without(job)\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
		},
		{
			description: "warns about topk() as source of series",
			content:     "- alert: foo\n  expr: topk(10, foo)\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "warns about topk() as source of series (or)",
			content:     "- alert: foo\n  expr: bar or topk(10, foo)\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "warns about topk() as source of series (multiple)",
			content:     "- alert: foo\n  expr: bar or topk(10, foo) or bottomk(10, foo)\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
			problems:    true,
		},
		{
			description: "ignores aggregated topk()",
			content:     "- alert: foo\n  expr: min(topk(10, foo)) > 5000\n",
			checker:     newFragileCheck,
			prometheus:  noProm,
		},
		{
			description: "fragile offset /",
			content: `
- alert: foo
  expr: sum(selector{job="foo"}) / sum(selector{job="foo"} offset 30m) > 5
`,
			checker:    newFragileCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "fragile offset *",
			content: `
- alert: foo
  expr: sum(selector{job="foo"}) * sum(selector{job="foo"} offset 30m) > 5
`,
			checker:    newFragileCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "fragile offset +",
			content: `
- alert: foo
  expr: sum(selector{job="foo"}) + sum(selector{job="foo"} offset 30m) > 5
`,
			checker:    newFragileCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "fragile offset -",
			content: `
- alert: foo
  expr: sum(selector{job="foo"}) - sum(selector{job="foo"} offset 30m) > 5
`,
			checker:    newFragileCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "fragile offset ^",
			content: `
- alert: foo
  expr: sum(selector{job="foo"}) ^ sum(selector{job="foo"} offset 30m) > 5
`,
			checker:    newFragileCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "fragile offset %",
			content: `
- alert: foo
  expr: sum(selector{job="foo"}) % sum(selector{job="foo"} offset 30m) > 5
`,
			checker:    newFragileCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "fragile offset and",
			content: `
- alert: foo
  expr: (sum(selector{job="foo"}) and sum(selector{job="foo"} offset 30m)) > 5
`,
			checker:    newFragileCheck,
			prometheus: noProm,
		},
		{
			description: "fragile offset without condition",
			content: `
- alert: foo
  expr: sum(selector{job="foo"}) / sum(selector{job="foo"} offset 30m)
`,
			checker:    newFragileCheck,
			prometheus: noProm,
		},
		{
			description: "fragile offset with for",
			content: `
- alert: foo
  expr: sum(selector{job="foo"}) / sum(selector{job="foo"} offset 30m) > 5
  for: 1m
`,
			checker:    newFragileCheck,
			prometheus: noProm,
		},
		{
			description: "aggr / non aggr",
			content: `
- alert: foo
  expr: sum(selector{job="foo"}) / selector{job="foo"} offset 30m > 5
`,
			checker:    newFragileCheck,
			prometheus: noProm,
		},
		{
			description: "fragile offset but mismatched labels",
			content: `
- alert: foo
  expr: sum(selector{job="foo"}) / sum(selector{job="bob"} offset 30m) > 5
`,
			checker:    newFragileCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "fragile division / different targets",
			content: `
- alert: foo
  expr: sum(rate(metric_a[30m])) / sum(max_over_time(metric_b[30m])) >= 0.02
`,
			checker:    newFragileCheck,
			prometheus: noProm,
			problems:   true,
		},
		{
			description: "false positive aggregation",
			content: `
- alert: foo
  expr: |
    count by (dc) (
      max(0 < (token_expiration - time()) < (6*60*60)) by (instance)
      * on (instance) group_right label_replace(
        configured_minions, "instance", "$1", "minion", "(.+)")
      ) > 5
`,
			checker:    newFragileCheck,
			prometheus: noProm,
		},
		{
			description: "false positive max - issues 1466",
			content: `
- alert: KubeNodeEviction
  expr: |
    sum(rate(kubelet_evictions{job="kubelet"}[15m])) by(cluster, eviction_signal, instance)
    * on (cluster, instance) group_left(node)
    max by (cluster, instance, node) (
      kubelet_node_name{job="kubelet"}
    )
    > 0
  for: 0s
`,
			checker:    newFragileCheck,
			prometheus: noProm,
		},
	}

	runTests(t, testCases)
}
