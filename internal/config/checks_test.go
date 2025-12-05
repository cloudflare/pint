package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChecksSettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf Checks
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
			err := tc.conf.validate()
			if err == nil || tc.err == nil {
				require.Equal(t, err, tc.err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}

func TestCheckMarshalJSONError(t *testing.T) {
	c := Check{Name: "invalid"}
	_, err := json.Marshal(c)
	require.EqualError(t, err, `json: error calling MarshalJSON for type config.Check: unknown check "invalid"`)
}
