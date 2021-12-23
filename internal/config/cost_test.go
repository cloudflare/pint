package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCostSettings(t *testing.T) {
	type testCaseT struct {
		conf CostSettings
		err  error
	}

	testCases := []testCaseT{
		{
			conf: CostSettings{},
		},
		{
			conf: CostSettings{
				MaxSeries: -1,
				Severity:  "bug",
			},
			err: errors.New("maxSeries value must be >= 0"),
		},
		{
			conf: CostSettings{
				Severity: "foo",
			},
			err: errors.New("unknown severity: foo"),
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
