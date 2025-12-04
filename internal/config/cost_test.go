package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCostSettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf CostSettings
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
		{
			conf: CostSettings{
				MaxPeakSamples: -1,
			},
			err: errors.New("maxPeakSamples value must be >= 0"),
		},
		{
			conf: CostSettings{
				MaxTotalSamples: -1,
			},
			err: errors.New("maxTotalSamples value must be >= 0"),
		},
		{
			conf: CostSettings{
				MaxEvaluationDuration: "1abc",
			},
			err: errors.New(`unknown unit "abc" in duration "1abc"`),
		},
		{
			conf: CostSettings{
				MaxEvaluationDuration: "5m",
				Severity:              "warning",
			},
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
