package config

import (
	"github.com/cloudflare/pint/internal/checks"
)

type TemplateSettings struct {
	Severity string `hcl:"severity,optional"`
}

func (ts TemplateSettings) validate() error {
	if ts.Severity != "" {
		if _, err := checks.ParseSeverity(ts.Severity); err != nil {
			return err
		}
	}
	return nil
}

func (ts TemplateSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if ts.Severity != "" {
		sev, _ := checks.ParseSeverity(ts.Severity)
		return sev
	}
	return fallback
}
