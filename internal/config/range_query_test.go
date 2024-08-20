package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRangeQuerySettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf RangeQuerySettings
	}

	testCases := []testCaseT{
		{
			conf: RangeQuerySettings{
				Max: "foo",
			},
			err: errors.New(`not a valid duration string: "foo"`),
		},
		{
			conf: RangeQuerySettings{
				Max: "0h",
			},
			err: errors.New("range_query max value cannot be zero"),
		},
		{
			conf: RangeQuerySettings{
				Max:      "1d",
				Severity: "bag",
			},
			err: errors.New("unknown severity: bag"),
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
