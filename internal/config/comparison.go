package config

import (
	"github.com/cloudflare/pint/internal/checks"
)

type ComparisonSettings struct {
	Severity string `hcl:"severity,optional"`
}

func (cs ComparisonSettings) validate() error {
	if cs.Severity != "" {
		if _, err := checks.ParseSeverity(cs.Severity); err != nil {
			return err
		}
	}
	return nil
}

func (cs ComparisonSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if cs.Severity != "" {
		sev, _ := checks.ParseSeverity(cs.Severity)
		return sev
	}
	return fallback
}
