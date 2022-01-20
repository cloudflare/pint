package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

type Config struct {
	CI                *CI                `hcl:"ci,block" json:"ci,omitempty"`
	Repository        *Repository        `hcl:"repository,block" json:"repository,omitempty"`
	Prometheus        []PrometheusConfig `hcl:"prometheus,block" json:"prometheus,omitempty"`
	Checks            *Checks            `hcl:"checks,block" json:"checks,omitempty"`
	Rules             []Rule             `hcl:"rule,block" json:"rules,omitempty"`
	prometheusServers []*promapi.Prometheus
}

func (cfg *Config) ClearCache() {
	for _, prom := range cfg.prometheusServers {
		prom.ClearCache()
	}
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

func (cfg *Config) GetChecksForRule(ctx context.Context, path string, r parser.Rule) []checks.RuleChecker {
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
	}

	proms := []*promapi.Prometheus{}
	for _, prom := range cfg.Prometheus {
		if !prom.isEnabledForPath(path) {
			continue
		}
		for _, p := range cfg.prometheusServers {
			if p.Name() == prom.Name {
				proms = append(proms, p)
				break
			}
		}
	}

	for _, p := range proms {
		allChecks = append(allChecks, checkMeta{
			name:  checks.RateCheckName,
			check: checks.NewRateCheck(p),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.SeriesCheckName,
			check: checks.NewSeriesCheck(p),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.VectorMatchingCheckName,
			check: checks.NewVectorMatchingCheck(p),
		})
	}

	for _, rule := range cfg.Rules {
		allChecks = append(allChecks, rule.resolveChecks(ctx, path, r, cfg.Checks.Enabled, cfg.Checks.Disabled, proms)...)
	}

	for _, cm := range allChecks {
		// check if rule was disabled
		if !isEnabled(cfg.Checks.Enabled, cfg.Checks.Disabled, r, cm.name, cm.check) {
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
	name := "unknown"
	if r.AlertingRule != nil {
		name = r.AlertingRule.Alert.Value.Value
	} else if r.RecordingRule != nil {
		name = r.RecordingRule.Record.Value.Value
	}
	log.Debug().Strs("enabled", el).Str("path", path).Str("rule", name).Msg("Configured checks for rule")

	return enabled
}

func Load(path string, failOnMissing bool) (cfg Config, err error) {
	cfg = Config{
		CI: &CI{
			MaxCommits: 20,
			BaseBranch: "master",
		},
		Checks: &Checks{
			Enabled:  checks.CheckNames,
			Disabled: []string{},
		},
		Rules: []Rule{},
	}

	if _, err := os.Stat(path); err == nil || failOnMissing {
		log.Info().Str("path", path).Msg("Loading configuration file")
		err = hclsimple.DecodeFile(path, nil, &cfg)
		if err != nil {
			return cfg, err
		}
	}

	if cfg.CI != nil {
		if err = cfg.CI.validate(); err != nil {
			return cfg, err
		}
	}

	if cfg.Repository != nil && cfg.Repository.BitBucket != nil {
		if err = cfg.Repository.BitBucket.validate(); err != nil {
			return cfg, err
		}
	}

	if cfg.Repository != nil && cfg.Repository.GitHub != nil {
		if err = cfg.Repository.GitHub.validate(); err != nil {
			return cfg, err
		}
	}

	if cfg.Checks != nil {
		if err = cfg.Checks.validate(); err != nil {
			return cfg, err
		}
	}

	for _, prom := range cfg.Prometheus {
		if err = prom.validate(); err != nil {
			return cfg, err
		}
		timeout, _ := parseDuration(prom.Timeout)
		cfg.prometheusServers = append(cfg.prometheusServers, promapi.NewPrometheus(prom.Name, prom.URI, timeout))
	}

	for _, rule := range cfg.Rules {
		if err = rule.validate(); err != nil {
			return cfg, err
		}
	}

	return cfg, nil
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
}
