package config

import (
	"regexp"

	"github.com/cloudflare/pint/internal/checks"
)

type AnnotationSettings struct {
	Key      string `hcl:",label"`
	Value    string `hcl:"value,optional"`
	Required bool   `hcl:"required,optional"`
	Severity string `hcl:"severity,optional"`
}

func (as AnnotationSettings) validate() error {
	if as.Severity != "" {
		if _, err := checks.ParseSeverity(as.Severity); err != nil {
			return err
		}
	}

	if _, err := regexp.Compile(as.Value); err != nil {
		return err
	}

	return nil
}

func (as AnnotationSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if as.Severity != "" {
		sev, _ := checks.ParseSeverity(as.Severity)
		return sev
	}
	return fallback
}
