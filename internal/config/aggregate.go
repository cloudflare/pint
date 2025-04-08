package config

import (
	"errors"

	"github.com/cloudflare/pint/internal/checks"
)

type AggregateSettings struct {
	Name     string   `hcl:",label"            json:"name"`
	Comment  string   `hcl:"comment,optional"  json:"comment,omitempty"`
	Severity string   `hcl:"severity,optional" json:"severity,omitempty"`
	Keep     []string `hcl:"keep,optional"     json:"keep,omitempty"`
	Strip    []string `hcl:"strip,optional"    json:"strip,omitempty"`
}

func (ag AggregateSettings) validate() error {
	if ag.Name == "" {
		return errors.New("empty name regex")
	}

	if ag.Severity != "" {
		if _, err := checks.ParseSeverity(ag.Severity); err != nil {
			return err
		}
	}

	if _, err := checks.NewTemplatedRegexp(ag.Name); err != nil {
		return err
	}

	if len(ag.Keep) == 0 && len(ag.Strip) == 0 {
		return errors.New("must specify keep or strip list")
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
