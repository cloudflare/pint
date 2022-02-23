package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRejectSettings(t *testing.T) {
	type testCaseT struct {
		conf RejectSettings
		err  error
	}

	testCases := []testCaseT{
		{
			conf: RejectSettings{
				Regex:    "foo",
				Severity: "bug",
			},
		},
		{
			conf: RejectSettings{},
		},
		{
			conf: RejectSettings{
				Regex: "foo.++",
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: RejectSettings{
				Regex: "{{nil}}",
			},
			err: errors.New(`template: regexp:1:125: executing "regexp" at <nil>: nil is not a command`),
		},
		{
			conf: RejectSettings{
				Regex:    "foo",
				Severity: "bugx",
			},
			err: errors.New("unknown severity: bugx"),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.conf), func(t *testing.T) {
			assert := assert.New(t)
			err := tc.conf.validate()
			if err == nil || tc.err == nil {
				assert.Equal(err, tc.err)
			} else {
				assert.EqualError(err, tc.err.Error())
			}
		})
	}
}
