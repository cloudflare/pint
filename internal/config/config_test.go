package config_test

import (
	"fmt"
	"io/ioutil"
	"path"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/stretchr/testify/assert"
)

func TestDisableOnlineChecksWithPrometheus(t *testing.T) {
	assert := assert.New(t)

	dir := t.TempDir()
	path := path.Join(dir, "config.hcl")
	err := ioutil.WriteFile(path, []byte(`
prometheus "prom" {
  uri     = "http://localhost"
  timeout = "1s"
}
`), 0644)
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
	err := ioutil.WriteFile(path, []byte(``), 0644)
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
`), 0644)
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
	err := ioutil.WriteFile(path, []byte(``), 0644)
	assert.NoError(err)

	cfg, err := config.Load(path, true)
	assert.NoError(err)
	assert.Empty(cfg.Checks.Disabled)

	cfg.SetDisabledChecks([]string{checks.SyntaxCheckName})
	cfg.SetDisabledChecks([]string{checks.SyntaxCheckName})
	cfg.SetDisabledChecks([]string{checks.RateCheckName})
	assert.Equal(cfg.Checks.Disabled, []string{checks.SyntaxCheckName, checks.RateCheckName})
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
	assert := assert.New(t)

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
	}

	dir := t.TempDir()
	for i, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			path := path.Join(dir, fmt.Sprintf("%d.hcl", i))
			err := ioutil.WriteFile(path, []byte(tc.config), 0644)
			assert.NoError(err)

			cfg, err := config.Load(path, true)
			assert.NoError(err)

			checks := cfg.GetChecksForRule(tc.path, tc.rule)
			checkNames := make([]string, 0, len(checks))
			for _, c := range checks {
				checkNames = append(checkNames, c.String())
			}
			assert.Equal(checkNames, tc.checks)
		})
	}
}
