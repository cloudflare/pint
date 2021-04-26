package config

import "regexp"

type CI struct {
	Include    []string `hcl:"include"`
	MaxCommits int      `hcl:"maxCommits,optional"`
	BaseBranch string   `hcl:"baseBranch,optional"`
}

func (ci CI) validate() error {
	for _, pattern := range ci.Include {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
	}
	return nil
}
