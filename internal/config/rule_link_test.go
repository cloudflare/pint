package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRuleLinkSettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf RuleLinkSettings
	}

	testCases := []testCaseT{
		{
			conf: RuleLinkSettings{
				Regex:    "foo",
				Severity: "bug",
			},
		},
		{
			conf: RuleLinkSettings{},
		},
		{
			conf: RuleLinkSettings{
				Regex: "foo.++",
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: RuleLinkSettings{
				Regex: "{{nil}}",
			},
			err: errors.New(
				`template: regexp:1:125: executing "regexp" at <nil>: nil is not a command`,
			),
		},
		{
			conf: RuleLinkSettings{
				Regex:    "foo",
				Severity: "bugx",
			},
			err: errors.New("unknown severity: bugx"),
		},
		{
			conf: RuleLinkSettings{
				Regex:   "foo",
				Timeout: "1m",
			},
		},
		{
			conf: RuleLinkSettings{
				Regex:   "foo",
				Timeout: "11f",
			},
			err: errors.New(`unknown unit "f" in duration "11f"`),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.conf), func(t *testing.T) {
			err := tc.conf.validate()
			if err == nil || tc.err == nil {
				require.Equal(t, err, tc.err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}
