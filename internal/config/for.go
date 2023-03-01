package config

import (
	"errors"

	"github.com/cloudflare/pint/internal/checks"
)

type ForSettings struct {
	Min      string `hcl:"min,optional" json:"min,omitempty"`
	Max      string `hcl:"max,optional" json:"max,omitempty"`
	Severity string `hcl:"severity,optional" json:"severity,omitempty"`
}

func (fs ForSettings) validate() error {
	if fs.Severity != "" {
		if _, err := checks.ParseSeverity(fs.Severity); err != nil {
			return err
		}
	}
	if fs.Min != "" {
		if _, err := parseDuration(fs.Min); err != nil {
			return err
		}
	}
	if fs.Max != "" {
		if _, err := parseDuration(fs.Max); err != nil {
			return err
		}
	}
	if fs.Min == "" && fs.Max == "" {
		return errors.New("must set either min or max option, or both")
	}
	return nil
}

func (fs ForSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if fs.Severity != "" {
		sev, _ := checks.ParseSeverity(fs.Severity)
		return sev
	}
	return fallback
}
