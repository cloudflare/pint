package config_test

import (
	"context"
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
				RecordingRule: &parser.RecordingRule{
					Labels: &parser.YamlMap{
						Items: []*parser.YamlKeyValue{
							{
								Key:   &parser.YamlNode{Value: "cluster"},
								Value: &parser.YamlNode{Value: "prod"},
							},
						},
					},
				},
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "foo", Value: "bar"},
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				RecordingRule: &parser.RecordingRule{
					Labels: &parser.YamlMap{
						Items: []*parser.YamlKeyValue{
							{
								Key:   &parser.YamlNode{Value: "cluster"},
								Value: &parser.YamlNode{Value: "prod"},
							},
						},
					},
				},
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "foo", Value: "bar"},
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				RecordingRule: &parser.RecordingRule{
					Labels: &parser.YamlMap{
						Items: []*parser.YamlKeyValue{
							{
								Key:   &parser.YamlNode{Value: "cluster"},
								Value: &parser.YamlNode{Value: "prod"},
							},
						},
					},
				},
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "cluster", Value: "dev"},
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				RecordingRule: &parser.RecordingRule{
					Labels: &parser.YamlMap{
						Items: []*parser.YamlKeyValue{
							{
								Key:   &parser.YamlNode{Value: "cluster"},
								Value: &parser.YamlNode{Value: "prod"},
							},
						},
					},
				},
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "cluster", Value: "dev"},
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				RecordingRule: &parser.RecordingRule{
					Labels: &parser.YamlMap{
						Items: []*parser.YamlKeyValue{
							{
								Key:   &parser.YamlNode{Value: "cluster"},
								Value: &parser.YamlNode{Value: "prod"},
							},
						},
					},
				},
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "cluster", Value: "prod"},
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				RecordingRule: &parser.RecordingRule{
					Labels: &parser.YamlMap{
						Items: []*parser.YamlKeyValue{
							{
								Key:   &parser.YamlNode{Value: "cluster"},
								Value: &parser.YamlNode{Value: "prod"},
							},
						},
					},
				},
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "cluster", Value: "prod"},
			},
			isMatch: true,
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
		{
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{
					Annotations: &parser.YamlMap{
						Items: []*parser.YamlKeyValue{
							{
								Key:   &parser.YamlNode{Value: "cluster"},
								Value: &parser.YamlNode{Value: "prod"},
							},
						},
					},
				},
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "foo", Value: "bar"},
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{
					Annotations: &parser.YamlMap{
						Items: []*parser.YamlKeyValue{
							{
								Key:   &parser.YamlNode{Value: "cluster"},
								Value: &parser.YamlNode{Value: "prod"},
							},
						},
					},
				},
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "foo", Value: "bar"},
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{
					Annotations: &parser.YamlMap{
						Items: []*parser.YamlKeyValue{
							{
								Key:   &parser.YamlNode{Value: "cluster"},
								Value: &parser.YamlNode{Value: "prod"},
							},
						},
					},
				},
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "cluster", Value: "prod"},
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{
					Annotations: &parser.YamlMap{
						Items: []*parser.YamlKeyValue{
							{
								Key:   &parser.YamlNode{Value: "cluster"},
								Value: &parser.YamlNode{Value: "prod"},
							},
						},
					},
				},
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "cluster", Value: "prod"},
			},
			isMatch: true,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "cluster", Value: "prod"},
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Label: &config.MatchLabel{Key: "cluster", Value: "prod"},
			},
			isMatch: false,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Command: &config.LintCommand,
			},
			isMatch: true,
		},
		{
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Command: &config.WatchCommand,
			},
			isMatch: false,
		},
	}

	ctx := context.WithValue(context.Background(), config.CommandKey, config.LintCommand)
	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			assert := assert.New(t)
			isMatch := tc.match.IsMatch(ctx, tc.path, tc.rule)
			assert.Equal(tc.isMatch, isMatch)
		})
	}
}
