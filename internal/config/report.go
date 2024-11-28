package config

import (
	"errors"

	"github.com/cloudflare/pint/internal/checks"
)

type ReportSettings struct {
	Comment  string `hcl:"comment" json:"comment"`
	Severity string `hcl:"severity" json:"severity"`
}

func (rs ReportSettings) validate() error {
	if rs.Comment == "" {
		return errors.New("report comment cannot be empty")
	}

	if rs.Severity != "" {
		if _, err := checks.ParseSeverity(rs.Severity); err != nil {
			return err
		}
	}

	return nil
}

func (rs ReportSettings) getSeverity() checks.Severity {
	sev, _ := checks.ParseSeverity(rs.Severity)
	return sev
}
