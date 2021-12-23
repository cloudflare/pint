package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChecksSettings(t *testing.T) {
	type testCaseT struct {
		conf Checks
		err  error
	}

	testCases := []testCaseT{
		{
			conf: Checks{},
		},
		{
			conf: Checks{
				Enabled: []string{"foo"},
			},
			err: errors.New("unknown check name foo"),
		},
		{
			conf: Checks{
				Disabled: []string{"foo"},
			},
			err: errors.New("unknown check name foo"),
		},
		{
			conf: Checks{
				Enabled:  []string{"promql/syntax"},
				Disabled: []string{"promql/syntax"},
			},
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
