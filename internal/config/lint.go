package config

type Lint struct {
	Include []string `hcl:"include,optional"       json:"include,omitempty"`
	Exclude []string `hcl:"exclude,optional"       json:"exclude,omitempty"`
}

func (lint Lint) validate() error {
	if err := ValidatePaths(lint.Include); err != nil {
		return err
	}
	if err := ValidatePaths(lint.Exclude); err != nil {
		return err
	}

	return nil
}
