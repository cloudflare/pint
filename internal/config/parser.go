package config

import (
	"regexp"
)

type Parser struct {
	Relaxed []string `hcl:"relaxed,optional" json:"relaxed,omitempty"`
	Include []string `hcl:"include,optional" json:"include,omitempty"`
	Exclude []string `hcl:"exclude,optional" json:"exclude,omitempty"`
}

func (p Parser) validate() error {
	for _, pattern := range p.Relaxed {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
	}
	for _, path := range p.Include {
		if _, err := regexp.Compile(path); err != nil {
			return err
		}
	}

	for _, path := range p.Exclude {
		if _, err := regexp.Compile(path); err != nil {
			return err
		}
	}
	return nil
}
