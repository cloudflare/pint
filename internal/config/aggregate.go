package config

import (
	"fmt"
	"regexp"

	"github.com/cloudflare/pint/internal/checks"
)

type AggregateSettings struct {
	Name     string   `hcl:",label"`
	Keep     []string `hcl:"keep,optional"`
	Strip    []string `hcl:"strip,optional"`
	Severity string   `hcl:"severity,optional"`
}

func (ag AggregateSettings) validate() error {
	if ag.Name == "" {
		return fmt.Errorf("empty name regex")
	}

	if ag.Severity != "" {
		if _, err := checks.ParseSeverity(ag.Severity); err != nil {
			return err
		}
	}

	if _, err := regexp.Compile(ag.Name); err != nil {
		return err
	}
	return nil
}

func (ag AggregateSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if ag.Severity != "" {
		sev, _ := checks.ParseSeverity(ag.Severity)
		return sev
	}
	return fallback
}
