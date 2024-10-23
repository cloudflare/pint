package config

import (
	"encoding/json"
	"fmt"

	"github.com/cloudflare/pint/internal/checks"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
)

type Check struct {
	Body hcl.Body `hcl:",remain" json:"-"`
	Name string   `hcl:",label" json:"name"`
}

func (c Check) MarshalJSON() ([]byte, error) {
	s, err := c.Decode()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(s, "", "  ")
}

func (c Check) Decode() (s CheckSettings, err error) {
	switch c.Name {
	case checks.SeriesCheckName:
		s = &checks.PromqlSeriesSettings{}
	case checks.RegexpCheckName:
		s = &checks.PromqlRegexpSettings{}
	default:
		return nil, fmt.Errorf("unknown check %q", c.Name)
	}

	if diag := gohcl.DecodeBody(c.Body, nil, s); diag != nil && diag.HasErrors() {
		return nil, diag
	}
	if err = s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (c Check) validate() error {
	s, err := c.Decode()
	if err != nil {
		return err
	}
	return s.Validate()
}

type CheckSettings interface {
	Validate() error
}
