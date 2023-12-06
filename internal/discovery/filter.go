package discovery

import "regexp"

func NewPathFilter(include, exclude, relaxed []*regexp.Regexp) PathFilter {
	return PathFilter{
		include: include,
		exclude: exclude,
		relaxed: relaxed,
	}
}

type PathFilter struct {
	include []*regexp.Regexp
	exclude []*regexp.Regexp
	relaxed []*regexp.Regexp
}

func (pf PathFilter) IsPathAllowed(path string) bool {
	if len(pf.include) == 0 && len(pf.exclude) == 0 {
		return true
	}

	for _, pattern := range pf.exclude {
		if pattern.MatchString(path) {
			return false
		}
	}

	for _, pattern := range pf.include {
		if pattern.MatchString(path) {
			return true
		}
	}

	return false
}

func (pf PathFilter) IsRelaxed(path string) bool {
	for _, r := range pf.relaxed {
		if v := r.MatchString(path); v {
			return true
		}
	}
	return false
}
