package config

import (
	"errors"
	"fmt"
	"go/parser"
	"regexp"
	"strings"
)

type PrometheusConfig struct {
	Name        string            `hcl:",label" json:"name"`
	URI         string            `hcl:"uri" json:"uri"`
	Headers     map[string]string `hcl:"headers,optional" json:"headers,omitempty"`
	Failover    []string          `hcl:"failover,optional" json:"failover,omitempty"`
	Timeout     string            `hcl:"timeout,optional"  json:"timeout"`
	Concurrency int               `hcl:"concurrency,optional" json:"concurrency"`
	RateLimit   int               `hcl:"rateLimit,optional" json:"rateLimit"`
	Uptime      string            `hcl:"uptime,optional" json:"uptime"`
	Include     []string          `hcl:"include,optional" json:"include,omitempty"`
	Exclude     []string          `hcl:"exclude,optional" json:"exclude,omitempty"`
	Tags        []string          `hcl:"tags,optional" json:"tags,omitempty"`
	Required    bool              `hcl:"required,optional" json:"required"`
}

func (pc PrometheusConfig) validate() error {
	if pc.URI == "" {
		return errors.New("prometheus URI cannot be empty")
	}

	if pc.Timeout != "" {
		if _, err := parseDuration(pc.Timeout); err != nil {
			return err
		}
	}

	if pc.Uptime != "" {
		if _, err := parser.ParseExpr(pc.Uptime); err != nil {
			return fmt.Errorf("invalid Prometheus uptime metric selector %q: %w", pc.Uptime, err)
		}
	}

	for _, path := range pc.Include {
		if _, err := regexp.Compile(path); err != nil {
			return err
		}
	}

	for _, path := range pc.Exclude {
		if _, err := regexp.Compile(path); err != nil {
			return err
		}
	}

	for _, tag := range pc.Tags {
		for _, s := range []string{" ", "\n"} {
			if strings.Contains(tag, s) {
				return fmt.Errorf("prometheus tag %q cannot contain %q", tag, s)
			}
		}
	}

	return nil
}

func (pc PrometheusConfig) isEnabledForPath(path string) bool {
	if len(pc.Include) == 0 && len(pc.Exclude) == 0 {
		return true
	}
	for _, pattern := range pc.Exclude {
		re := strictRegex(pattern)
		if re.MatchString(path) {
			return false
		}
	}
	for _, pattern := range pc.Include {
		re := strictRegex(pattern)
		if re.MatchString(path) {
			return true
		}
	}
	return false
}
