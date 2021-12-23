package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAggregateSettings(t *testing.T) {
	type testCaseT struct {
		conf AggregateSettings
		err  error
	}

	testCases := []testCaseT{
		{
			conf: AggregateSettings{},
			err:  errors.New("empty name regex"),
		},
		{
			conf: AggregateSettings{
				Name: "foo",
			},
			err: errors.New("must specify keep or strip list"),
		},
		{
			conf: AggregateSettings{
				Name: ".+",
				Keep: []string{"foo"},
			},
		},
		{
			conf: AggregateSettings{
				Name: ".+++",
				Keep: []string{"foo"},
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: AggregateSettings{
				Name:     ".+",
				Keep:     []string{"foo"},
				Severity: "foo",
			},
			err: errors.New("unknown severity: foo"),
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
