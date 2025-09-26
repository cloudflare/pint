package options_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/config/options"
)

func TestSelectorSettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf options.SelectorSettings
	}

	testCases := []testCaseT{
		{
			conf: options.SelectorSettings{
				Key:            "summary",
				RequiredLabels: []string{"foo"},
			},
		},
		{
			conf: options.SelectorSettings{},
			err:  errors.New("selector key cannot be empty"),
		},
		{
			conf: options.SelectorSettings{
				Key: ".++",
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: options.SelectorSettings{
				Key:      ".+",
				Severity: "foo",
			},
			err: errors.New("unknown severity: foo"),
		},
		{
			conf: options.SelectorSettings{
				Key: "summary",
			},
			err: errors.New("requiredLabels cannot be empty"),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.conf), func(t *testing.T) {
			err := tc.conf.Validate()
			if err == nil || tc.err == nil {
				require.Equal(t, err, tc.err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}
