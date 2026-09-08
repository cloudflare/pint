package config

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"

	"github.com/cloudflare/pint/internal/checks"
)

type Check struct {
	Body hcl.Body `hcl:",remain"`
	Name string   `hcl:",label"`
}

func (c Check) MarshalJSONTo(enc *jsontext.Encoder) error {
	s, err := c.Decode()
	if err != nil {
		return err
	}
	return json.MarshalEncode(enc, s)
}

func (c Check) Decode() (s CheckSettings, err error) {
	switch c.Name {
	case checks.SeriesCheckName:
		s = &checks.PromqlSeriesSettings{}
	case checks.RegexpCheckName:
		s = &checks.PromqlRegexpSettings{}
	case checks.RuleDependencyCheckName:
		s = &checks.RuleDependencySettings{}
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
