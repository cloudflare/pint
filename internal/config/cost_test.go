package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
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
				BytesPerSample: -1,
				Severity:       "bug",
			},
			err: errors.New("bytesPerSample value must be >= 0"),
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
			err := tc.conf.validate()
			if err == nil || tc.err == nil {
				require.Equal(t, err, tc.err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}
