package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONReporterSettings(t *testing.T) {
	type testCaseT struct {
		conf JSONReporterSettings
		err  error
	}

	testCases := []testCaseT{
		{
			conf: JSONReporterSettings{Path: "out.json"},
		},
		{
			conf: JSONReporterSettings{},
			err:  errors.New("empty path"),
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
