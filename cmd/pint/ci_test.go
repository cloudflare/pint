package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/config"
)

func TestDetectCurrentBranch(t *testing.T) {
	type testCaseT struct {
		envVars  map[string]string
		input    string
		expected string
		name     string
	}

	testCases := []testCaseT{
		{
			// Non-HEAD branch is returned as-is.
			name:     "real branch unchanged",
			envVars:  map[string]string{},
			input:    "feature/foo",
			expected: "feature/foo",
		},
		{
			// HEAD with GITHUB_HEAD_REF resolves to GitHub PR source branch.
			name: "HEAD resolved from GITHUB_HEAD_REF",
			envVars: map[string]string{
				"GITHUB_HEAD_REF": "fix/gh-auth",
			},
			input:    "HEAD",
			expected: "fix/gh-auth",
		},
		{
			// GITHUB_HEAD_REF takes priority over GitLab env vars.
			name: "GITHUB_HEAD_REF takes priority over GitLab env vars",
			envVars: map[string]string{
				"GITHUB_HEAD_REF":                     "fix/gh-auth",
				"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "fix/gl-auth",
			},
			input:    "HEAD",
			expected: "fix/gh-auth",
		},
		{
			// HEAD with CI_MERGE_REQUEST_SOURCE_BRANCH_NAME resolves to MR source branch.
			name: "HEAD resolved from CI_MERGE_REQUEST_SOURCE_BRANCH_NAME",
			envVars: map[string]string{
				"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "fix/auth",
			},
			input:    "HEAD",
			expected: "fix/auth",
		},
		{
			// HEAD with CI_COMMIT_BRANCH resolves when MR env var is absent.
			name: "HEAD resolved from CI_COMMIT_BRANCH",
			envVars: map[string]string{
				"CI_COMMIT_BRANCH": "develop",
			},
			input:    "HEAD",
			expected: "develop",
		},
		{
			// CI_MERGE_REQUEST_SOURCE_BRANCH_NAME takes priority over CI_COMMIT_BRANCH.
			name: "CI_MERGE_REQUEST_SOURCE_BRANCH_NAME takes priority",
			envVars: map[string]string{
				"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "fix/auth",
				"CI_COMMIT_BRANCH":                    "develop",
			},
			input:    "HEAD",
			expected: "fix/auth",
		},
		{
			// HEAD without any env vars stays HEAD.
			name:     "HEAD stays HEAD without env vars",
			envVars:  map[string]string{},
			input:    "HEAD",
			expected: "HEAD",
		},
		{
			// Non-HEAD branch ignores env vars even when they are set.
			name: "non-HEAD ignores env vars",
			envVars: map[string]string{
				"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME": "fix/auth",
			},
			input:    "main",
			expected: "main",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{
				"GITHUB_HEAD_REF",
				"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME",
				"CI_COMMIT_BRANCH",
			} {
				t.Setenv(key, "")
			}

			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			result := detectCurrentBranch(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestDetectBaseBranch(t *testing.T) {
	type testCaseT struct {
		envVars  map[string]string
		input    string
		expected string
		name     string
	}

	testCases := []testCaseT{
		{
			// No env vars set, baseBranch stays at config default.
			name:     "no env vars keeps config default",
			envVars:  map[string]string{},
			input:    "master",
			expected: "master",
		},
		{
			// GITHUB_BASE_REF overrides baseBranch for GitHub PRs.
			name: "GITHUB_BASE_REF overrides baseBranch",
			envVars: map[string]string{
				"GITHUB_BASE_REF": "develop",
			},
			input:    "master",
			expected: "develop",
		},
		{
			// CI_MERGE_REQUEST_TARGET_BRANCH_NAME overrides baseBranch for GitLab MRs.
			name: "CI_MERGE_REQUEST_TARGET_BRANCH_NAME overrides baseBranch",
			envVars: map[string]string{
				"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "release/v2",
			},
			input:    "master",
			expected: "release/v2",
		},
		{
			// GITHUB_BASE_REF takes priority over CI_MERGE_REQUEST_TARGET_BRANCH_NAME.
			name: "GITHUB_BASE_REF takes priority over GitLab env var",
			envVars: map[string]string{
				"GITHUB_BASE_REF":                     "develop",
				"CI_MERGE_REQUEST_TARGET_BRANCH_NAME": "release/v2",
			},
			input:    "master",
			expected: "develop",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{
				"GITHUB_BASE_REF",
				"CI_MERGE_REQUEST_TARGET_BRANCH_NAME",
			} {
				t.Setenv(key, "")
			}

			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			result := detectBaseBranch(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}

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
