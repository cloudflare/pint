package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlertsSettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf AlertsSettings
	}

	testCases := []testCaseT{
		{
			conf: AlertsSettings{
				Range: "7d",
			},
		},
		{
			conf: AlertsSettings{
				Step: "7d",
			},
		},
		{
			conf: AlertsSettings{
				Resolve: "7d",
			},
		},
		{
			conf: AlertsSettings{
				Range:   "foo",
				Step:    "1m",
				Resolve: "5m",
			},
			err: errors.New(`not a valid duration string: "foo"`),
		},
		{
			conf: AlertsSettings{
				Range:   "7d",
				Step:    "foo",
				Resolve: "5m",
			},
			err: errors.New(`not a valid duration string: "foo"`),
		},
		{
			conf: AlertsSettings{
				Range:   "7d",
				Step:    "1m",
				Resolve: "foo",
			},
			err: errors.New(`not a valid duration string: "foo"`),
		},
		{
			conf: AlertsSettings{
				Resolve:  "7d",
				Severity: "xxx",
			},
			err: errors.New("unknown severity: xxx"),
		},
		{
			conf: AlertsSettings{
				MinCount: -1,
			},
			err: errors.New("minCount cannot be < 0, got -1"),
		},
		{
			conf: AlertsSettings{
				MinCount: 0,
				Severity: "bug",
			},
			err: errors.New(`cannot set severity to "bug" when minCount is 0`),
		},
		{
			conf: AlertsSettings{
				MinCount: 5,
				Severity: "bug",
			},
		},
		{
			conf: AlertsSettings{
				MinCount: 0,
				Severity: "info",
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
