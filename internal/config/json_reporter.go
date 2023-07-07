package config

import "errors"

type JSONReporterSettings struct {
	Path string `hcl:"path" json:"path"`
}

func (settings JSONReporterSettings) validate() error {
	if settings.Path == "" {
		return errors.New("empty path")
	}

	return nil
}
