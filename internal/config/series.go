package config

import (
	"github.com/cloudflare/pint/internal/checks"
)

type SeriesSettings struct {
	Severity string `hcl:"severity,optional"`
	IgnoreRR bool   `hcl:"ignore_recordingrules,optional"`
}

func (rs SeriesSettings) validate() error {
	if rs.Severity != "" {
		if _, err := checks.ParseSeverity(rs.Severity); err != nil {
			return err
		}
	}
	return nil
}

func (rs SeriesSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if rs.Severity != "" {
		sev, _ := checks.ParseSeverity(rs.Severity)
		return sev
	}
	return fallback
}
