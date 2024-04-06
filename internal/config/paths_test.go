package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePaths(t *testing.T) {
	type testCaseT struct {
		err   error
		paths []string
	}

	testCases := []testCaseT{
		{
			paths: []string{"foo/.+"},
		},
		{
			paths: []string{".+++"},
			err:   errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.paths), func(t *testing.T) {
			err := ValidatePaths(tc.paths)
			if err == nil || tc.err == nil {
				require.Equal(t, err, tc.err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}
