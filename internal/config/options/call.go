package options

import (
	"errors"

	"github.com/cloudflare/pint/internal/checks"
)

type CallSettings struct {
	Key       string             `hcl:",label" json:"key"`
	Selectors []SelectorSettings `hcl:"selector,block" json:"selector,omitempty"`
}

func (cs CallSettings) Validate() error {
	if cs.Key == "" {
		return errors.New("call key cannot be empty")
	}

	if _, err := checks.NewTemplatedRegexp(cs.Key); err != nil {
		return err
	}

	if len(cs.Selectors) == 0 {
		return errors.New("you must specific at least one `selector` block")
	}

	for _, s := range cs.Selectors {
		if err := s.Validate(); err != nil {
			return err
		}
	}

	return nil
}
