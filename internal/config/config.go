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

func (cfg *Config) GetChecksForEntry(ctx context.Context, gen *PrometheusGenerator, entry discovery.Entry) []checks.RuleChecker {
	enabled := []checks.RuleChecker{}

	defaultStates := defaultMatchStates(commandFromContext(ctx))
	defaultMatch := []Match{{State: defaultStates}}
	proms := gen.ServersForPath(entry.Path.Name)

	if entry.PathError != nil || entry.Rule.Error.Err != nil {
		check := checks.NewErrorCheck(entry)
		enabled = parsedRule{
			match: defaultMatch,
			name:  check.Reporter(),
			check: check,
		}.entryChecks(ctx, cfg.Checks.Enabled, cfg.Checks.Disabled, enabled, entry)
	} else {
		for _, pr := range baseRules(proms, defaultMatch) {
			enabled = pr.entryChecks(ctx, cfg.Checks.Enabled, cfg.Checks.Disabled, enabled, entry)
		}
		for _, rule := range cfg.Rules {
			for _, pr := range parseRule(rule, proms, defaultStates) {
				enabled = pr.entryChecks(ctx, cfg.Checks.Enabled, cfg.Checks.Disabled, enabled, entry)
			}
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

func commandFromContext(ctx context.Context) (cmd ContextCommandVal) {
	if val := ctx.Value(CommandKey); val != nil {
		cmd = val.(ContextCommandVal)
	}
	return cmd
}
