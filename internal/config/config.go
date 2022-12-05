package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	"github.com/zclconf/go-cty/cty"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

type Config struct {
	CI                *CI                      `hcl:"ci,block" json:"ci,omitempty"`
	Parser            *Parser                  `hcl:"parser,block" json:"parser,omitempty"`
	Repository        *Repository              `hcl:"repository,block" json:"repository,omitempty"`
	Prometheus        []PrometheusConfig       `hcl:"prometheus,block" json:"prometheus,omitempty"`
	Checks            *Checks                  `hcl:"checks,block" json:"checks,omitempty"`
	Check             []Check                  `hcl:"check,block" json:"check,omitempty"`
	Rules             []Rule                   `hcl:"rule,block" json:"rules,omitempty"`
	PrometheusServers []*promapi.FailoverGroup `json:"-"`
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

func (cfg *Config) GetChecksForRule(ctx context.Context, path string, r parser.Rule, disabledChecks []string) []checks.RuleChecker {
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
	}

	proms := []*promapi.FailoverGroup{}
	for _, prom := range cfg.Prometheus {
		if !prom.isEnabledForPath(path) {
			continue
		}
		for _, p := range cfg.PrometheusServers {
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
		allChecks = append(allChecks, checkMeta{
			name:  checks.RangeQueryCheckName,
			check: checks.NewRangeQueryCheck(p),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.RuleDuplicateCheckName,
			check: checks.NewRuleDuplicateCheck(p),
		})
		allChecks = append(allChecks, checkMeta{
			name:  checks.LabelsConflictCheckName,
			check: checks.NewLabelsConflictCheck(p),
		})
	}

	for _, rule := range cfg.Rules {
		allChecks = append(allChecks, rule.resolveChecks(ctx, path, r, cfg.Checks.Enabled, cfg.Checks.Disabled, proms)...)
	}

	for _, cm := range allChecks {
		// check if check is disabled for specific rule
		if !isEnabled(cfg.Checks.Enabled, disabledChecks, r, cm.name, cm.check) {
			continue
		}

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

func getContext() *hcl.EvalContext {
	vars := map[string]cty.Value{}
	for _, e := range os.Environ() {
		if i := strings.Index(e, "="); i >= 0 {
			vars[fmt.Sprintf("ENV_%s", e[:i])] = cty.StringVal(e[i+1:])
		}
	}
	return &hcl.EvalContext{Variables: vars}
}

func Load(path string, failOnMissing bool) (cfg Config, err error) {
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
	}

	if _, err = os.Stat(path); err == nil || failOnMissing {
		log.Info().Str("path", path).Msg("Loading configuration file")
		ectx := getContext()
		err = hclsimple.DecodeFile(path, ectx, &cfg)
		if err != nil {
			return cfg, err
		}
	}

	if cfg.CI != nil {
		if err = cfg.CI.validate(); err != nil {
			return cfg, err
		}
	}

	if cfg.Parser != nil {
		if err = cfg.Parser.validate(); err != nil {
			return cfg, err
		}
	}

	if cfg.Repository != nil && cfg.Repository.BitBucket != nil {
		if cfg.Repository.BitBucket.Timeout == "" {
			cfg.Repository.BitBucket.Timeout = time.Minute.String()
		}
		if err = cfg.Repository.BitBucket.validate(); err != nil {
			return cfg, err
		}
	}

	if cfg.Repository != nil && cfg.Repository.GitHub != nil {
		if cfg.Repository.GitHub.Timeout == "" {
			cfg.Repository.GitHub.Timeout = time.Minute.String()
		}
		if err = cfg.Repository.GitHub.validate(); err != nil {
			return cfg, err
		}
	}

	if cfg.Checks != nil {
		if err = cfg.Checks.validate(); err != nil {
			return cfg, err
		}
	}

	for _, chk := range cfg.Check {
		if err = chk.validate(); err != nil {
			return cfg, err
		}
	}

	promNames := make([]string, 0, len(cfg.Prometheus))
	for i, prom := range cfg.Prometheus {
		if err = prom.validate(); err != nil {
			return cfg, err
		}

		if slices.Contains(promNames, prom.Name) {
			return cfg, fmt.Errorf("prometheus server name must be unique, found two or more config blocks using %q name", prom.Name)
		}
		promNames = append(promNames, prom.Name)

		var timeout time.Duration
		if prom.Timeout != "" {
			timeout, _ = parseDuration(prom.Timeout)
		} else {
			timeout = time.Minute * 2
			cfg.Prometheus[i].Timeout = timeout.String()
		}

		concurrency := prom.Concurrency
		if concurrency <= 0 {
			concurrency = 16
			cfg.Prometheus[i].Concurrency = concurrency
		}

		rateLimit := prom.RateLimit
		if rateLimit <= 0 {
			rateLimit = 100
			cfg.Prometheus[i].RateLimit = rateLimit
		}

		uptime := prom.Uptime
		if uptime == "" {
			uptime = "up"
			cfg.Prometheus[i].Uptime = uptime
		}

		upstreams := []*promapi.Prometheus{
			promapi.NewPrometheus(prom.Name, prom.URI, prom.Headers, timeout, concurrency, rateLimit),
		}
		for _, uri := range prom.Failover {
			upstreams = append(upstreams, promapi.NewPrometheus(prom.Name, uri, prom.Headers, timeout, concurrency, rateLimit))
		}
		var include, exclude []*regexp.Regexp
		for _, path := range prom.Include {
			include = append(include, strictRegex(path))
		}
		for _, path := range prom.Exclude {
			exclude = append(exclude, strictRegex(path))
		}
		cfg.PrometheusServers = append(cfg.PrometheusServers, promapi.NewFailoverGroup(prom.Name, upstreams, prom.Required, uptime, include, exclude))
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
