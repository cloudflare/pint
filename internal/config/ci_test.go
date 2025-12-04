package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCISettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf CI
	}

	testCases := []testCaseT{
		{
			conf: CI{
				MaxCommits: -5,
			},
			err: errors.New("maxCommits cannot be <= 0"),
		},
		{
			conf: CI{
				MaxCommits: 0,
			},
			err: errors.New("maxCommits cannot be <= 0"),
		},
		{
			conf: CI{
				MaxCommits: 10,
				BaseBranch: "main",
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
