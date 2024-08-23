package config_test

import (
	"context"
	"log/slog"
	"strconv"
	"testing"

	"github.com/neilotoole/slogt"

	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"

	"github.com/stretchr/testify/require"
)

func TestMatch(t *testing.T) {
	type testCaseT struct {
		match   config.Match
		cmd     config.ContextCommandVal
		path    string
		entry   discovery.Entry
		isMatch bool
	}

	testCases := []testCaseT{
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule:  parser.Rule{},
				State: discovery.Noop,
			},
			match:   config.Match{},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule:  parser.Rule{},
				State: discovery.Noop,
			},
			match: config.Match{
				Path: "bar.yaml",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule:  parser.Rule{},
				State: discovery.Noop,
			},
			match: config.Match{
				Path: "foo.yaml",
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule:  parser.Rule{},
				State: discovery.Noop,
			},
			match: config.Match{
				Path: ".+.yaml",
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule:  parser.Rule{},
				State: discovery.Noop,
			},
			match: config.Match{
				Path: "bar.+.yaml",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{Value: "Foo"},
					},
				},
				State: discovery.Noop,
			},
			match: config.Match{
				Name: "Foo",
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{Value: "Foo"},
					},
				},
				State: discovery.Noop,
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
			entry: discovery.Entry{
				Rule: parser.Rule{
					AlertingRule: &parser.AlertingRule{
						Alert: parser.YamlNode{Value: "Foo"},
					},
				},
				State: discovery.Noop,
			},
			match: config.Match{
				Name: "Bar",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
					RecordingRule: &parser.RecordingRule{
						Record: parser.YamlNode{Value: "Foo"},
					},
				},
				State: discovery.Noop,
			},
			match: config.Match{
				Name: "Bar",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
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
				State: discovery.Noop,
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "foo", Value: "bar"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
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
				State: discovery.Noop,
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "foo", Value: "bar"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
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
				State: discovery.Noop,
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "cluster", Value: "dev"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
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
				State: discovery.Noop,
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "cluster", Value: "dev"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
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
				State: discovery.Noop,
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "cluster", Value: "prod"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
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
				State: discovery.Noop,
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "cluster", Value: "prod"},
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
					AlertingRule: &parser.AlertingRule{},
				},
				State: discovery.Noop,
			},
			match: config.Match{
				Kind: "alerting",
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
					AlertingRule: &parser.AlertingRule{},
				},
				State: discovery.Noop,
			},
			match:   config.Match{},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
					AlertingRule: &parser.AlertingRule{},
				},
				State: discovery.Noop,
			},
			match: config.Match{
				Kind: "recording",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
					RecordingRule: &parser.RecordingRule{},
				},
				State: discovery.Noop,
			},
			match: config.Match{
				Kind: "recording",
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
					RecordingRule: &parser.RecordingRule{},
				},
				State: discovery.Noop,
			},
			match: config.Match{
				Kind: "alerting",
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
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
				State: discovery.Noop,
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "foo", Value: "bar"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
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
				State: discovery.Noop,
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "foo", Value: "bar"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
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
				State: discovery.Noop,
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "cluster", Value: "prod"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule: parser.Rule{
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
				State: discovery.Noop,
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "cluster", Value: "prod"},
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule:  parser.Rule{},
				State: discovery.Noop,
			},
			match: config.Match{
				Annotation: &config.MatchAnnotation{Key: "cluster", Value: "prod"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule:  parser.Rule{},
				State: discovery.Noop,
			},
			match: config.Match{
				Label: &config.MatchLabel{Key: "cluster", Value: "prod"},
			},
			isMatch: false,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule:  parser.Rule{},
				State: discovery.Noop,
			},
			match: config.Match{
				Command: &config.LintCommand,
			},
			isMatch: true,
		},
		{
			cmd:  config.LintCommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule:  parser.Rule{},
				State: discovery.Noop,
			},
			match: config.Match{
				Command: &config.WatchCommand,
			},
			isMatch: false,
		},
		{
			cmd:  config.CICommand,
			path: "foo.yaml",
			entry: discovery.Entry{
				Rule:  parser.Rule{},
				State: discovery.Noop,
			},
			match: config.Match{
				Command: &config.CICommand,
				Path:    "bar.yaml",
			},
			isMatch: false,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			slog.SetDefault(slogt.New(t))
			ctx := context.WithValue(context.Background(), config.CommandKey, tc.cmd)
			isMatch := tc.match.IsMatch(ctx, tc.path, tc.entry)
			require.Equal(t, tc.isMatch, isMatch)
		})
	}
}
