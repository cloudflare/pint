package config

import (
	"fmt"

	"github.com/cloudflare/pint/internal/checks"
)

type Checks struct {
	Enabled  []string `hcl:"enabled,optional" json:"enabled,omitempty"`
	Disabled []string `hcl:"disabled,optional" json:"disabled,omitempty"`
}

func (c Checks) validate() error {
	for _, name := range c.Enabled {
		if err := validateCheckName(name); err != nil {
			return err
		}
	}
	for _, name := range c.Disabled {
		if err := validateCheckName(name); err != nil {
			return err
		}
	}

	return nil
}

func validateCheckName(name string) error {
	for _, c := range checks.CheckNames {
		if name == c {
			return nil
		}
	}
	return fmt.Errorf("unknown check name %s", name)
}
