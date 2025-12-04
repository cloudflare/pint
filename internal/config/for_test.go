package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
)

func TestForSettingsValidate(t *testing.T) {
	type testCaseT struct {
		err  error
		conf ForSettings
	}

	testCases := []testCaseT{
		{
			conf: ForSettings{Min: "5m"},
		},
		{
			conf: ForSettings{Max: "10m"},
		},
		{
			conf: ForSettings{Min: "5m", Max: "10m"},
		},
		{
			conf: ForSettings{},
			err:  errors.New("must set either min or max option, or both"),
		},
		{
			conf: ForSettings{Min: "invalid"},
			err:  errors.New(`not a valid duration string: "invalid"`),
		},
		{
			conf: ForSettings{Max: "invalid"},
			err:  errors.New(`not a valid duration string: "invalid"`),
		},
		{
			conf: ForSettings{Min: "5m", Severity: "invalid"},
			err:  errors.New("unknown severity: invalid"),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.conf), func(t *testing.T) {
			err := tc.conf.validate()
			if err == nil || tc.err == nil {
				require.Equal(t, tc.err, err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}

func TestForSettingsGetSeverity(t *testing.T) {
	type testCaseT struct {
		conf     ForSettings
		fallback checks.Severity
		expected checks.Severity
	}

	testCases := []testCaseT{
		{
			conf:     ForSettings{Severity: "bug"},
			fallback: checks.Warning,
			expected: checks.Bug,
		},
		{
			conf:     ForSettings{Severity: "warning"},
			fallback: checks.Bug,
			expected: checks.Warning,
		},
		{
			conf:     ForSettings{},
			fallback: checks.Bug,
			expected: checks.Bug,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v/%s", tc.conf, tc.fallback), func(t *testing.T) {
			result := tc.conf.getSeverity(tc.fallback)
			require.Equal(t, tc.expected, result)
		})
	}
}
