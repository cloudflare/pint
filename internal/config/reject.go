package config

import (
	"github.com/cloudflare/pint/internal/checks"
)

type RejectSettings struct {
	Regex            string `hcl:",label"                     json:"key,omitempty"`
	Comment          string `hcl:"comment,optional"           json:"comment,omitempty"`
	Severity         string `hcl:"severity,optional"          json:"severity,omitempty"`
	LabelKeys        bool   `hcl:"label_keys,optional"        json:"label_keys,omitempty"`
	LabelValues      bool   `hcl:"label_values,optional"      json:"label_values,omitempty"`
	AnnotationKeys   bool   `hcl:"annotation_keys,optional"   json:"annotation_keys,omitempty"`
	AnnotationValues bool   `hcl:"annotation_values,optional" json:"annotation_values,omitempty"`
}

func (rs RejectSettings) validate() error {
	if _, err := checks.NewTemplatedRegexp(rs.Regex); err != nil {
		return err
	}

	if rs.Severity != "" {
		if _, err := checks.ParseSeverity(rs.Severity); err != nil {
			return err
		}
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
