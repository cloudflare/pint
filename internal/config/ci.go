package config

import (
	"errors"
	"fmt"

	"github.com/cloudflare/pint/internal/checks"
)

type CI struct {
	BaseBranch  string `hcl:"baseBranch,optional" json:"baseBranch,omitempty"`
	MaxCommits  int    `hcl:"maxCommits,optional" json:"maxCommits,omitempty"`
	MinSeverity string `hcl:"minSeverity,optional" json:"minSeverity,omitempty"`
}

func (ci CI) validate() error {
	if ci.MaxCommits <= 0 {
		return errors.New("maxCommits cannot be <= 0")
	}
	if _, err := checks.ParseSeverity(ci.MinSeverity); err != nil {
		return fmt.Errorf("invalid minSeverity: %w", err)
	}
	return nil
}
