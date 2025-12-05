package promapi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDoRequestErrors(t *testing.T) {
	type testCaseT struct {
		desc      string
		uri       string
		path      string
		method    string
		safeURI   string
		expectErr bool
	}

	testCases := []testCaseT{
		{
			desc:    "valid URI",
			uri:     "http://example.com",
			safeURI: "http://example.com",
		},
		{
			desc:    "URI with user and password",
			uri:     "http://user:pass@example.com",
			safeURI: "http://user:xxx@example.com",
		},
		{
			desc:    "URI with user only",
			uri:     "http://user@example.com",
			safeURI: "http://user@example.com",
		},
		{
			desc:    "URI with invalid fragment",
			uri:     "http://example.com#%",
			safeURI: "http://example.com#%",
		},
		{
			desc:    "URI with invalid fragment escape",
			uri:     "http://example.com#%zz",
			safeURI: "http://example.com#%zz",
		},
		{
			desc:      "http.NewRequestWithContext error with invalid method",
			uri:       "http://example.com",
			path:      "/api/v1/query",
			method:    "INVALID METHOD",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.safeURI != "" {
				result := sanitizeURI(tc.uri)
				require.Equal(t, tc.safeURI, result)
			}
			if tc.expectErr {
				prom := &Prometheus{
					unsafeURI: tc.uri,
					timeout:   time.Second,
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_, err := prom.doRequest(ctx, tc.method, tc.path, nil)
				require.Error(t, err)
			}
		})
	}
}
