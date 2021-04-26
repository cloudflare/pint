package config

import (
	"github.com/cloudflare/pint/internal/checks"
)

type ValueSettings struct {
	Severity string `hcl:"severity,optional"`
}

func (vs ValueSettings) validate() error {
	if vs.Severity != "" {
		if _, err := checks.ParseSeverity(vs.Severity); err != nil {
			return err
		}
	}
	return nil
}

func (vs ValueSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if vs.Severity != "" {
		sev, _ := checks.ParseSeverity(vs.Severity)
		return sev
	}
	return fallback
}
