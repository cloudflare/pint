package config

import (
	"errors"
)

type CI struct {
	BaseBranch string   `hcl:"baseBranch,optional" json:"baseBranch,omitempty"`
	Include    []string `hcl:"include,optional" json:"include,omitempty"`
	Exclude    []string `hcl:"exclude,optional" json:"exclude,omitempty"`
	MaxCommits int      `hcl:"maxCommits,optional" json:"maxCommits,omitempty"`
}

func (ci CI) validate() error {
	if ci.MaxCommits <= 0 {
		return errors.New("maxCommits cannot be <= 0")
	}

	if err := ValidatePaths(ci.Include); err != nil {
		return err
	}
	if err := ValidatePaths(ci.Exclude); err != nil {
		return err
	}

	return nil
}
