package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCISettings(t *testing.T) {
	type testCaseT struct {
		conf CI
		err  error
	}

	testCases := []testCaseT{
		{
			conf: CI{
				MaxCommits: 1,
				BaseBranch: "main",
				Include:    []string{"foo/.+"},
			},
		},
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
				MaxCommits: 20,
				Include:    []string{".+++"},
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
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
