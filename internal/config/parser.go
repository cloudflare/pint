package config

import (
	"fmt"
	"regexp"
)

const (
	SchemaPrometheus = "prometheus"
	SchemaThanos     = "thanos"
)

type Parser struct {
	Schema  string   `hcl:"schema,optional" json:"schema,omitempty"`
	Relaxed []string `hcl:"relaxed,optional" json:"relaxed,omitempty"`
	Include []string `hcl:"include,optional" json:"include,omitempty"`
	Exclude []string `hcl:"exclude,optional" json:"exclude,omitempty"`
}

func (p Parser) getSchema() string {
	if p.Schema == "" {
		return SchemaPrometheus
	}
	return p.Schema
}

func (p Parser) validate() error {
	switch s := p.getSchema(); s {
	case SchemaPrometheus:
	case SchemaThanos:
	default:
		return fmt.Errorf("unsupported parser scheme: %s", s)
	}

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
