package config

import (
	"regexp"
)

type Owners struct {
	Allowed []string `hcl:"allowed,optional" json:"allowed,omitempty"`
}

func (o Owners) validate() error {
	for _, pattern := range o.Allowed {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o Owners) CompileAllowed() (r []*regexp.Regexp) {
	if len(o.Allowed) == 0 {
		r = append(r, regexp.MustCompile(".*"))
		return r
	}
	r = append(r, MustCompileRegexes(o.Allowed...)...)
	return r
}
