package config

import (
	"regexp"
)

type Parser struct {
	Relaxed []string `hcl:"relaxed,optional" json:"relaxed,omitempty"`
}

func (p Parser) validate() error {
	for _, pattern := range p.Relaxed {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p Parser) CompileRelaxed() (r []*regexp.Regexp) {
	for _, pattern := range p.Relaxed {
		r = append(r, regexp.MustCompile("^"+pattern+"$"))
	}
	return r
}
