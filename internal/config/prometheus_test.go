package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrometheusConfig(t *testing.T) {
	type testCaseT struct {
		conf PrometheusConfig
		err  error
	}

	testCases := []testCaseT{
		{
			conf: PrometheusConfig{
				Name:    "prom",
				URI:     "http://localhost",
				Timeout: "5m",
				Paths:   []string{"foo", "bar"},
			},
		},
		{
			conf: PrometheusConfig{
				Name:     "prom",
				URI:      "http://localhost",
				Failover: []string{"http://localhost", "http://localhost"},
				Timeout:  "5m",
				Paths:    []string{"foo", "bar"},
			},
		},
		{
			conf: PrometheusConfig{URI: "http://localhost"},
		},
		{
			conf: PrometheusConfig{},
			err:  errors.New("prometheus URI cannot be empty"),
		},
		{
			conf: PrometheusConfig{
				URI:     "http://localhost",
				Timeout: "foo",
			},
			err: errors.New(`not a valid duration string: "foo"`),
		},
		{
			conf: PrometheusConfig{
				Name:    "prom",
				URI:     "http://localhost",
				Timeout: "5m",
				Paths:   []string{"foo.++", "bar"},
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
