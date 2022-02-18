package config

import (
	"errors"
	"regexp"
)

type PrometheusConfig struct {
	Name     string   `hcl:",label" json:"name"`
	URI      string   `hcl:"uri" json:"uri"`
	Failover []string `hcl:"failover,optional" json:"failover,omitempty"`
	Timeout  string   `hcl:"timeout"  json:"timeout"`
	Paths    []string `hcl:"paths,optional" json:"paths,omitempty"`
	Required bool     `hcl:"required,optional" json:"required"`
}

func (pc PrometheusConfig) validate() error {
	if pc.URI == "" {
		return errors.New("prometheus URI cannot be empty")
	}

	if _, err := parseDuration(pc.Timeout); err != nil {
		return err
	}

	for _, path := range pc.Paths {
		if _, err := regexp.Compile(path); err != nil {
			return err
		}
	}

	return nil
}

func (pc PrometheusConfig) isEnabledForPath(path string) bool {
	if len(pc.Paths) == 0 {
		return true
	}
	for _, pattern := range pc.Paths {
		re := strictRegex(pattern)
		if re.MatchString(path) {
			return true
		}
	}
	return false
}
