package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

func (cfg *Config) SetDisabledChecks(offline bool, l []string) {
	disabled := map[string]struct{}{}
	if offline {
		for _, name := range checks.OnlineChecks {
			disabled[name] = struct{}{}
		}
	}
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

func (cfg *Config) GetChecksForRule(path string, r parser.Rule) []checks.RuleChecker {
	enabled := []checks.RuleChecker{}

	if isEnabled(cfg.Checks.Enabled, cfg.Checks.Disabled, checks.SyntaxCheckName, r) {
		enabled = append(enabled, checks.NewSyntaxCheck())
	}

	if isEnabled(cfg.Checks.Enabled, cfg.Checks.Disabled, checks.AlertForCheckName, r) {
		enabled = append(enabled, checks.NewAlertsForCheck())
	}

	if isEnabled(cfg.Checks.Enabled, cfg.Checks.Disabled, checks.ComparisonCheckName, r) {
		enabled = append(enabled, checks.NewComparisonCheck())
	}

	if isEnabled(cfg.Checks.Enabled, cfg.Checks.Disabled, checks.TemplateCheckName, r) {
		enabled = append(enabled, checks.NewTemplateCheck())
	}

	proms := []*promapi.Prometheus{}
	for _, prom := range cfg.Prometheus {
		if !prom.isEnabledForPath(path) {
			continue
		}
		for _, p := range cfg.prometheusServers {
			if p.Name() == prom.Name {
				proms = append(proms, p)
			}
		}
	}

	for _, rule := range cfg.Rules {
		for _, c := range rule.resolveChecks(path, r, cfg.Checks.Enabled, cfg.Checks.Disabled, proms) {
			if r.HasComment(fmt.Sprintf("disable %s", removeRedundantSpaces(c.String()))) {
				log.Debug().
					Str("path", path).
					Str("check", c.String()).
					Msg("Check disabled by comment")
				continue
			}
			// check if rule was already enabled
			var v bool
			for _, er := range enabled {
				if er.String() == c.String() {
					v = true
					break
				}
			}
			if !v {
				enabled = append(enabled, c)
			}
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
		if rule.Match != nil {
			if err = rule.Match.validate(true); err != nil {
				return cfg, err
			}
		}
		if rule.Ignore != nil {
			if err = rule.Ignore.validate(false); err != nil {
				return cfg, err
			}
		}

		for _, aggr := range rule.Aggregate {
			if err = aggr.validate(); err != nil {
				return cfg, err
			}
		}

		for _, ann := range rule.Annotation {
			if err = ann.validate(); err != nil {
				return cfg, err
			}
		}

		for _, lab := range rule.Label {
			if err = lab.validate(); err != nil {
				return cfg, err
			}
		}

		if rule.Cost != nil {
			if err = rule.Cost.validate(); err != nil {
				return cfg, err
			}
		}

		if rule.Alerts != nil {
			if err = rule.Alerts.validate(); err != nil {
				return cfg, err
			}
		}

		for _, reject := range rule.Reject {
			if err = reject.validate(); err != nil {
				return cfg, err
			}

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

func removeRedundantSpaces(line string) string {
	return strings.Join(strings.Fields(line), " ")
}
