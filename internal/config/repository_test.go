package config

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBitBucketSettings(t *testing.T) {
	type testCaseT struct {
		conf BitBucket
		err  error
	}

	testCases := []testCaseT{
		{
			conf: BitBucket{
				URI:        "http://localhost",
				Timeout:    "5m",
				Project:    "foo",
				Repository: "foo",
			},
		},
		{
			conf: BitBucket{},
			err:  errors.New(`empty duration string`),
		},
		{
			conf: BitBucket{
				Timeout: "foo",
			},
			err: errors.New(`not a valid duration string: "foo"`),
		},
		{
			conf: BitBucket{
				URI:        "http://localhost",
				Timeout:    "5m",
				Repository: "foo",
			},
			err: errors.New("project cannot be empty"),
		},
		{
			conf: BitBucket{
				URI:     "http://localhost",
				Timeout: "5m",
				Project: "foo",
			},
			err: errors.New("repository cannot be empty"),
		},
		{
			conf: BitBucket{
				URI:        "",
				Timeout:    "5m",
				Project:    "foo",
				Repository: "bar",
			},
			err: errors.New("uri cannot be empty"),
		},
		{
			conf: BitBucket{
				URI:        "http://localhost",
				Timeout:    "abc",
				Project:    "foo",
				Repository: "bar",
			},
			err: errors.New(`not a valid duration string: "abc"`),
		},
		{
			conf: BitBucket{
				URI:         "http://localhost",
				Timeout:     "5m",
				MaxComments: -1,
				Project:     "foo",
				Repository:  "bar",
			},
			err: errors.New("maxComments cannot be negative"),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.conf), func(t *testing.T) {
			err := tc.conf.validate()
			if err == nil || tc.err == nil {
				require.Equal(t, tc.err, err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}

func TestGitHubSettings(t *testing.T) {
	type testCaseT struct {
		conf GitHub
		env  map[string]string
		err  error
	}

	testCases := []testCaseT{
		{
			conf: GitHub{
				Repo:    "foo",
				Owner:   "bar",
				Timeout: "5m",
			},
			env: map[string]string{"GITHUB_REPOSITORY": ""},
		},
		{
			conf: GitHub{
				Repo:  "foo",
				Owner: "bar",
			},
			env: map[string]string{"GITHUB_REPOSITORY": ""},
			err: errors.New(`empty duration string`),
		},
		{
			conf: GitHub{
				Repo:    "foo",
				Owner:   "bar",
				Timeout: "foo",
			},
			env: map[string]string{"GITHUB_REPOSITORY": ""},
			err: errors.New(`not a valid duration string: "foo"`),
		},
		{
			conf: GitHub{
				Owner:   "bar",
				Timeout: "5m",
			},
			env: map[string]string{"GITHUB_REPOSITORY": ""},
			err: errors.New("repo cannot be empty"),
		},
		{
			conf: GitHub{
				Repo:    "foo",
				Timeout: "5m",
			},
			env: map[string]string{"GITHUB_REPOSITORY": ""},
			err: errors.New("owner cannot be empty"),
		},
		{
			conf: GitHub{
				Repo:    "foo",
				Owner:   "bar",
				Timeout: "5m",
				BaseURI: "http://%41:8080/",
			},
			env: map[string]string{"GITHUB_REPOSITORY": ""},
			err: errors.New(`invalid baseuri: parse "http://%41:8080/": invalid URL escape "%41"`),
		},
		{
			conf: GitHub{
				Repo:      "foo",
				Owner:     "bar",
				Timeout:   "5m",
				UploadURI: "http://%41:8080/",
			},
			env: map[string]string{"GITHUB_REPOSITORY": ""},
			err: errors.New(`invalid uploaduri: parse "http://%41:8080/": invalid URL escape "%41"`),
		},
		{
			conf: GitHub{},
			env:  map[string]string{"GITHUB_REPOSITORY": "xxx"},
			err:  errors.New("GITHUB_REPOSITORY is set, but with an invalid repository format: xxx"),
		},
		{
			conf: GitHub{},
			env:  map[string]string{"GITHUB_REPOSITORY": "/foo"},
			err:  errors.New("owner cannot be empty"),
		},
		{
			conf: GitHub{},
			env:  map[string]string{"GITHUB_REPOSITORY": "foo/"},
			err:  errors.New("repo cannot be empty"),
		},
		{
			conf: GitHub{
				Owner:       "bob",
				Repo:        "foo",
				Timeout:     "5m",
				MaxComments: -1,
			},
			err: errors.New("maxComments cannot be negative"),
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.conf), func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			err := tc.conf.validate()
			if err == nil || tc.err == nil {
				require.Equal(t, tc.err, err)
			} else {
				require.EqualError(t, err, tc.err.Error())
			}
		})
	}
}
