package config

import (
	"errors"
	"regexp"
)

type CI struct {
	Include    []string `hcl:"include,optional" json:"include,omitempty"`
	Exclude    []string `hcl:"exclude,optional" json:"exclude,omitempty"`
	MaxCommits int      `hcl:"maxCommits,optional" json:"maxCommits,omitempty"`
	BaseBranch string   `hcl:"baseBranch,optional" json:"baseBranch,omitempty"`
}

func (ci CI) validate() error {
	if ci.MaxCommits <= 0 {
		return errors.New("maxCommits cannot be <= 0")
	}

	for _, pattern := range ci.Include {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
	}

	for _, pattern := range ci.Exclude {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
	}
	return nil
}
