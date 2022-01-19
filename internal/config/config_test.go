package config_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/parser"
)

func TestMain(t *testing.M) {
	v := t.Run()
	snaps.Clean()
	os.Exit(v)
}

func TestConfigLoadMissingFile(t *testing.T) {
	assert := assert.New(t)

	_, err := config.Load("/foo/bar/pint.hcl", true)
	assert.EqualError(err, "<nil>: Configuration file not found; The configuration file /foo/bar/pint.hcl does not exist.")
}

func TestConfigLoadMissingFileOk(t *testing.T) {
	assert := assert.New(t)

	_, err := config.Load("/foo/bar/pint.hcl", false)
	assert.Nil(err)
}

func TestDisableOnlineChecksWithPrometheus(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := ioutil.WriteFile(path, []byte(`
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
}
`), 0o644)
	assert.NoError(err)

	cfg, err := config.Load(path, true)
	assert.NoError(err)
	assert.Empty(cfg.Checks.Disabled)

	cfg.DisableOnlineChecks()
	for _, c := range checks.OnlineChecks {
		assert.Contains(cfg.Checks.Disabled, c)
	}
}

func TestDisableOnlineChecksWithoutPrometheus(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := ioutil.WriteFile(path, []byte(``), 0o644)
	assert.NoError(err)

	cfg, err := config.Load(path, true)
	assert.NoError(err)
	assert.Empty(cfg.Checks.Disabled)

	cfg.DisableOnlineChecks()
	for _, c := range checks.OnlineChecks {
		assert.Contains(cfg.Checks.Disabled, c)
	}
}

func TestDisableOnlineChecksAfterSetDisabledChecks(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := ioutil.WriteFile(path, []byte(`
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
}
`), 0o644)
	assert.NoError(err)

	cfg, err := config.Load(path, true)
	assert.NoError(err)
	assert.Empty(cfg.Checks.Disabled)

	cfg.SetDisabledChecks([]string{checks.SyntaxCheckName})
	assert.Contains(cfg.Checks.Disabled, checks.SyntaxCheckName)

	cfg.SetDisabledChecks([]string{checks.RateCheckName})
	assert.Contains(cfg.Checks.Disabled, checks.RateCheckName)

	cfg.DisableOnlineChecks()
	for _, c := range checks.OnlineChecks {
		assert.Contains(cfg.Checks.Disabled, c)
	}
}

func TestSetDisabledChecks(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := ioutil.WriteFile(path, []byte(``), 0o644)
	assert.NoError(err)

	cfg, err := config.Load(path, true)
	assert.NoError(err)
	assert.Empty(cfg.Checks.Disabled)

	cfg.SetDisabledChecks([]string{checks.SyntaxCheckName})
	cfg.SetDisabledChecks([]string{checks.SyntaxCheckName})
	cfg.SetDisabledChecks([]string{checks.RateCheckName})
	assert.Equal([]string{checks.SyntaxCheckName, checks.RateCheckName}, cfg.Checks.Disabled)
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
		title  string
		config string
		path   string
		rule   parser.Rule
		checks []string
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
				checks.RateCheckName + "(prom)",
				checks.SeriesCheckName + "(prom)",
				checks.VectorMatchingCheckName + "(prom)",
			},
		},
		{
			title: "single prometheus server / path mismatch",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
  paths   = [ "foo.yml" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
			},
		},
		{
			title: "single prometheus server / path match",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
  paths   = [ "rules.yml" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.RateCheckName + "(prom)",
				checks.SeriesCheckName + "(prom)",
				checks.VectorMatchingCheckName + "(prom)",
			},
		},
		{
			title: "multiple prometheus servers",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
  paths   = [ "rules.yml" ]
}
prometheus "ignore" {
  uri     = "http://localhost"
  timeout = "1s"
  paths   = [ "foo.+" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.RateCheckName + "(prom)",
				checks.SeriesCheckName + "(prom)",
				checks.VectorMatchingCheckName + "(prom)",
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
				checks.AggregationCheckName + "(job:true)",
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
				checks.AggregationCheckName + "(job:true)",
				checks.AggregationCheckName + "(rack:false)",
			},
		},
		{
			title: "prometheus check without prometheus server",
			config: `
rule {
  cost {
    bytesPerSample = 4096
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
			},
		},
		{
			title: "prometheus check with prometheus servers and disable comment",
			config: `
rule {
  cost {
    bytesPerSample = 4096
  }
}
prometheus "prom1" {
  uri     = "http://localhost"
  timeout = "1s"
  paths   = [ "rules.yml" ]
}
prometheus "prom2" {
  uri     = "http://localhost"
  timeout = "1s"
  paths   = [ "rules.yml" ]
}  
`,
			path: "rules.yml",
			rule: newRule(t, `
# pint disable query/series(prom1)
# pint disable query/cost(prom2)
- record: foo
  # pint disable promql/rate(prom2)
  # pint disable promql/vector_matching(prom1)
  expr: sum(foo)
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
				checks.RateCheckName + "(prom1)",
				checks.SeriesCheckName + "(prom2)",
				checks.VectorMatchingCheckName + "(prom2)",
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
				checks.LabelCheckName + "(team:true)",
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
  paths   = [ "rules.yml" ]
}
prometheus "prom2" {
  uri     = "http://localhost"
  timeout = "1s"
  paths   = [ "rules.yml" ]
}
rule {
  cost {
    bytesPerSample = 4096
    severity  = "info"
  }
}
rule {
  cost {
    bytesPerSample = 4096
	maxSeries = 10000
	severity  = "warning"
  }
}
rule {
  cost {
    bytesPerSample = 4096
    maxSeries = 20000
    severity  = "bug"
  }
}
`,
			path: "rules.yml",
			rule: newRule(t, `
# pint disable query/series
# pint disable promql/rate
# pint disable promql/vector_matching(prom1)
# pint disable promql/vector_matching(prom2)
- record: foo
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
				checks.RejectCheckName + "(key=~'^http://.+$')",
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
				checks.LabelCheckName + "(priority:true)",
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
				checks.LabelCheckName + "(priority:true)",
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
  ]
}
prometheus "prom1" {
  uri     = "http://localhost"
  timeout = "1s"
  paths   = [ "rules.yml" ]
}
`,
			path: "rules.yml",
			rule: newRule(t, `
- record: foo
  expr: sum(foo)
  # pint disable query/series
`),
			checks: []string{
				checks.SyntaxCheckName,
				checks.AlertForCheckName,
				checks.ComparisonCheckName,
				checks.TemplateCheckName,
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
  paths   = [ "rules.yml" ]
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
				checks.RateCheckName + "(prom1)",
				checks.SeriesCheckName + "(prom1)",
				checks.VectorMatchingCheckName + "(prom1)",
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
  paths   = [ "rules.yml" ]
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
  paths   = [ "rules.yml" ]
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
  paths   = [ "rules.yml" ]
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
	}

	dir := t.TempDir()
	for i, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			assert := assert.New(t)

			path := path.Join(dir, fmt.Sprintf("%d.hcl", i))
			if tc.config != "" {
				err := ioutil.WriteFile(path, []byte(tc.config), 0o644)
				assert.NoError(err)
			}

			cfg, err := config.Load(path, false)
			assert.NoError(err)

			checks := cfg.GetChecksForRule(tc.path, tc.rule)
			checkNames := make([]string, 0, len(checks))
			for _, c := range checks {
				checkNames = append(checkNames, c.String())
			}
			assert.Equal(tc.checks, checkNames)
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
			err: "empty duration string",
		},
		{
			config: `repository {
  github {
    baseuri = ""
    timeout = ""
    owner   = ""
    repo    = ""
  }
}`,
			err: "empty duration string",
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
  cost {
    bytesPerSample = -34343
  }
}`,
			err: "bytesPerSample value must be >= 0",
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
	}

	dir := t.TempDir()
	for i, tc := range testCases {
		t.Run(tc.err, func(t *testing.T) {
			assert := assert.New(t)

			path := path.Join(dir, fmt.Sprintf("%d.hcl", i))
			if tc.config != "" {
				err := ioutil.WriteFile(path, []byte(tc.config), 0o644)
				assert.NoError(err)
			}

			_, err := config.Load(path, false)
			assert.EqualError(err, tc.err, tc.config)
		})
	}
}
