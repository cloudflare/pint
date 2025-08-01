package config_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

func TestMain(t *testing.M) {
	v := t.Run()
	if _, err := snaps.Clean(t, snaps.CleanOpts{Sort: true}); err != nil {
		fmt.Printf("snaps.Clean() returned an error: %s", err)
		os.Exit(100)
	}
	os.Exit(v)
}

func TestConfigLoadMissingFile(t *testing.T) {
	_, ok, err := config.Load("/foo/bar/pint.hcl", true)
	require.EqualError(t, err, "<nil>: Configuration file not found; The configuration file /foo/bar/pint.hcl does not exist.")
	require.True(t, ok)
}

func TestConfigLoadMissingFileOk(t *testing.T) {
	_, ok, err := config.Load("/foo/bar/pint.hcl", false)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestConfigLoadMergeDefaults(t *testing.T) {
	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := os.WriteFile(path, []byte("parser {}\n"), 0o644)
	require.NoError(t, err)

	cfg, ok, err := config.Load(path, true)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, cfg.CI)
	require.Equal(t, 20, cfg.CI.MaxCommits)
	require.NotNil(t, cfg.Repository)
}

func TestConfigLoadMergeDefaultsWhenMissing(t *testing.T) {
	cfg, ok, err := config.Load("xxx.hcl", false)
	require.NoError(t, err)
	require.False(t, ok)
	require.NotNil(t, cfg.CI)
	require.Equal(t, 20, cfg.CI.MaxCommits)
	require.NotNil(t, cfg.Repository)
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

	cfg, ok, err := config.Load(path, true)
	require.NoError(t, err)
	require.True(t, ok)

	gen := config.NewPrometheusGenerator(cfg, prometheus.NewRegistry())
	defer gen.Stop()
	require.NoError(t, gen.GenerateStatic())

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

	cfg, _, err := config.Load(path, true)
	require.NoError(t, err)

	gen := config.NewPrometheusGenerator(cfg, prometheus.NewRegistry())
	defer gen.Stop()
	require.NoError(t, gen.GenerateStatic())

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

	cfg, _, err := config.Load(path, true)
	require.NoError(t, err)

	gen := config.NewPrometheusGenerator(cfg, prometheus.NewRegistry())
	defer gen.Stop()
	require.NoError(t, gen.GenerateStatic())

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

	cfg, _, err := config.Load(path, true)
	require.NoError(t, err)

	gen := config.NewPrometheusGenerator(cfg, prometheus.NewRegistry())
	defer gen.Stop()
	require.NoError(t, gen.GenerateStatic())

	require.Empty(t, cfg.Checks.Disabled)

	cfg.SetDisabledChecks([]string{checks.SyntaxCheckName})
	cfg.SetDisabledChecks([]string{checks.SyntaxCheckName})
	cfg.SetDisabledChecks([]string{checks.RateCheckName})
	require.Equal(t, []string{checks.SyntaxCheckName, checks.RateCheckName}, cfg.Checks.Disabled)
}

func newRule(t *testing.T, content string) parser.Rule {
	p := parser.NewParser(false, parser.PrometheusSchema, model.UTF8Validation)
	file := p.Parse(strings.NewReader(content))
	if file.Error.Err != nil {
		t.Error(file.Error)
		t.FailNow()
	}
	for _, group := range file.Groups {
		for _, rule := range group.Rules {
			return rule
		}
	}
	return parser.Rule{}
}

func TestGetChecksForRule(t *testing.T) {
	type testCaseT struct {
		title  string
		config string
		entry  discovery.Entry
	}

	type SnapEntry struct {
		Path         discovery.Path
		FileComments []comments.Comment
		RuleComments []comments.Comment
	}

	type Snapshot struct {
		Title  string
		Config string
		Entry  SnapEntry
		Checks []string
	}

	testCases := []testCaseT{
		{
			title:  "defaults",
			config: "",
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
  disabled = [ "alerts/template", "alerts/external_labels" ]
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
# pint disable promql/counter
# pint disable promql/rate
# pint disable promql/series
# pint disable promql/vector_matching
# pint disable promql/range_query
# pint disable rule/duplicate
# pint disable labels/conflict
# pint disable alerts/absent
- record: foo
  expr: sum(foo)
`),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			},
		},
		{
			title:  "single empty rule",
			config: "rule{}\n",
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
	comment  = "this is rule comment"
    severity = "bug"
	strip    = ["instance", "rack"]
  }
}`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo
  # pint disable promql/aggregate(instance:false)
  # pint disable promql/impossible
  expr: sum(foo)
`),
			},
		},
		{
			title: "prometheus check without prometheus server",
			config: `
rule {
  cost {
	maxSeries = 10000
	comment   = "this is rule comment"
	severity  = "warning"
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
# pint disable promql/series(prom1)
# pint disable query/cost(prom2)
- record: foo
  # pint disable promql/rate(prom2)
  # pint disable promql/vector_matching(prom1)
  # pint disable rule/duplicate(prom1)
  # pint disable labels/conflict(prom2)
  # pint disable alerts/external_labels(prom2)
  # pint disable promql/counter(prom1)
  expr: sum(foo)
`),
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
	comment  = "this is rule comment"
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
	comment  = "this is rule comment"
    required = true
  }
}
rule {
	annotation "summary" {
	  token    = "\\w+"
	  value    = "foo.+"
	  severity = "bug"
	  required = true
	}
  }
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n"),
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
	comment  = "this is rule comment"
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
# pint disable promql/counter
# pint disable promql/series
# pint disable promql/rate
# pint disable promql/vector_matching(prom1)
# pint disable promql/vector_matching(prom2)
# pint disable promql/range_query
# pint disable rule/duplicate
# pint disable labels/conflict
# pint disable alerts/external_labels
# pint disable promql/impossible
- record: foo
  # pint disable promql/fragile
  # pint disable promql/regexp
  expr: sum(foo)
`),
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
	comment  = "this is rule comment"
    annotation_keys = true
    label_keys = true
  }
  reject "" {
	comment  = "this is rule comment"
    annotation_values = true
	severity = "bug"
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  labels:\n    cluster: dev\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  labels:\n    cluster: prod\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  annotations:\n    cluster: dev\n"),
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
	comment  = "this is rule comment"
    severity = "bug"
	token    = "\\w+"
    value    = "(1|2|3|4|5)"
    required = true
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  annotations:\n    cluster: prod\n"),
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
	"promql/counter",
    "promql/rate",
	"promql/vector_matching",
	"promql/range_query",
	"rule/duplicate",
	"labels/conflict",
	"alerts/absent",
  ]
}
prometheus "prom1" {
  uri     = "http://localhost"
  timeout = "1s"
  include =[ "rules.yml" ]
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo
  expr: sum(foo)
  # pint disable promql/series
`),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo
  expr: sum(foo)
`),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo
  expr: sum(foo)
`),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- alert: foo
  expr: sum(foo)
`),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- alert: foo
  expr: sum(foo)
`),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  for: 16m\n"),
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
	comment  = "this is rule comment"
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  for: 14m\n"),
			},
		},
		{
			title: "for match / passing",
			config: `
rule {
  match {
	keep_firing_for = "> 15m"
  }
  annotation "summary" {
    required = true
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  keep_firing_for: 16m\n"),
			},
		},
		{
			title: "for match / passing",
			config: `
rule {
  match {
	keep_firing_for = "> 15m"
  }
  annotation "summary" {
    required = true
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  for: 16m\n"),
			},
		},
		{
			title: "for match / passing",
			config: `
rule {
  match {
	keep_firing_for = "> 15m"
  }
  annotation "summary" {
    required = true
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  keep_firing_for: 14m\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  for: 16m\n"),
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
	comment  = "this is rule comment"
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- alert: foo\n  expr: sum(foo)\n  for: 14m\n"),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
	comment  = "this is rule comment"
	severity = "bug"
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			},
		},
		{
			title: "name",
			config: `
rule {
  name "total:.+" {}
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
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
  disabled = [ "alerts/template", "alerts/external_labels", "alerts/absent" ]
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
# Some extra comment
# pint disable promql/series
# Some extra comment
# pint disable promql/range_query
- record: foo
  expr: sum(foo)
`),
				DisabledChecks: []string{"promql/rate", "promql/vector_matching", "rule/duplicate", "labels/conflict", "promql/counter"},
			},
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
# pint snooze 2099-11-AB labels/conflict
# pint snooze 2099-11-28 labels/conflict won't work
# pint snooze 2099-11-28
# pint snooze 2099-11-28 promql/series(prom1)
# pint snooze 2099-11-28T10:24:18Z promql/range_query
# pint snooze 2099-11-28 rule/duplicate
# pint snooze 2099-11-28T00:00:00+00:00 promql/vector_matching
# pint snooze 2099-11-28 promql/counter
- record: foo # pint snooze 2099-11-28 alerts/absent
  expr: sum(foo)
# pint file/disable promql/vector_matching
`),
				DisabledChecks: []string{"promql/rate"},
			},
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
# pint snooze 2000-11-28 promql/series(prom1)
# pint snooze 2000-11-28T10:24:18Z promql/range_query
# pint snooze 2000-11-28 rule/duplicate
# pint snooze 2000-11-28T00:00:00+00:00 promql/vector_matching
- record: foo
  expr: sum(foo)
# pint file/disable promql/vector_matching
`),
				DisabledChecks: []string{"promql/rate"},
			},
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
# pint disable alerts/count(+disable)
# pint disable alerts/external_labels(+disable)
# pint disable labels/conflict(+disable)
# pint disable promql/counter(+disable)
# pint disable promql/range_query(+disable)
# pint disable promql/regexp(+disable)
# pint disable promql/series(+disable)
# pint disable promql/rate(+disable)
# pint disable promql/vector_matching(+disable)
# pint disable rule/duplicate(+disable)
# pint disable alerts/absent(+disable)
- record: foo
  expr: sum(foo)
`),
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
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
# pint snooze 2099-11-28 alerts/count(+disable)
# pint snooze 2099-11-28 alerts/external_labels(+disable)
# pint snooze 2099-11-28 labels/conflict(+disable)
# pint snooze 2099-11-28 promql/range_query(+disable)
# pint snooze 2099-11-28 promql/regexp(+disable)
# pint snooze 2099-11-28 promql/counter(+disable)
# pint snooze 2099-11-28 promql/series(+disable)
# pint snooze 2099-11-28 promql/rate(+disable)
# pint snooze 2099-11-28 promql/vector_matching(+disable)
# pint snooze 2099-11-28 rule/duplicate(+disable)
# pint snooze 2099-11-28 alerts/absent(+disable)
- record: foo
  expr: sum(foo)
`),
			},
		},
		{
			title: "alerts/count defaults",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
}

rule {
  alerts {
    range    = "1d"
    step     = "1m"
    resolve  = "5m"
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			},
		},
		{
			title: "alerts/count full",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
}

rule {
  alerts {
    range    = "1d"
    step     = "1m"
    resolve  = "5m"
	minCount = 100
	comment  = "this is rule comment"
	severity = "bug"
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			},
		},
		{
			title: "custom range_query",
			config: `rule {
  range_query {
    max      = "1h"
	severity = "bug"
  }
}`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			},
		},
		{
			title: "state mismatch",
			config: `
rule {
  match {
    state = ["renamed"]
  }
  aggregate ".+" {
    severity = "bug"
	keep     = ["job"]
  }
}
rule {
  ignore {
    state = ["modified"]
  }
  aggregate ".+" {
    severity = "bug"
	strip    = ["instance", "rack"]
  }
}`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo
  expr: sum(foo)
`),
			},
		},
		{
			title: "state match",
			config: `
rule {
  match {
    state = ["renamed"]
  }
  aggregate ".+" {
    severity = "bug"
	keep     = ["job"]
  }
}
rule {
  ignore {
    state = ["modified"]
  }
  aggregate ".+" {
    severity = "bug"
	strip    = ["instance", "rack"]
  }
}`,
			entry: discovery.Entry{
				State: discovery.Moved,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo
  expr: sum(foo)
`),
			},
		},
		{
			title: "check disabled globally but enabled via rule{}",
			config: `
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
}
checks {
  disabled = [ "alerts/template", "alerts/external_labels", "rule/duplicate", "alerts/absent", "promql/series", "promql/vector_matching" ]
}
rule {
  disable = [ "rule/duplicate" ]
}
rule {
  match { kind = "alerting" }
  disable = [ "promql/series" ]
}
rule {
  enable = [ "promql/series" ]
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo
  expr: sum(foo)
`),
				DisabledChecks: []string{"promql/rate", "promql/range_query"},
			},
		},
		{
			title: "check enabled globally but disabled via rule{}",
			config: `
rule {
  match {
    kind = "recording"
  }
  disable = ["rule/duplicate"]
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo
  expr: sum(foo)
`),
				DisabledChecks: []string{"alerts/template", "alerts/external_labels", "alerts/absent"},
			},
		},
		{
			title: "two prometheus servers / check disable via rule {}",
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
rule {
  match {
    path = "rules.yml"
  }
  disable = ["promql/series", "promql/range_query", "rule/duplicate", "promql/vector_matching", "promql/counter"]
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo # pint snooze 2099-11-28 alerts/absent
  expr: sum(foo)
# pint file/disable promql/vector_matching
`),
				DisabledChecks: []string{"promql/rate"},
			},
		},
		{
			title: "reject rules",
			config: `
rule {
  match {
    kind = "recording"
  }
  report {
    comment  = "You cannot add any recording rules to this Prometheus server."
    severity = "bug"
  }
}
`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, "- record: foo\n  expr: sum(foo)\n"),
			},
		},
		{
			title: "multiple checks and disable comment / locked rule",
			config: `
rule {
  locked = false
  aggregate ".+" {
    severity = "bug"
	keep     = ["job"]
  }
}
rule {
  locked = true
  aggregate ".+" {
	comment  = "this is rule comment"
    severity = "bug"
	strip    = ["instance", "rack"]
  }
}`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo
  # pint disable promql/aggregate
  expr: sum(foo)
`),
			},
		},
		{
			title: "multiple checks and snooze comment / locked rule",
			config: `
rule {
  locked = false
  aggregate ".+" {
    severity = "bug"
	keep     = ["job"]
  }
}
rule {
  locked = true
  aggregate ".+" {
	comment  = "this is rule comment"
    severity = "bug"
	strip    = ["instance", "rack"]
  }
}`,
			entry: discovery.Entry{
				State: discovery.Modified,
				Path: discovery.Path{
					Name:          "rules.yml",
					SymlinkTarget: "rules.yml",
				},
				Rule: newRule(t, `
- record: foo
  # pint snooze 2099-11-28 promql/aggregate
  expr: sum(foo)
`),
			},
		},
	}

	dir := t.TempDir()
	ctx := context.WithValue(t.Context(), config.CommandKey, config.LintCommand)
	for i, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			path := path.Join(dir, fmt.Sprintf("%d.hcl", i))
			if tc.config != "" {
				err := os.WriteFile(path, []byte(tc.config), 0o644)
				require.NoError(t, err)
			}

			cfg, _, err := config.Load(path, false)
			require.NoError(t, err)

			gen := config.NewPrometheusGenerator(cfg, prometheus.NewRegistry())
			defer gen.Stop()
			require.NoError(t, gen.GenerateStatic())

			checks := cfg.GetChecksForEntry(ctx, gen, tc.entry)
			checkNames := make([]string, 0, len(checks))
			for _, c := range checks {
				checkNames = append(checkNames, c.String())
			}

			var fileComments []comments.Comment
			if tc.entry.File != nil {
				fileComments = tc.entry.File.Comments
			}
			d, err := yaml.Marshal(Snapshot{
				Title:  tc.title,
				Config: cfg.String(),
				Entry: SnapEntry{
					Path:         tc.entry.Path,
					FileComments: fileComments,
					RuleComments: tc.entry.Rule.Comments,
				},
				Checks: checkNames,
			})
			require.NoError(t, err)
			snaps.WithConfig(snaps.Filename(fmt.Sprintf("%03d", i+1))).MatchSnapshot(t, string(d))
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
			config: `parser {include = [".+", ".+++"]}`,
			err:    "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `parser {schema = "foo"}`,
			err:    "unsupported parser schema: foo",
		},
		{
			config: `parser {names = "foo"}`,
			err:    "unsupported parser names: foo",
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
			config: `check "promql/series" {
  ignoreLabelsValue = {
    "foo bar" = [ "abc" ]
  }
}`,
			err: `"foo bar" is not a valid PromQL metric selector: 1:5: parse error: unexpected identifier "bar"`,
		},
		{
			config: `check "promql/series" {
  ignoreLabelsValue = {
    "foo{" = [ "abc" ]
  }
}`,
			err: `"foo{" is not a valid PromQL metric selector: 1:5: parse error: unexpected end of input inside braces`,
		},
		{
			config: `check "promql/series" { lookbackRange = "1x" }`,
			err:    `unknown unit "x" in duration "1x"`,
		},
		{
			config: `check "promql/series" { lookbackStep = "1x" }`,
			err:    `unknown unit "x" in duration "1x"`,
		},
		{
			config: `check "promql/series" { fallbackTimeout = "1x" }`,
			err:    `unknown unit "x" in duration "1x"`,
		},
		{
			config: `check "promql/series" {
  ignoreMatchingElsewhere = ["a b c"]
}`,
			err: `"a b c" is not a valid PromQL metric selector: 1:3: parse error: unexpected identifier "b"`,
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
    severity  = "xxx"
  }
}`,
			err: "unknown severity: xxx",
		},
		{
			config: `rule {
  keep_firing_for {
    severity  = "info"
	min       = "v"
  }
}`,
			err: `not a valid duration string: "v"`,
		},
		{
			config: `rule {
  keep_firing_for {
    severity  = "info"
	min       = "5m"
	max       = "v"
  }
}`,
			err: `not a valid duration string: "v"`,
		},
		{
			config: `rule {
  keep_firing_for {
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
		{
			config: `discovery {
  filepath {
	directory = ""
	match = ""
  }
}`,
			err: "prometheusQuery discovery requires at least one template",
		},
		{
			config: `rule {
  name "....+++" {}
}`,
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			config: `rule {
  name "xxx" {
    severity  = "xxx"
  }
}`,
			err: "unknown severity: xxx",
		},
		{
			config: `rule {
  range_query {
	max = "abc"
  }
}`,
			err: `not a valid duration string: "abc"`,
		},
		{
			config: `rule {
  match {
    state = ["added", "foo"]
  }
}`,
			err: "unknown rule state: foo",
		},
		{
			config: `rule {
  enable = ["bob"]
}`,
			err: "unknown check name bob",
		},
		{
			config: `rule {
  disable = ["bob"]
}`,
			err: "unknown check name bob",
		},
		{
			config: `rule {
  report {
    comment = ""
	severity = "warning"
  }
}`,
			err: "report comment cannot be empty",
		},
		{
			config: `rule {
  report {
    comment = "foo"
	severity = "xxx"
  }
}`,
			err: "unknown severity: xxx",
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

			_, _, err := config.Load(path, false)
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

	_, _, err = config.Load(path, true)
	require.EqualError(t, err, `prometheus server name must be unique, found two or more config blocks using "prom" name`)
}
