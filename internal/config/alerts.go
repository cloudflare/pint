package config

import (
	"fmt"

	"github.com/cloudflare/pint/internal/checks"
)

type AlertsSettings struct {
	Range    string `hcl:"range" json:"range"`
	Step     string `hcl:"step" json:"step"`
	Resolve  string `hcl:"resolve" json:"resolve"`
	MinCount int    `hcl:"minCount,optional" json:"minCount,omitempty"`
	Severity string `hcl:"severity,optional" json:"severity,omitempty"`
}

func (as AlertsSettings) validate() error {
	if as.Range != "" {
		if _, err := parseDuration(as.Range); err != nil {
			return err
		}
	}
	if as.Step != "" {
		if _, err := parseDuration(as.Step); err != nil {
			return err
		}
	}
	if as.Resolve != "" {
		if _, err := parseDuration(as.Resolve); err != nil {
			return err
		}
	}
	if as.MinCount < 0 {
		return fmt.Errorf("minCount cannot be < 0, got %d", as.MinCount)
	}
	if as.Severity != "" {
		sev, err := checks.ParseSeverity(as.Severity)
		if err != nil {
			return err
		}
		if as.MinCount <= 0 && sev > checks.Information {
			return fmt.Errorf("cannot set serverity to %q when minCount is 0", as.Severity)
		}
	}
	return nil
}

func (as AlertsSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if as.Severity != "" {
		sev, _ := checks.ParseSeverity(as.Severity)
		return sev
	}
	return fallback
}
