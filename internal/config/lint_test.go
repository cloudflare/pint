package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLintSettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf Lint
	}

	testCases := []testCaseT{
		{
			conf: Lint{
				Include: []string{"foo/.+"},
				Exclude: []string{"foo/.+"},
			},
		},
		{
			conf: Lint{
				Include: []string{".+++"},
				Exclude: []string{"foo/.+"},
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: Lint{
				Include: []string{"foo/.+"},
				Exclude: []string{".+++"},
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
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
