package config_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/parser"

	"github.com/stretchr/testify/require"
)

func TestMatch(t *testing.T) {
	type testCaseT struct {
		cmd     config.ContextCommandVal
		path    string
		rule    parser.Rule
		match   config.Match
		isMatch bool
	}

	testCases := []testCaseT{
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{},

			match:   config.Match{},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Path: "bar.yaml",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Path: "foo.yaml",
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Path: ".+.yaml",
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Path: "bar.+.yaml",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{
					Alert: parser.YamlNode{Value: "Foo"},
				},
			},
			match: config.Match{
				Name: "Foo",
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{
					Alert: parser.YamlNode{Value: "Foo"},
				},
			},
			match: config.Match{
				Name: "Foo",
				Path: "bar.yml",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{
					Alert: parser.YamlNode{Value: "Foo"},
				},
			},
			match: config.Match{
				Name: "Bar",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{
				RecordingRule: &parser.RecordingRule{
					Record: parser.YamlNode{Value: "Foo"},
				},
			},
			match: config.Match{
				Name: "Bar",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{
				AlertingRule: &parser.AlertingRule{},
			},

			match:   config.Match{},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
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
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "cluster", Value: "prod"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Label: &config.MatchLabel{Key: "cluster", Value: "prod"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Command: &config.LintCommand,
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Command: &config.WatchCommand,
			},
			isMatch: false,
		},
		{
			cmd:  config.CICommand,
			path: "foo.yaml",
			rule: parser.Rule{},
			match: config.Match{
				Command: &config.CICommand,
				Path:    "bar.yaml",
			},
			isMatch: false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			ctx := context.WithValue(context.Background(), config.CommandKey, tc.cmd)
			isMatch := tc.match.IsMatch(ctx, tc.path, tc.rule)
			require.Equal(t, tc.isMatch, isMatch)
		})
	}
}
