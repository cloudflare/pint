package config

import (
	"regexp"

	"github.com/cloudflare/pint/internal/checks"
)

type RejectSettings struct {
	Regex            string `hcl:",label"`
	LabelKeys        bool   `hcl:"label_keys,optional"`
	LabelValues      bool   `hcl:"label_values,optional"`
	AnnotationKeys   bool   `hcl:"annotation_keys,optional"`
	AnnotationValues bool   `hcl:"annotation_values,optional"`
	Severity         string `hcl:"severity,optional"`
}

func (rs RejectSettings) validate() error {
	if rs.Severity != "" {
		if _, err := checks.ParseSeverity(rs.Severity); err != nil {
			return err
		}
	}

	if _, err := regexp.Compile(rs.Regex); err != nil {
		return err
	}

	return nil
}

func (rs RejectSettings) getSeverity(fallback checks.Severity) checks.Severity {
	if rs.Severity != "" {
		sev, _ := checks.ParseSeverity(rs.Severity)
		return sev
	}
	return fallback
}
