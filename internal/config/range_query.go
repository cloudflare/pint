package config

import (
	"errors"

	"github.com/cloudflare/pint/internal/checks"
)

type RangeQuerySettings struct {
	Max      string `hcl:"max"               json:"max"`
	Comment  string `hcl:"comment,optional"  json:"comment,omitempty"`
	Severity string `hcl:"severity,optional" json:"severity,omitempty"`
}

func (s RangeQuerySettings) validate() error {
	if s.Max != "" {
		dur, err := parseDuration(s.Max)
		if err != nil {
			return err
		}
		if dur == 0 {
			return errors.New("range_query max value cannot be zero")
		}
	}

	if s.Severity != "" {
		if _, err := checks.ParseSeverity(s.Severity); err != nil {
			return err
		}
	}

	return nil
}

func (s RangeQuerySettings) getSeverity(fallback checks.Severity) checks.Severity {
	if s.Severity != "" {
		sev, _ := checks.ParseSeverity(s.Severity)
		return sev
	}
	return fallback
}
