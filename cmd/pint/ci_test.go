package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/config"
)

func TestDetectGithubActions(t *testing.T) {
	type testCaseT struct {
		envVars  map[string]string
		input    *config.GitHub
		expected *config.GitHub
		name     string
	}

	testCases := []testCaseT{
		{
			name:     "nil input no env vars returns nil",
			envVars:  map[string]string{},
			input:    nil,
			expected: nil,
		},
		{
			name: "nil input with GITHUB_REPOSITORY sets owner and repo",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "owner/repo",
			},
			input: nil,
			expected: &config.GitHub{
				Owner:   "owner",
				Repo:    "repo",
				Timeout: "1m0s",
			},
		},
		{
			name: "existing config not overwritten",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "env-owner/env-repo",
			},
			input: &config.GitHub{
				Owner: "config-owner",
				Repo:  "config-repo",
			},
			expected: &config.GitHub{
				Owner: "config-owner",
				Repo:  "config-repo",
			},
		},
		{
			name: "partial config gets filled from env",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "env-owner/env-repo",
			},
			input: &config.GitHub{
				Owner: "config-owner",
			},
			expected: &config.GitHub{
				Owner: "config-owner",
				Repo:  "env-repo",
			},
		},
		{
			name: "GITHUB_API_URL sets base and upload URI",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "owner/repo",
				"GITHUB_API_URL":    "https://api.github.example.com",
			},
			input: nil,
			expected: &config.GitHub{
				Owner:     "owner",
				Repo:      "repo",
				BaseURI:   "https://api.github.example.com",
				UploadURI: "https://api.github.example.com",
				Timeout:   "1m0s",
			},
		},
		{
			name: "existing URIs not overwritten",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "owner/repo",
				"GITHUB_API_URL":    "https://api.github.example.com",
			},
			input: &config.GitHub{
				BaseURI:   "https://custom.api.com",
				UploadURI: "https://custom.upload.com",
			},
			expected: &config.GitHub{
				Owner:     "owner",
				Repo:      "repo",
				BaseURI:   "https://custom.api.com",
				UploadURI: "https://custom.upload.com",
			},
		},
		{
			name: "GITHUB_REF sets PR number for pull_request event",
			envVars: map[string]string{
				"GITHUB_EVENT_NAME": "pull_request",
				"GITHUB_REF":        "refs/pull/123/merge",
				"GITHUB_REPOSITORY": "owner/repo",
			},
			input: nil,
			expected: &config.GitHub{
				Owner:   "owner",
				Repo:    "repo",
				Timeout: "1m0s",
			},
		},
		{
			name: "invalid GITHUB_REPOSITORY format",
			envVars: map[string]string{
				"GITHUB_REPOSITORY": "invalid",
			},
			input:    nil,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear relevant env vars
			for _, key := range []string{
				"GITHUB_PULL_REQUEST_NUMBER",
				"GITHUB_EVENT_NAME",
				"GITHUB_REF",
				"GITHUB_REPOSITORY",
				"GITHUB_API_URL",
			} {
				t.Setenv(key, "")
			}

			// Set test env vars
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			result := detectGithubActions(context.Background(), tc.input)

			if tc.expected == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				require.Equal(t, tc.expected.Owner, result.Owner)
				require.Equal(t, tc.expected.Repo, result.Repo)
				require.Equal(t, tc.expected.BaseURI, result.BaseURI)
				require.Equal(t, tc.expected.UploadURI, result.UploadURI)
			}
		})
	}
}
