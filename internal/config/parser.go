package config

import (
	"fmt"
	"regexp"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/parser"
)

const (
	SchemaPrometheus = "prometheus"
	SchemaThanos     = "thanos"

	NamesLegacy = "legacy"
	NamesUTF8   = "utf-8"
)

type Parser struct {
	Schema  string   `hcl:"schema,optional" json:"schema,omitempty"`
	Names   string   `hcl:"names,optional" json:"names,omitempty"`
	Relaxed []string `hcl:"relaxed,optional" json:"relaxed,omitempty"`
	Include []string `hcl:"include,optional" json:"include,omitempty"`
	Exclude []string `hcl:"exclude,optional" json:"exclude,omitempty"`

	opts parser.Options
}

func (p Parser) Options() parser.Options {
	return p.opts
}

func (p Parser) getSchema() string {
	if p.Schema == "" {
		return SchemaPrometheus
	}
	return p.Schema
}

func (p Parser) getNames() string {
	if p.Names == "" {
		return NamesUTF8
	}
	return p.Names
}

func (p Parser) validate() error {
	switch s := p.getSchema(); s {
	case SchemaPrometheus:
	case SchemaThanos:
	default:
		return fmt.Errorf("unsupported parser schema: %s", s)
	}

	switch n := p.getNames(); n {
	case NamesUTF8:
	case NamesLegacy:
	default:
		return fmt.Errorf("unsupported parser names: %s", n)
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

func (p *Parser) initOptions() {
	var names model.ValidationScheme
	if p.getNames() == NamesLegacy {
		names = model.LegacyValidation
	} else {
		names = model.UTF8Validation
	}

	var schema parser.Schema
	if p.getSchema() == SchemaThanos {
		schema = parser.ThanosSchema
	}

	p.opts = parser.Options{
		Names:    names,
		Schema:   schema,
		IsStrict: false,
	}
}
