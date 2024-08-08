package config

import (
	"errors"
)

type CI struct {
	BaseBranch string `hcl:"baseBranch,optional" json:"baseBranch,omitempty"`
	MaxCommits int    `hcl:"maxCommits,optional" json:"maxCommits,omitempty"`
}

func (ci CI) validate() error {
	if ci.MaxCommits <= 0 {
		return errors.New("maxCommits cannot be <= 0")
	}
	return nil
}
