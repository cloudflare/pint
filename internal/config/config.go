package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

type Config struct {
	CI         *CI                `hcl:"ci,block"`
	Repository *Repository        `hcl:"repository,block"`
	Prometheus []PrometheusConfig `hcl:"prometheus,block"`
	Checks     *Checks            `hcl:"checks,block"`
	Rules      []Rule             `hcl:"rule,block"`
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

func (cfg Config) GetChecksForRule(path string, r parser.Rule) []checks.RuleChecker {
	enabled := []checks.RuleChecker{}

	if isEnabled(cfg.Checks.Enabled, cfg.Checks.Disabled, checks.SyntaxCheckName, r) {
		enabled = append(enabled, checks.NewSyntaxCheck())
	}

	proms := []PrometheusConfig{}
	for _, prom := range cfg.Prometheus {
		if prom.isEnabledForPath(path) {
			proms = append(proms, prom)
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
			enabled = append(enabled, c)

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

func Load(path string) (cfg Config, err error) {
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

	if _, err := os.Stat(path); err != nil {

		if path != ".pint.hcl" {
			return cfg, err
		}
	} else {
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
	}

	for _, rule := range cfg.Rules {
		if rule.Match != nil {
			if err = rule.Match.validate(); err != nil {
				return cfg, err
			}
		}

		for _, aggr := range rule.Aggregate {
			if err = aggr.validate(); err != nil {
				return cfg, err
			}
		}

		if rule.Rate != nil {
			if err = rule.Rate.validate(); err != nil {
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

		if rule.Series != nil {
			if err = rule.Series.validate(); err != nil {
				return cfg, err
			}
		}

		if rule.Alerts != nil {
			if err = rule.Alerts.validate(); err != nil {
				return cfg, err
			}
		}

		if rule.Value != nil {
			if err = rule.Value.validate(); err != nil {
				return cfg, err
			}
		}

		for _, reject := range rule.Reject {
			if err = reject.validate(); err != nil {
				return cfg, err
			}

		}

		if rule.Comparison != nil {
			if err = rule.Comparison.validate(); err != nil {
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
