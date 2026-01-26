package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAggregateSettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf AggregateSettings
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
				Name: "{{nil}}",
				Keep: []string{"foo"},
			},
			err: errors.New(`template: regexp:1:125: executing "regexp" at <nil>: nil is not a command`),
		},
		{
			conf: AggregateSettings{
				Name:     ".+",
				Keep:     []string{"foo"},
				Severity: "foo",
			},
			err: errors.New("unknown severity: foo"),
		},
		{
			conf: AggregateSettings{
				Name:  ".+",
				Strip: []string{"bar"},
			},
		},
		{
			conf: AggregateSettings{
				Name:     ".+",
				Keep:     []string{"foo"},
				Severity: "warning",
			},
		},
		// Regex label pattern tests
		{
			conf: AggregateSettings{
				Name: ".+",
				Keep: []string{"job_.+"},
			},
		},
		{
			conf: AggregateSettings{
				Name:  ".+",
				Strip: []string{".*instance.*"},
			},
		},
		{
			conf: AggregateSettings{
				Name: ".+",
				Keep: []string{".+++"},
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: AggregateSettings{
				Name:  ".+",
				Strip: []string{"{{nil}}"},
			},
			err: errors.New(`template: regexp:1:125: executing "regexp" at <nil>: nil is not a command`),
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
