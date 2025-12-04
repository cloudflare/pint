package git_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/git"
)

func TestPathFilterIsPathAllowed(t *testing.T) {
	type testCaseT struct {
		title   string
		path    string
		include []*regexp.Regexp
		exclude []*regexp.Regexp
		allowed bool
	}

	testCases := []testCaseT{
		{
			title:   "no filters - allow all",
			include: nil,
			exclude: nil,
			path:    "foo.txt",
			allowed: true,
		},
		{
			title:   "include only - path matches",
			include: []*regexp.Regexp{regexp.MustCompile(`\.yaml$`)},
			exclude: nil,
			path:    "rules.yaml",
			allowed: true,
		},
		{
			title:   "include only - path does not match",
			include: []*regexp.Regexp{regexp.MustCompile(`\.yaml$`)},
			exclude: nil,
			path:    "rules.txt",
			allowed: false,
		},
		{
			title:   "exclude only - path matches exclude",
			include: nil,
			exclude: []*regexp.Regexp{regexp.MustCompile(`^vendor/`)},
			path:    "vendor/lib.go",
			allowed: false,
		},
		{
			title:   "exclude only - path does not match exclude",
			include: nil,
			exclude: []*regexp.Regexp{regexp.MustCompile(`^vendor/`)},
			path:    "main.go",
			allowed: true,
		},
		{
			title:   "include and exclude - path matches both",
			include: []*regexp.Regexp{regexp.MustCompile(`\.go$`)},
			exclude: []*regexp.Regexp{regexp.MustCompile(`^vendor/`)},
			path:    "vendor/lib.go",
			allowed: false,
		},
		{
			title:   "include and exclude - path matches include only",
			include: []*regexp.Regexp{regexp.MustCompile(`\.go$`)},
			exclude: []*regexp.Regexp{regexp.MustCompile(`^vendor/`)},
			path:    "main.go",
			allowed: true,
		},
		{
			title:   "include and exclude - path matches neither",
			include: []*regexp.Regexp{regexp.MustCompile(`\.go$`)},
			exclude: []*regexp.Regexp{regexp.MustCompile(`^vendor/`)},
			path:    "README.md",
			allowed: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			filter := git.NewPathFilter(tc.include, tc.exclude, nil)
			allowed := filter.IsPathAllowed(tc.path)
			require.Equal(t, tc.allowed, allowed)
		})
	}
}

func TestPathFilterIsRelaxed(t *testing.T) {
	type testCaseT struct {
		title   string
		path    string
		relaxed []*regexp.Regexp
		isMatch bool
	}

	testCases := []testCaseT{
		{
			title:   "no relaxed patterns",
			relaxed: nil,
			path:    "foo.txt",
			isMatch: false,
		},
		{
			title:   "path matches relaxed pattern",
			relaxed: []*regexp.Regexp{regexp.MustCompile(`^tests/`)},
			path:    "tests/foo.yaml",
			isMatch: true,
		},
		{
			title:   "path does not match relaxed pattern",
			relaxed: []*regexp.Regexp{regexp.MustCompile(`^tests/`)},
			path:    "rules/foo.yaml",
			isMatch: false,
		},
		{
			title:   "multiple relaxed patterns - matches first",
			relaxed: []*regexp.Regexp{regexp.MustCompile(`^tests/`), regexp.MustCompile(`^examples/`)},
			path:    "tests/foo.yaml",
			isMatch: true,
		},
		{
			title:   "multiple relaxed patterns - matches second",
			relaxed: []*regexp.Regexp{regexp.MustCompile(`^tests/`), regexp.MustCompile(`^examples/`)},
			path:    "examples/foo.yaml",
			isMatch: true,
		},
		{
			title:   "multiple relaxed patterns - matches none",
			relaxed: []*regexp.Regexp{regexp.MustCompile(`^tests/`), regexp.MustCompile(`^examples/`)},
			path:    "rules/foo.yaml",
			isMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			filter := git.NewPathFilter(nil, nil, tc.relaxed)
			isMatch := filter.IsRelaxed(tc.path)
			require.Equal(t, tc.isMatch, isMatch)
		})
	}
}
