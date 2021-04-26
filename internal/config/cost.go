package config

import (
	"fmt"

	"github.com/cloudflare/pint/internal/checks"
)

type CostSettings struct {
	BytesPerSample int    `hcl:"bytesPerSample,optional"`
	MaxSeries      int    `hcl:"maxSeries,optional"`
	Severity       string `hcl:"severity,optional"`
}

func (cs CostSettings) validate() error {
	if cs.Severity != "" {
		if _, err := checks.ParseSeverity(cs.Severity); err != nil {
			return err
		}
	}
	if cs.MaxSeries < 0 {
		return fmt.Errorf("maxSeries value must be >= 0")
	}
	return nil
}

func (cs CostSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if cs.Severity != "" {
		sev, _ := checks.ParseSeverity(cs.Severity)
		return sev
	}
	return fallback
}
