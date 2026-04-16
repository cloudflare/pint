package source_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser/source"
)

func TestParseVersion(t *testing.T) {
	testCases := []struct {
		description string
		input       string
		err         string
		expected    source.PrometheusVersion
	}{
		// Verifies that a plain major.minor.patch version is parsed correctly.
		{
			description: "plain version",
			input:       "2.49.0",
			expected:    source.PrometheusVersion{Major: 2, Minor: 49, Patch: 0},
		},
		// Verifies that a leading "v" prefix is stripped before parsing.
		{
			description: "v prefix",
			input:       "v3.5.1",
			expected:    source.PrometheusVersion{Major: 3, Minor: 5, Patch: 1},
		},
		// Verifies that pre-release suffixes are stripped before parsing.
		{
			description: "pre-release suffix",
			input:       "3.5.0-rc.1",
			expected:    source.PrometheusVersion{Major: 3, Minor: 5, Patch: 0},
		},
		// Verifies that a version with only two components is rejected.
		{
			description: "too few parts",
			input:       "2.49",
			err:         `failed to parse Prometheus version "2.49": expected major.minor.patch format`,
		},
		// Verifies that a non-numeric major component is rejected.
		{
			description: "bad major",
			input:       "abc.1.2",
			err:         `failed to parse Prometheus version "abc.1.2"`,
		},
		// Verifies that a non-numeric minor component is rejected.
		{
			description: "bad minor",
			input:       "2.abc.0",
			err:         `failed to parse Prometheus version "2.abc.0"`,
		},
		// Verifies that a non-numeric patch component is rejected.
		{
			description: "bad patch",
			input:       "2.49.abc",
			err:         `failed to parse Prometheus version "2.49.abc"`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			v, err := source.ParseVersion(tc.input)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, v)
			}
		})
	}
}
