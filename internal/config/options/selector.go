package options

import (
	"errors"

	"github.com/cloudflare/pint/internal/checks"
)

type SelectorSettings struct {
	Key            string   `hcl:",label" json:"key"`
	Comment        string   `hcl:"comment,optional" json:"comment,omitempty"`
	Severity       string   `hcl:"severity,optional" json:"severity,omitempty"`
	RequiredLabels []string `hcl:"requiredLabels" json:"requiredLabels"`
}

func (ss SelectorSettings) Validate() error {
	if ss.Key == "" {
		return errors.New("selector key cannot be empty")
	}

	if _, err := checks.NewTemplatedRegexp(ss.Key); err != nil {
		return err
	}

	if ss.Severity != "" {
		if _, err := checks.ParseSeverity(ss.Severity); err != nil {
			return err
		}
	}

	if len(ss.RequiredLabels) == 0 {
		return errors.New("requiredLabels cannot be empty")
	}

	return nil
}

func (ss SelectorSettings) GetSeverity(fallback checks.Severity) checks.Severity {
	if ss.Severity != "" {
		sev, _ := checks.ParseSeverity(ss.Severity)
		return sev
	}
	return fallback
}
