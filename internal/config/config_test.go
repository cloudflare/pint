package config_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/parser"
)

func TestMain(t *testing.M) {
	v := t.Run()
	snaps.Clean(t)
	os.Exit(v)
}

func TestConfigLoadMissingFile(t *testing.T) {
	_, err := config.Load("/foo/bar/pint.hcl", true)
	require.EqualError(t, err, "<nil>: Configuration file not found; The configuration file /foo/bar/pint.hcl does not exist.")
}

func TestConfigLoadMissingFileOk(t *testing.T) {
	_, err := config.Load("/foo/bar/pint.hcl", false)
	require.NoError(t, err)
}

func TestDisableOnlineChecksWithPrometheus(t *testing.T) {
	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := os.WriteFile(path, []byte(`
prometheus "prom" {
  uri     = "http://localhost"
}
`), 0o644)
	require.NoError(t, err)

	cfg, err := config.Load(path, true)
	require.NoError(t, err)
	require.Empty(t, cfg.Checks.Disabled)

	cfg.DisableOnlineChecks()
	for _, c := range checks.OnlineChecks {
		require.Contains(t, cfg.Checks.Disabled, c)
	}
}

func TestDisableOnlineChecksWithoutPrometheus(t *testing.T) {
	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := os.WriteFile(path, []byte(``), 0o644)
	require.NoError(t, err)

	cfg, err := config.Load(path, true)
	require.NoError(t, err)
	require.Empty(t, cfg.Checks.Disabled)

	cfg.DisableOnlineChecks()
	for _, c := range checks.OnlineChecks {
		require.Contains(t, cfg.Checks.Disabled, c)
	}
}

func TestDisableOnlineChecksAfterSetDisabledChecks(t *testing.T) {
	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := os.WriteFile(path, []byte(`
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
}
`), 0o644)
	require.NoError(t, err)

	cfg, err := config.Load(path, true)
	require.NoError(t, err)
	require.Empty(t, cfg.Checks.Disabled)

	cfg.SetDisabledChecks([]string{checks.SyntaxCheckName})
	require.Contains(t, cfg.Checks.Disabled, checks.SyntaxCheckName)

	cfg.SetDisabledChecks([]string{checks.RateCheckName})
	require.Contains(t, cfg.Checks.Disabled, checks.RateCheckName)

	cfg.DisableOnlineChecks()
	for _, c := range checks.OnlineChecks {
		require.Contains(t, cfg.Checks.Disabled, c)
	}
}

func TestSetDisabledChecks(t *testing.T) {
	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := os.WriteFile(path, []byte(``), 0o644)
	require.NoError(t, err)

	cfg, err := config.Load(path, true)
	require.NoError(t, err)
	require.Empty(t, cfg.Checks.Disabled)

	cfg.SetDisabledChecks([]string{checks.SyntaxCheckName})
	cfg.SetDisabledChecks([]string{checks.SyntaxCheckName})
	cfg.SetDisabledChecks([]string{checks.RateCheckName})
	require.Equal(t, []string{checks.SyntaxCheckName, checks.RateCheckName}, cfg.Checks.Disabled)
}

func newRule(t *testing.T, content string) parser.Rule {
	p := parser.NewParser()
	rules, err := p.Parse([]byte(content))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	return rules[0]
}

func TestGetChecksForRule(t *testing.T) {
	type testCaseT struct {
		title          string
		config         string
		path           string
		rule           parser.Rule
		checks         []string
		disabledChecks []string
	}

	testCases := []testCaseT{
		{
			title:  "defaults",
			config: "",
			path:   "rules.yml",
			rule:   newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "single prometheus server",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.RateCheckName + "(prom)",
				checks.SeriesCheckName + "(prom)",
				checks.VectorMatchingCheckName + "(prom)",
				checks.RangeQueryCheckName + "(prom)",
				checks.RuleDuplicateCheckName + "(prom)",
				checks.LabelsConflictCheckName + "(prom)",
			},
		},
		{
			title: "multiple URIs",
			config: `
prometheus "prom" {
  uri      = "http://localhost"
  failover = ["http://localhost/1", "http://localhost/2"]
  timeout  = "1s"
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.RateCheckName + "(prom)",
				checks.SeriesCheckName + "(prom)",
				checks.VectorMatchingCheckName + "(prom)",
				checks.RangeQueryCheckName + "(prom)",
				checks.RuleDuplicateCheckName + "(prom)",
				checks.LabelsConflictCheckName + "(prom)",
			},
		},
		{
			title: "two prometheus servers / disable all checks via comment",
			config: `
prometheus "prom1" {
  uri     = "http://localhost/1"
  timeout = "1s"
}
prometheus "prom2" {
  uri     = "http://localhost/2"
  timeout = "1s"
}
checks {
  disabled = [ "alerts/template" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
# pint disable promql/rate
# pint disable promql/series
# pint disable promql/vector_matching
# pint disable promql/range_query
# pint disable rule/duplicate
# pint disable labels/conflict
- record: foo
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "single prometheus server / path mismatch",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
  include = [ "foo.yml" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "single prometheus server / include & exclude",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
  include = [ ".*" ]
  exclude = [ "rules.yml" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "single prometheus server / excluded",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
  exclude = [ "rules.yml" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "single prometheus server / path match",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
  include = [ "rules.yml" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.RateCheckName + "(prom)",
				checks.SeriesCheckName + "(prom)",
				checks.VectorMatchingCheckName + "(prom)",
				checks.RangeQueryCheckName + "(prom)",
				checks.RuleDuplicateCheckName + "(prom)",
				checks.LabelsConflictCheckName + "(prom)",
			},
		},
		{
			title: "multiple prometheus servers",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
  include = [ "rules.yml" ]
}
prometheus "ignore" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "foo.+" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.RateCheckName + "(prom)",
				checks.SeriesCheckName + "(prom)",
				checks.VectorMatchingCheckName + "(prom)",
				checks.RangeQueryCheckName + "(prom)",
				checks.RuleDuplicateCheckName + "(prom)",
				checks.LabelsConflictCheckName + "(prom)",
			},
		},
		{
			title:  "single empty rule",
			config: "rule{}\n",
			path:   "rules.yml",
			rule:   newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "rule with aggregate checks",
			config: `
rule {
  aggregate ".+" {
    severity = "bug"
	keep     = ["job"]
  }
  aggregate ".+" {
    severity = "bug"
	strip    = ["instance", "rack"]
  }
}`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.AggregationCheckName + "(job:true)",
				checks.AggregationCheckName + "(instance:false)",
				checks.AggregationCheckName + "(rack:false)",
			},
		},
		{
			title: "multiple checks and disable comment",
			config: `
rule {
  aggregate ".+" {
    severity = "bug"
	keep     = ["job"]
  }
  aggregate ".+" {
    severity = "bug"
	strip    = ["instance", "rack"]
  }
}`,
			path: "rules.yml",
			rule: newRule(t, `
- record: foo
  # pint disable promql/aggregate(instance:false)
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.AggregationCheckName + "(job:true)",
				checks.AggregationCheckName + "(rack:false)",
			},
		},
		{
			title: "prometheus check without prometheus server",
			config: `
rule {
  cost {
	maxSeries = 10000
	severity  = "warning"
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "prometheus check with prometheus servers and disable comment",
			config: `
rule {
  cost {}
}
prometheus "prom1" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "rules.yml" ]
}
prometheus "prom2" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "rules.yml" ]
}  
`,
			path: "rules.yml",
			rule: newRule(t, `
# pint disable promql/series(prom1)
# pint disable query/cost(prom2)
- record: foo
  # pint disable promql/rate(prom2)
  # pint disable promql/vector_matching(prom1)
  # pint disable rule/duplicate(prom1)
  # pint disable labels/conflict(prom2)
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
				checks.RateCheckName + "(prom1)",
				checks.RangeQueryCheckName + "(prom1)",
				checks.LabelsConflictCheckName + "(prom1)",
				checks.SeriesCheckName + "(prom2)",
				checks.VectorMatchingCheckName + "(prom2)",
				checks.RangeQueryCheckName + "(prom2)",
				checks.RuleDuplicateCheckName + "(prom2)",
				checks.CostCheckName + "(prom1)",
			},
		},
		{
			title: "duplicated rules",
			config: `
rule {
  label "team" {
    severity = "bug"
    required = true
  }
}
rule {
  annotation "summary" {
    severity = "bug"
    required = true
  }
}
rule {
  label "team" {
    severity = "warning"
    required = false
  }
  annotation "summary" {
    severity = "bug"
    required = true
  }
}
rule {
  annotation "summary" {
    value    = "foo.+"
    severity = "bug"
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.LabelCheckName + "(team:true)",
				checks.AnnotationCheckName + "(summary:true)",
				checks.LabelCheckName + "(team:false)",
				checks.AnnotationCheckName + "(summary=~^foo.+$:true)",
			},
		},
		{
			title: "multiple cost checks",
			config: `
prometheus "prom1" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "rules.yml" ]
}
prometheus "prom2" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "rules.yml" ]
}
rule {
  cost {
    severity  = "info"
  }
}
rule {
  cost {
	maxSeries = 10000
	severity  = "warning"
  }
}
rule {
  cost {
    maxSeries = 20000
    severity  = "bug"
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, `
# pint disable promql/series
# pint disable promql/rate
# pint disable promql/vector_matching(prom1)
# pint disable promql/vector_matching(prom2)
# pint disable promql/range_query
# pint disable rule/duplicate
# pint disable labels/conflict
- record: foo
  # pint disable promql/fragile
  # pint disable promql/regexp
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.CostCheckName + "(prom1)",
				checks.CostCheckName + "(prom2)",
				checks.CostCheckName + "(prom1:10000)",
				checks.CostCheckName + "(prom2:10000)",
				checks.CostCheckName + "(prom1:20000)",
				checks.CostCheckName + "(prom2:20000)",
			},
		},
		{
			title: "reject rules",
			config: `
rule {
  reject "http://.+" {
    label_keys = true
    label_values = true
  }
  reject ".* +.*" {
    annotation_keys = true
    label_keys = true
  }
  reject "" {
    annotation_values = true
	severity = "bug"
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.RejectCheckName + "(key=~'^http://.+$')",
				checks.RejectCheckName + "(val=~'^http://.+$')",
				checks.RejectCheckName + "(key=~'^.* +.*$')",
				checks.RejectCheckName + "(val=~'^$')",
			},
		},
		{
			title: "rule with label match / type mismatch",
			config: `
rule {
  match {
    kind = "alerting"
    label "cluster" {
      value = "prod"
    }
  }
  label "priority" {
    severity = "bug"
    value    = "(1|2|3|4|5)"
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "rule with label match / no label",
			config: `
rule {
  match {
    kind = "alerting"
    label "cluster" {
      value = "prod"
    }
  }
  label "priority" {
    severity = "bug"
    value    = "(1|2|3|4|5)"
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "rule with label match / label mismatch",
			config: `
rule {
  match {
    kind = "alerting"
    label "cluster" {
      value = "prod"
    }
  }
  label "priority" {
    severity = "bug"
    value    = "(1|2|3|4|5)"
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  labels:\n    cluster: dev\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "rule with label match / label match",
			config: `
rule {
  match {
    kind = "alerting"
    label "cluster" {
      value = "prod"
    }
  }
  label "priority" {
    severity = "bug"
    value    = "(1|2|3|4|5)"
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  labels:\n    cluster: prod\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.LabelCheckName + "(priority:true)",
			},
		},
		{
			title: "rule with annotation match / no annotation",
			config: `
rule {
  match {
    kind = "alerting"
    annotation "cluster" {
      value = "prod"
    }
  }
  label "priority" {
    severity = "bug"
    value    = "(1|2|3|4|5)"
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "rule with annotation match / annotation mismatch",
			config: `
rule {
  match {
    kind = "alerting"
    annotation "cluster" {
      value = "prod"
    }
  }
  label "priority" {
    severity = "bug"
    value    = "(1|2|3|4|5)"
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  annotations:\n    cluster: dev\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "rule with annotation match / annotation match",
			config: `
rule {
  match {
    kind = "alerting"
    annotation "cluster" {
      value = "prod"
    }
  }
  label "priority" {
    severity = "bug"
    value    = "(1|2|3|4|5)"
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  annotations:\n    cluster: prod\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.LabelCheckName + "(priority:true)",
			},
		},
		{
			title: "checks disabled via config",
			config: `
rule {
  alerts {
	range      = "1h"
	step       = "1m"
	resolve    = "5m"
  }
}
checks {
  disabled = [
    "promql/rate",
	"promql/vector_matching",
	"promql/range_query",
	"rule/duplicate",
	"labels/conflict",
  ]
}
prometheus "prom1" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "rules.yml" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
- record: foo
  expr: sum(foo)
  # pint disable promql/series
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
				checks.AlertsCheckName + "(prom1)",
			},
		},
		{
			title: "single check enabled via config",
			config: `
rule {
  alerts {
	range      = "1h"
	step       = "1m"
	resolve    = "5m"
  }
}
checks {
  enabled = []
}
prometheus "prom1" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "rules.yml" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
- record: foo
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.RateCheckName + "(prom1)",
				checks.SeriesCheckName + "(prom1)",
				checks.VectorMatchingCheckName + "(prom1)",
				checks.RangeQueryCheckName + "(prom1)",
				checks.RuleDuplicateCheckName + "(prom1)",
				checks.LabelsConflictCheckName + "(prom1)",
				checks.AlertsCheckName + "(prom1)",
			},
		},
		{
			title: "two checks enabled via config",
			config: `
rule {
  alerts {
	range      = "1h"
	step       = "1m"
	resolve    = "5m"
  }
}
checks {
  enabled = [
    "promql/syntax",
	"alerts/count",
  ]
}
prometheus "prom1" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "rules.yml" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
- record: foo
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertsCheckName + "(prom1)",
			},
		},
		{
			title: "rule with ignore block / mismatch",
			config: `
rule {
  ignore {
    path = "foo.xml"
  }
  alerts {
	range      = "1h"
	step       = "1m"
	resolve    = "5m"
  }
}
prometheus "prom1" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "rules.yml" ]
}
checks {
  enabled = [
    "promql/syntax",
    "alerts/count",
  ]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
- alert: foo
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertsCheckName + "(prom1)",
			},
		},
		{
			title: "rule with ignore block / match",
			config: `
rule {
  ignore {
    path = "rules.yml"
  }
  alerts {
	range      = "1h"
	step       = "1m"
	resolve    = "5m"
  }
}
prometheus "prom1" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "rules.yml" ]
}
checks {
  enabled = [
    "promql/syntax",
    "alerts/count",
  ]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
- alert: foo
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
			},
		},
		{
			title: "for match / passing",
			config: `
rule {
  match {
	for = "> 15m"
  }
  annotation "summary" {
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  for: 16m\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
				checks.AnnotationCheckName + "(summary:true)",
			},
		},
		{
			title: "for match / not passing",
			config: `
rule {
  match {
	for = "> 15m"
  }
  annotation "summary" {
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  for: 14m\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "for match / recording rules / not passing",
			config: `
rule {
  match {
	for = "!= 15m"
  }
  annotation "summary" {
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "for ignore / passing",
			config: `
rule {
  ignore {
	for = "< 15m"
  }
  annotation "summary" {
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  for: 16m\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
				checks.AnnotationCheckName + "(summary:true)",
			},
		},
		{
			title: "for ignore / not passing",
			config: `
rule {
  ignore {
	for = "< 15m"
  }
  annotation "summary" {
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  for: 14m\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
		},
		{
			title: "for ignore / recording rules / passing",
			config: `
rule {
  ignore {
	for = "> 0"
  }
  annotation "summary" {
    required = true
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
				checks.AnnotationCheckName + "(summary:true)",
			},
		},
		{
			title: "link",
			config: `
rule {
  link "https?://(.+)" {
    uri = "http://localhost/$1"
	timeout = "10s"
	headers = {
		X-Auth = "xxx"
	}
	severity = "bug"
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
				checks.RuleLinkCheckName + "(^https?://(.+)$)",
			},
		},
		{
			title: "two prometheus servers / disable checks via file/disable comment",
			config: `
prometheus "prom1" {
  uri     = "http://localhost/1"
  timeout = "1s"
}
prometheus "prom2" {
  uri     = "http://localhost/2"
  timeout = "1s"
}
checks {
  disabled = [ "alerts/template" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
# Some extra comment
# pint disable promql/series
# Some extra comment
# pint disable promql/range_query
- record: foo
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName,
			},
			disabledChecks: []string{"promql/rate", "promql/vector_matching", "rule/duplicate", "labels/conflict"},
		},
		{
			title: "two prometheus servers / snoozed checks via comment",
			config: `
prometheus "prom1" {
  uri     = "http://localhost/1"
  timeout = "1s"
}
prometheus "prom2" {
  uri     = "http://localhost/2"
  timeout = "1s"
}
checks {
  disabled = [ "alerts/template", "promql/regexp" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
# pint snooze 2099-11-AB labels/conflict
# pint snooze 2099-11-28 labels/conflict won't work
# pint snooze 2099-11-28
# pint snooze 2099-11-28 promql/series(prom1)
# pint snooze 2099-11-28T10:24:18Z promql/range_query
# pint snooze 2099-11-28 rule/duplicate
# pint snooze 2099-11-28T00:00:00+00:00 promql/vector_matching
- record: foo
  expr: sum(foo)
# pint file/disable promql/vector_matching
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.FragileCheckName,
				checks.LabelsConflictCheckName + "(prom1)",
				checks.SeriesCheckName + "(prom2)",
				checks.LabelsConflictCheckName + "(prom2)",
			},
			disabledChecks: []string{"promql/rate"},
		},
		{
			title: "two prometheus servers / expired snooze",
			config: `
prometheus "prom1" {
  uri     = "http://localhost/1"
  timeout = "1s"
}
prometheus "prom2" {
  uri     = "http://localhost/2"
  timeout = "1s"
}
checks {
  disabled = [ "alerts/template", "promql/regexp" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
# pint snooze 2000-11-28 promql/series(prom1)
# pint snooze 2000-11-28T10:24:18Z promql/range_query
# pint snooze 2000-11-28 rule/duplicate
# pint snooze 2000-11-28T00:00:00+00:00 promql/vector_matching
- record: foo
  expr: sum(foo)
# pint file/disable promql/vector_matching
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.FragileCheckName,
				checks.SeriesCheckName + "(prom1)",
				checks.VectorMatchingCheckName + "(prom1)",
				checks.RangeQueryCheckName + "(prom1)",
				checks.RuleDuplicateCheckName + "(prom1)",
				checks.LabelsConflictCheckName + "(prom1)",
				checks.SeriesCheckName + "(prom2)",
				checks.VectorMatchingCheckName + "(prom2)",
				checks.RangeQueryCheckName + "(prom2)",
				checks.RuleDuplicateCheckName + "(prom2)",
				checks.LabelsConflictCheckName + "(prom2)",
			},
			disabledChecks: []string{"promql/rate"},
		},
		{
			title: "tag disables all prometheus checks",
			config: `
prometheus "prom1" {
  uri  = "http://localhost/1"
  tags = ["foo", "disable", "bar"]
}
prometheus "prom2" {
  uri  = "http://localhost/2"
  tags = []
}
prometheus "prom3" {
  uri  = "http://localhost/3"
  tags = ["foo"]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
# pint disable alerts/count(+disable)
# pint disable labels/conflict(+disable)
# pint disable promql/range_query(+disable)
# pint disable promql/regexp(+disable)
# pint disable promql/series(+disable)
# pint disable promql/rate(+disable)
# pint disable promql/vector_matching(+disable)
# pint disable rule/duplicate(+disable)
- record: foo
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.RateCheckName + "(prom2)",
				checks.SeriesCheckName + "(prom2)",
				checks.VectorMatchingCheckName + "(prom2)",
				checks.RangeQueryCheckName + "(prom2)",
				checks.RuleDuplicateCheckName + "(prom2)",
				checks.LabelsConflictCheckName + "(prom2)",
				checks.RateCheckName + "(prom3)",
				checks.SeriesCheckName + "(prom3)",
				checks.VectorMatchingCheckName + "(prom3)",
				checks.RangeQueryCheckName + "(prom3)",
				checks.RuleDuplicateCheckName + "(prom3)",
				checks.LabelsConflictCheckName + "(prom3)",
			},
		},
		{
			title: "tag snoozes all prometheus checks",
			config: `
prometheus "prom1" {
  uri  = "http://localhost/1"
  tags = ["foo", "disable", "bar"]
}
prometheus "prom2" {
  uri  = "http://localhost/2"
  tags = []
}
prometheus "prom3" {
  uri  = "http://localhost/3"
  tags = ["foo"]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
# pint snooze 2099-11-28 alerts/count(+disable)
# pint snooze 2099-11-28 labels/conflict(+disable)
# pint snooze 2099-11-28 promql/range_query(+disable)
# pint snooze 2099-11-28 promql/regexp(+disable)
# pint snooze 2099-11-28 promql/series(+disable)
# pint snooze 2099-11-28 promql/rate(+disable)
# pint snooze 2099-11-28 promql/vector_matching(+disable)
# pint snooze 2099-11-28 rule/duplicate(+disable)
- record: foo
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.FragileCheckName,
				checks.RegexpCheckName, checks.RateCheckName + "(prom2)",
				checks.SeriesCheckName + "(prom2)",
				checks.VectorMatchingCheckName + "(prom2)",
				checks.RangeQueryCheckName + "(prom2)",
				checks.RuleDuplicateCheckName + "(prom2)",
				checks.LabelsConflictCheckName + "(prom2)",
				checks.RateCheckName + "(prom3)",
				checks.SeriesCheckName + "(prom3)",
				checks.VectorMatchingCheckName + "(prom3)",
				checks.RangeQueryCheckName + "(prom3)",
				checks.RuleDuplicateCheckName + "(prom3)",
				checks.LabelsConflictCheckName + "(prom3)",
			},
		},
		
	}

	dir := t.TempDir()
	ctx := context.WithValue(context.Background(), config.CommandKey, config.LintCommand)
	for i, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			path := path.Join(dir, fmt.Sprintf("%d.hcl", i))
			if tc.config != "" {
				err := os.WriteFile(path, []byte(tc.config), 0o644)
				require.NoError(t, err)
			}

			cfg, err := config.Load(path, false)
			require.NoError(t, err)

			checks := cfg.GetChecksForRule(ctx, tc.path, tc.rule, tc.disabledChecks)
			checkNames := make([]string, 0, len(checks))
			for _, c := range checks {
				checkNames = append(checkNames, c.String())
			}
			require.Equal(t, tc.checks, checkNames)
			snaps.MatchSnapshot(t, cfg.String())
		})
	}
}

func TestConfigErrors(t *testing.T) {
	type testCaseT struct {
		config string
		err    string
	}

	testCases := []testCaseT{
		{
			config: "ci {maxCommits = -1}",
			err:    "maxCommits cannot be <= 0",
		},
		{
			config: `ci {include = [".+", ".+++"]}`,
			err:    "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `repository {
  bitbucket {
    project    = ""
    repository = ""
    timeout    = ""
	uri        = ""
  }
}`,
			err: "project cannot be empty",
		},
		{
			config: `checks { enabled = ["foo"] }`,
			err:    "unknown check name foo",
		},
		{
			config: `prometheus "prom" {
  uri     = "http://localhost"
  timeout = "abc"
}`,
			err: `not a valid duration string: "abc"`,
		},
		{
			config: `rule {
  aggregate ".+++" {}
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  annotation ".+++" {}
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  label ".+++" {}
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  reject ".+++" {}
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  alerts {
    range   = "abc"
	step    = "abc"
	resolve = "abc"
  }
}`,
			err: `not a valid duration string: "abc"`,
		},
		{
			config: `rule {
  alerts {
    range    = "1d"
	step     = "5m"
	resolve  = "5m"
	minCount = -10
  }
}`,
			err: `minCount cannot be < 0, got -10`,
		},
		{
			config: `rule {
  alerts {
    range    = "1d"
	step     = "5m"
	resolve  = "5m"
	severity = "bug"
  }
}`,
			err: `cannot set serverity to "bug" when minCount is 0`,
		},
		{
			config: `rule {
  match {
    path = ".+++"
  }
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  match {
    label ".+++" {
	  value = "bar"
    }
  }
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  match {
    label "foo" {
	  value = ".+++"
    }
  }
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  match {
    annotation ".***" {
	  value = "bar"
    }
  }
}`,
			err: "error parsing regexp: invalid nested repetition operator: `**`",
		},
		{
			config: `rule {
  match {
    annotation "foo" {
	  value = ".+++"
    }
  }
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  match {
    kind = "foo"
  }
}`,
			err: "unknown rule type: foo",
		},
		{
			config: `rule {
  ignore {
    name = ".+++"
  }
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  ignore {}
}`,
			err: "ignore block must have at least one condition",
		},
		{
			config: `rule {
  match {
	for = "!1s"
  }
}`,
			err: `not a valid duration string: "!1s"`,
		},
		{
			config: `parser {
  relaxed = ["foo", ".+", "(.+++)"]
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `check "bob" {}`,
			err:    `unknown check "bob"`,
		},
		{
			config: `check "promql/series " {}`,
			err:    `unknown check "promql/series "`,
		},
		{
			config: `check "promql/series" { ignoreMetrics = [".+++"] }`,
			err:    "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  link ".+++" {}
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  link ".*" {
	timeout = "abc"
  }
}`,
			err: `not a valid duration string: "abc"`,
		},
		{
			config: `rule {
  cost {
    severity  = "xxx"
  }
}`,
			err: "unknown severity: xxx",
		},
		{
			config: `rule {
  for {
    severity  = "xxx"
  }
}`,
			err: "unknown severity: xxx",
		},
		{
			config: `rule {
  for {
    severity  = "info"
	min       = "v"
  }
}`,
			err: `not a valid duration string: "v"`,
		},
		{
			config: `rule {
  for {
    severity  = "info"
	min       = "5m"
	max       = "v"
  }
}`,
			err: `not a valid duration string: "v"`,
		},
		{
			config: `rule {
  for {
    severity  = "info"
  }
}`,
			err: "must set either min or max option, or both",
		},
		{
			config: `owners {
  allowed = [".+++"]
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
	}

	dir := t.TempDir()
	for i, tc := range testCases {
		t.Run(tc.err, func(t *testing.T) {
			path := path.Join(dir, fmt.Sprintf("%d.hcl", i))
			if tc.config != "" {
				err := os.WriteFile(path, []byte(tc.config), 0o644)
				require.NoError(t, err)
			}

			_, err := config.Load(path, false)
			require.EqualError(t, err, tc.err, tc.config)
		})
	}
}

func TestDuplicatedPrometeusName(t *testing.T) {
	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := os.WriteFile(path, []byte(`
prometheus "prom" {
  uri     = "http://localhost:3000"
  timeout = "1s"
}
prometheus "prom" {
	uri     = "http://localhost:3001"
	timeout = "1s"
  }
`), 0o644)
	require.NoError(t, err)

	_, err = config.Load(path, true)
	require.EqualError(t, err, `prometheus server name must be unique, found two or more config blocks using "prom" name`)
}
