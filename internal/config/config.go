package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"github.com/zclconf/go-cty/cty"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/prometheus/common/model"
)

type Config struct {
	CI         *CI                `hcl:"ci,block" json:"ci,omitempty"`
	Parser     *Parser            `hcl:"parser,block" json:"parser,omitempty"`
	Repository *Repository        `hcl:"repository,block" json:"repository,omitempty"`
	Discovery  *Discovery         `hcl:"discovery,block" json:"discovery,omitempty"`
	Checks     *Checks            `hcl:"checks,block" json:"checks,omitempty"`
	Owners     *Owners            `hcl:"owners,block" json:"owners,omitempty"`
	Prometheus []PrometheusConfig `hcl:"prometheus,block" json:"prometheus,omitempty"`
	Check      []Check            `hcl:"check,block" json:"check,omitempty"`
	Rules      []Rule             `hcl:"rule,block" json:"rules,omitempty"`
}

func (cfg *Config) DisableOnlineChecks() {
	for _, name := range checks.OnlineChecks {
		var found bool
		for _, n := range cfg.Checks.Disabled {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			cfg.Checks.Disabled = append(cfg.Checks.Disabled, name)
		}
	}
}

func (cfg *Config) SetDisabledChecks(l []string) {
	disabled := map[string]struct{}{}
	for _, s := range l {
		// add raw string: promql/series(prom)
		disabled[s] = struct{}{}
		// find any check name that matches string as regexp
		re := strictRegex(s)
		for _, name := range checks.CheckNames {
			if re.MatchString(name) {
				disabled[name] = struct{}{}
			}
		}
	}
	for name := range disabled {
		var found bool
		for _, c := range cfg.Checks.Disabled {
			if c == name {
				found = true
				break
			}
		}
		if !found {
			cfg.Checks.Disabled = append(cfg.Checks.Disabled, name)
		}
	}
}

func (cfg Config) String() string {
	content, _ := json.MarshalIndent(cfg, "", "  ")
	return string(content)
}

func (cfg *Config) GetChecksForRule(ctx context.Context, gen *PrometheusGenerator, entry discovery.Entry, disabledChecks []string) []checks.RuleChecker {
	enabled := []checks.RuleChecker{}

	allChecks := []checkMeta{
		{
			name:  checks.SyntaxCheckName,
			check: checks.NewSyntaxCheck(),
		},
		{
			name:  checks.AlertForCheckName,
			check: checks.NewAlertsForCheck(),
		},
		{
			name:  checks.ComparisonCheckName,
			check: checks.NewComparisonCheck(),
		},
		{
			name:  checks.TemplateCheckName,
			check: checks.NewTemplateCheck(),
		},
		{
			name:  checks.FragileCheckName,
			check: checks.NewFragileCheck(),
		},
		{
			name:  checks.RegexpCheckName,
			check: checks.NewRegexpCheck(),
		},
		{
			name:  checks.RuleDependencyCheckName,
			check: checks.NewRuleDependencyCheck(),
		},
	}

	proms := gen.ServersForPath(entry.Path.Name)

	for _, p := range proms {
		allChecks = append(allChecks, checkMeta{
			name:  checks.RateCheckName,
			check: checks.NewRateCheck(p),
			tags:  p.Tags(),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.SeriesCheckName,
			check: checks.NewSeriesCheck(p),
			tags:  p.Tags(),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.VectorMatchingCheckName,
			check: checks.NewVectorMatchingCheck(p),
			tags:  p.Tags(),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.RangeQueryCheckName,
			check: checks.NewRangeQueryCheck(p, 0, "", checks.Warning),
			tags:  p.Tags(),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.RuleDuplicateCheckName,
			check: checks.NewRuleDuplicateCheck(p),
			tags:  p.Tags(),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.LabelsConflictCheckName,
			check: checks.NewLabelsConflictCheck(p),
			tags:  p.Tags(),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.AlertsExternalLabelsCheckName,
			check: checks.NewAlertsExternalLabelsCheck(p),
			tags:  p.Tags(),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.CounterCheckName,
			check: checks.NewCounterCheck(p),
			tags:  p.Tags(),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.AlertsAbsentCheckName,
			check: checks.NewAlertsAbsentCheck(p),
			tags:  p.Tags(),
		})
	}

	for _, rule := range cfg.Rules {
		allChecks = append(allChecks, rule.resolveChecks(ctx, entry, proms)...)
	}

	for _, cm := range allChecks {
		// Entry state is not what the check is for.
		if !slices.Contains(cm.check.Meta().States, entry.State) {
			continue
		}

		// check if check is disabled for specific rule
		if !isEnabled(cfg.Checks.Enabled, disabledChecks, entry.Rule, cm.name, cm.check, cm.tags) {
			continue
		}

		// check if rule was disabled
		if !isEnabled(cfg.Checks.Enabled, cfg.Checks.Disabled, entry.Rule, cm.name, cm.check, cm.tags) {
			continue
		}
		// check if rule was already enabled
		var v bool
		for _, er := range enabled {
			if er.String() == cm.check.String() {
				v = true
				break
			}
		}
		if !v {
			enabled = append(enabled, cm.check)
		}
	}

	el := []string{}
	for _, e := range enabled {
		el = append(el, fmt.Sprintf("%v", e))
	}
	slog.Debug("Configured checks for rule",
		slog.Any("enabled", el),
		slog.String("path", entry.Path.Name),
		slog.String("rule", entry.Rule.Name()),
	)

	return enabled
}

func getContext() *hcl.EvalContext {
	vars := map[string]cty.Value{}
	for _, e := range os.Environ() {
		if k, v, ok := strings.Cut(e, "="); ok {
			vars["ENV_"+k] = cty.StringVal(v)
		}
	}
	return &hcl.EvalContext{Variables: vars}
}

func Load(path string, failOnMissing bool) (cfg Config, fromFile bool, err error) {
	cfg = Config{
		CI: &CI{
			MaxCommits: 20,
			BaseBranch: "master",
		},
		Parser: &Parser{},
		Checks: &Checks{
			Enabled:  checks.CheckNames,
			Disabled: []string{},
		},
		Rules: []Rule{},
		Owners: &Owners{
			Allowed: []string{},
		},
	}

	if _, err = os.Stat(path); err == nil || failOnMissing {
		fromFile = true
		slog.Info("Loading configuration file", slog.String("path", path))
		ectx := getContext()
		err = hclsimple.DecodeFile(path, ectx, &cfg)
		if err != nil {
			return cfg, fromFile, err
		}
	}

	if cfg.CI != nil {
		if err = cfg.CI.validate(); err != nil {
			return cfg, fromFile, err
		}
	}

	if cfg.Owners != nil {
		if err = cfg.Owners.validate(); err != nil {
			return cfg, fromFile, err
		}
	}

	if cfg.Parser != nil {
		if err = cfg.Parser.validate(); err != nil {
			return cfg, fromFile, err
		}
	}

	if cfg.Repository != nil {
		if err = cfg.Repository.validate(); err != nil {
			return cfg, fromFile, err
		}
	}

	if cfg.Checks != nil {
		if err = cfg.Checks.validate(); err != nil {
			return cfg, fromFile, err
		}
	}

	for _, chk := range cfg.Check {
		if err = chk.validate(); err != nil {
			return cfg, fromFile, err
		}
	}

	promNames := make([]string, 0, len(cfg.Prometheus))
	for i, prom := range cfg.Prometheus {
		if err = prom.validate(); err != nil {
			return cfg, fromFile, err
		}

		if slices.Contains(promNames, prom.Name) {
			return cfg, fromFile, fmt.Errorf("prometheus server name must be unique, found two or more config blocks using %q name", prom.Name)
		}
		promNames = append(promNames, prom.Name)

		cfg.Prometheus[i].applyDefaults()

		if _, err = prom.TLS.toHTTPConfig(); err != nil {
			return cfg, fromFile, fmt.Errorf("invalid prometheus TLS configuration: %w", err)
		}
	}

	if cfg.Discovery != nil {
		if err = cfg.Discovery.validate(); err != nil {
			return cfg, fromFile, err
		}
	}

	for _, rule := range cfg.Rules {
		if err = rule.validate(); err != nil {
			return cfg, fromFile, err
		}
	}

	return cfg, fromFile, nil
}

func parseDuration(d string) (time.Duration, error) {
	mdur, err := model.ParseDuration(d)
	if err != nil {
		return 0, err
	}
	return time.Duration(mdur), nil
}

type checkMeta struct {
	name  string
	check checks.RuleChecker
	tags  []string
}
