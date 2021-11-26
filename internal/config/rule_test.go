package config_test

import (
	"strconv"
	"testing"

	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/parser"

	"github.com/stretchr/testify/assert"
)

func TestMatch(t *testing.T) {
	type testCaseT struct {
		path    string
		rule    parser.Rule
		match   config.Match
		isMatch bool
	}

	testCases := []testCaseT{
		{
			path:    "foo.yaml",
			rule:    parser.Rule{},
			match:   config.Match{},
			isMatch: true,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Path: "bar.yaml",
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Path: "foo.yaml",
			},
			isMatch: true,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Path: ".+.yaml",
			},
			isMatch: true,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Path: "bar.+.yaml",
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{
					Alert: parser.YamlKeyValue{
						Key:   &parser.YamlNode{Value: "alert"},
						Value: &parser.YamlNode{Value: "Foo"},
					},
				},
			},
			match: config.Match{
				Name: "Foo",
			},
			isMatch: true,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{
					Alert: parser.YamlKeyValue{
						Key:   &parser.YamlNode{Value: "alert"},
						Value: &parser.YamlNode{Value: "Foo"},
					},
				},
			},
			match: config.Match{
				Name: "Foo",
				Path: "bar.yml",
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{
					Alert: parser.YamlKeyValue{
						Key:   &parser.YamlNode{Value: "alert"},
						Value: &parser.YamlNode{Value: "Foo"},
					},
				},
			},
			match: config.Match{
				Name: "Bar",
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				RecordingRule: &parser.RecordingRule{
					Record: parser.YamlKeyValue{
						Key:   &parser.YamlNode{Value: "record"},
						Value: &parser.YamlNode{Value: "Foo"},
					},
				},
			},
			match: config.Match{
				Name: "Bar",
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{},
			},
			match: config.Match{
				Kind: "alerting",
			},
			isMatch: true,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{},
			},
			match:   config.Match{},
			isMatch: true,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{},
			},
			match: config.Match{
				Kind: "recording",
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				RecordingRule: &parser.RecordingRule{},
			},
			match: config.Match{
				Kind: "recording",
			},
			isMatch: true,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				RecordingRule: &parser.RecordingRule{},
			},
			match: config.Match{
				Kind: "alerting",
			},
			isMatch: false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert := assert.New(t)
			isMatch := tc.match.IsMatch(tc.path, tc.rule)
			assert.Equal(tc.isMatch, isMatch)
		})
	}
}
