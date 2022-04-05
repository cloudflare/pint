package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParserSettings(t *testing.T) {
	type testCaseT struct {
		conf Parser
		err  error
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
