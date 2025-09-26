package options_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/config/options"
)

func TestCallSettings(t *testing.T) {
	type testCaseT struct {
		err  error
		conf options.CallSettings
	}

	testCases := []testCaseT{
		{
			conf: options.CallSettings{
				Key: "sum",
				Selectors: []options.SelectorSettings{
					{
						Key:            ".+",
						RequiredLabels: []string{"foo"},
					},
				},
			},
		},
		{
			conf: options.CallSettings{},
			err:  errors.New("call key cannot be empty"),
		},
		{
			conf: options.CallSettings{
				Key: ".++",
			},
			err: errors.New("error parsing regexp: invalid nested repetition operator: `++`"),
		},
		{
			conf: options.CallSettings{
				Key: ".+",
			},
			err: errors.New("you must specific at least one `selector` block"),
		},
		{
			conf: options.CallSettings{
				Key: ".+",
				Selectors: []options.SelectorSettings{
					{},
				},
			},
			err: errors.New("selector key cannot be empty"),
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
