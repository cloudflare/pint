package config

import (
	"github.com/cloudflare/pint/internal/checks"
)

type RuleLinkSettings struct {
	Regex    string            `hcl:",label" json:"key,omitempty"`
	URI      string            `hcl:"uri,optional" json:"uri,omitempty"`
	Timeout  string            `hcl:"timeout,optional" json:"timeout,omitempty"`
	Headers  map[string]string `hcl:"headers,optional" json:"headers,omitempty"`
	Comment  string            `hcl:"comment,optional" json:"comment,omitempty"`
	Severity string            `hcl:"severity,optional" json:"severity,omitempty"`
}

func (s RuleLinkSettings) validate() error {
	if _, err := checks.NewTemplatedRegexp(s.Regex); err != nil {
		return err
	}

	if s.Timeout != "" {
		if _, err := parseDuration(s.Timeout); err != nil {
			return err
		}
	}

	if s.Severity != "" {
		if _, err := checks.ParseSeverity(s.Severity); err != nil {
			return err
		}
	}

	return nil
}

func (s RuleLinkSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if s.Severity != "" {
		sev, _ := checks.ParseSeverity(s.Severity)
		return sev
	}
	return fallback
}
