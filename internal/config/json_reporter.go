package config

import "errors"

type JSONReporterSettings struct {
	Path string `hcl:"path" json:"path"`
}

func (ag JSONReporterSettings) validate() error {
	if ag.Path == "" {
		return errors.New("empty path")
	}

	return nil
}
