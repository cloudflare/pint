package git

import (
	"log/slog"
	"regexp"
)

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

func (pf PathFilter) IsPathAllowed(path string) (ok bool) {
	if len(pf.include) == 0 && len(pf.exclude) == 0 {
		return true
	}

	for _, pattern := range pf.exclude {
		if pattern.MatchString(path) {
			slog.Debug("File path is in the exclude list",
				slog.String("path", path),
				slog.Any("exclude", pf.exclude),
			)
			return false
		}
	}

	for _, pattern := range pf.include {
		if pattern.MatchString(path) {
			slog.Debug("File path is in the include list",
				slog.String("path", path),
				slog.Any("include", pf.include),
			)
			return true
		}
	}

	ok = len(pf.include) == 0
	if !ok {
		slog.Debug("File path is not allowed",
			slog.String("path", path),
			slog.Any("include", pf.include),
			slog.Any("exclude", pf.exclude),
		)
	}
	return ok
}

func (pf PathFilter) IsRelaxed(path string) bool {
	for _, r := range pf.relaxed {
		if v := r.MatchString(path); v {
			return true
		}
	}
	return false
}
