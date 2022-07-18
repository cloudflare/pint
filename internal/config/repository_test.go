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
			err:  errors.New(`project cannot be empty`),
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
		err  error
	}

	testCases := []testCaseT{
		{
			conf: GitHub{
				Repo:    "foo",
				Owner:   "bar",
				Timeout: "5m",
			},
		},
		{
			conf: GitHub{
				Repo:  "foo",
				Owner: "bar",
			},
		},
		{
			conf: GitHub{
				Repo:    "foo",
				Owner:   "bar",
				Timeout: "foo",
			},
			err: errors.New(`not a valid duration string: "foo"`),
		},
		{
			conf: GitHub{
				Owner:   "bar",
				Timeout: "5m",
			},
			err: errors.New("repo cannot be empty"),
		},
		{
			conf: GitHub{
				Repo:    "foo",
				Timeout: "5m",
			},
			err: errors.New("owner cannot be empty"),
		},
		{
			conf: GitHub{
				Repo:    "foo",
				Owner:   "bar",
				Timeout: "5m",
				BaseURI: "http://%41:8080/",
			},
			err: errors.New(`invalid baseuri: parse "http://%41:8080/": invalid URL escape "%41"`),
		},
		{
			conf: GitHub{
				Repo:      "foo",
				Owner:     "bar",
				Timeout:   "5m",
				UploadURI: "http://%41:8080/",
			},
			err: errors.New(`invalid uploaduri: parse "http://%41:8080/": invalid URL escape "%41"`),
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
