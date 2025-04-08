package config

import (
	"errors"

	"github.com/cloudflare/pint/internal/checks"
)

type AnnotationSettings struct {
	Key      string   `hcl:",label"            json:"key"`
	Token    string   `hcl:"token,optional"    json:"token,omitempty"`
	Value    string   `hcl:"value,optional"    json:"value,omitempty"`
	Comment  string   `hcl:"comment,optional"  json:"comment,omitempty"`
	Severity string   `hcl:"severity,optional" json:"severity,omitempty"`
	Values   []string `hcl:"values,optional"   json:"values,omitempty"`
	Required bool     `hcl:"required,optional" json:"required,omitempty"`
}

func (as AnnotationSettings) validate() error {
	if as.Key == "" {
		return errors.New("annotation key cannot be empty")
	}

	if _, err := checks.NewTemplatedRegexp(as.Key); err != nil {
		return err
	}

	if _, err := checks.NewRawTemplatedRegexp(as.Token); err != nil {
		return err
	}

	if _, err := checks.NewTemplatedRegexp(as.Value); err != nil {
		return err
	}

	if as.Severity != "" {
		if _, err := checks.ParseSeverity(as.Severity); err != nil {
			return err
		}
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
