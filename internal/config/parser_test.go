package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
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
		{
			conf: Parser{
				Schema: SchemaPrometheus,
			},
		},
		{
			conf: Parser{
				Schema: SchemaThanos,
			},
		},
		{
			conf: Parser{
				Schema: "xxx",
			},
			err: errors.New("unsupported parser schema: xxx"),
		},
		{
			conf: Parser{
				Names: "xxx",
			},
			err: errors.New("unsupported parser names: xxx"),
		},
		{
			conf: Parser{
				Names: NamesLegacy,
			},
		},
		{
			conf: Parser{
				Names: NamesUTF8,
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

func TestParserInitOptions(t *testing.T) {
	type testCaseT struct {
		// Describes the parser configuration being tested.
		description string
		conf        Parser
		expected    parser.Options
	}

	testCases := []testCaseT{
		{
			// Default parser with no configuration set produces UTF-8 names and Prometheus schema.
			description: "defaults",
			conf:        Parser{},
			expected: parser.Options{
				Names:    model.UTF8Validation,
				Schema:   parser.PrometheusSchema,
				IsStrict: false,
			},
		},
		{
			// Legacy names setting produces LegacyValidation.
			description: "legacy names",
			conf:        Parser{Names: NamesLegacy},
			expected: parser.Options{
				Names:    model.LegacyValidation,
				Schema:   parser.PrometheusSchema,
				IsStrict: false,
			},
		},
		{
			// Thanos schema setting produces ThanosSchema.
			description: "thanos schema",
			conf:        Parser{Schema: SchemaThanos},
			expected: parser.Options{
				Names:    model.UTF8Validation,
				Schema:   parser.ThanosSchema,
				IsStrict: false,
			},
		},
		{
			// Legacy names with thanos schema produces both overrides.
			description: "legacy names with thanos schema",
			conf:        Parser{Names: NamesLegacy, Schema: SchemaThanos},
			expected: parser.Options{
				Names:    model.LegacyValidation,
				Schema:   parser.ThanosSchema,
				IsStrict: false,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tc.conf.initOptions()
			require.Equal(t, tc.expected, tc.conf.Options())
		})
	}
}
