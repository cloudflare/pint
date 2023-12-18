package config

import (
	"fmt"

	"github.com/cloudflare/pint/internal/checks"
)

type CostSettings struct {
	MaxEvaluationDuration string `hcl:"maxEvaluationDuration,optional" json:"maxEvaluationDuration,omitempty"`
	Severity              string `hcl:"severity,optional" json:"severity,omitempty"`
	MaxSeries             int    `hcl:"maxSeries,optional" json:"maxSeries,omitempty"`
	MaxPeakSamples        int    `hcl:"maxPeakSamples,optional" json:"maxPeakSamples,omitempty"`
	MaxTotalSamples       int    `hcl:"maxTotalSamples,optional" json:"maxTotalSamples,omitempty"`
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
	if cs.MaxTotalSamples < 0 {
		return fmt.Errorf("maxTotalSamples value must be >= 0")
	}
	if cs.MaxPeakSamples < 0 {
		return fmt.Errorf("maxPeakSamples value must be >= 0")
	}
	if cs.MaxEvaluationDuration != "" {
		if _, err := parseDuration(cs.MaxEvaluationDuration); err != nil {
			return err
		}
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
