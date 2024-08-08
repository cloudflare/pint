package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParserSettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf Parser
	}

	testCases := []testCaseT{
		{
			conf: Parser{},
		},
		{
			conf: Parser{
				Relaxed: []string{"foo.+"},
			},
		},
		{
			conf: Parser{
				Relaxed: []string{"(.+++)"},
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: Parser{
				Include: []string{"(.+++)"},
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: Parser{
				Exclude: []string{"(.+++)"},
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
