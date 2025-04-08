package config

import (
	"github.com/cloudflare/pint/internal/checks"
)

type RuleNameSettings struct {
	Regex    string `hcl:",label"            json:"key,omitempty"`
	Comment  string `hcl:"comment,optional"  json:"comment,omitempty"`
	Severity string `hcl:"severity,optional" json:"severity,omitempty"`
}

func (rs RuleNameSettings) validate() error {
	if _, err := checks.NewTemplatedRegexp(rs.Regex); err != nil {
		return err
	}

	if rs.Severity != "" {
		if _, err := checks.ParseSeverity(rs.Severity); err != nil {
			return err
		}
	}

	return nil
}

func (rs RuleNameSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if rs.Severity != "" {
		sev, _ := checks.ParseSeverity(rs.Severity)
		return sev
	}
	return fallback
}
