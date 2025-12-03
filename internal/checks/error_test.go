package checks_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

func TestErrorCheck(t *testing.T) {
	testCases := []struct {
		description     string
		entry           *discovery.Entry
		expectedDetails string
	}{
		{
			description: "rule error with default details",
			entry: &discovery.Entry{
				Rule: parser.Rule{
					Error: parser.ParseError{
						Err:  errors.New("some error"),
						Line: 1,
					},
				},
			},
			expectedDetails: `This Prometheus rule is not valid.
This usually means that it's missing some required fields.`,
		},
		{
			description: "rule error with custom details",
			entry: &discovery.Entry{
				Rule: parser.Rule{
					Error: parser.ParseError{
						Err:     errors.New("some error"),
						Details: "custom error details",
						Line:    1,
					},
				},
			},
			expectedDetails: "custom error details",
		},
	}

	// This test doesn't use runTests() because ErrorCheck creates problems
	// with Diagnostics: nil for the default error case, while runTests() requires
	// all problems to have non-empty Diagnostics for snapshot testing.
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			c := checks.NewErrorCheck(tc.entry)
			problems := c.Check(context.Background(), tc.entry, nil)
			require.Len(t, problems, 1)
			require.Equal(t, tc.expectedDetails, problems[0].Details)
		})
	}
}
