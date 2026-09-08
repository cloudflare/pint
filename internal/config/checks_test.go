package config

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
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

	// SemanticError randomizes its modal verb, so compare all exported fields.
	semanticErr, ok := errors.AsType[*json.SemanticError](err)
	require.True(t, ok)
	require.Equal(t, int64(0), semanticErr.ByteOffset)
	require.Equal(t, jsontext.Pointer(""), semanticErr.JSONPointer)
	require.Equal(t, jsontext.Kind(0), semanticErr.JSONKind)
	require.Equal(t, jsontext.Value(nil), semanticErr.JSONValue)
	require.EqualError(t, semanticErr.Err, `unknown check "invalid"`)
}
