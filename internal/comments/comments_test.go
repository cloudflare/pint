package comments_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/diags"

	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	type testCaseT struct {
		input  string
		output []comments.Comment
	}

	parseUntil := func(s string) time.Time {
		until, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return until
	}

	errUntil := func(s string) error {
		_, err := time.Parse("2006-01-02", s)
		require.Error(t, err)
		return err
	}

	testCases := []testCaseT{
		{
			input: "code\n",
		},
		{
			input: "code # bob\n",
		},
		{
			input: "code # bob\ncode # alice\n",
		},
		{
			input: "# pint   bamboozle me this",
		},
		{
			input: "# pint/xxx   bamboozle me this",
		},
		{
			input: "# pint  bambo[]ozle me this",
		},
		{
			input: "# pint ignore/file \t this file",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: unexpected comment suffix: "this file"`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  30,
									},
								},
								FirstColumn: 1,
								LastColumn:  30,
							},
						},
					},
				},
			},
		},
		{
			input: "# pint ignore/file",
			output: []comments.Comment{
				{Type: comments.IgnoreFileType},
			},
		},
		{
			input: "# pint ignore/line this line",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: unexpected comment suffix: "this line"`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  28,
									},
								},
								FirstColumn: 1,
								LastColumn:  28,
							},
						},
					},
				},
			},
		},
		{
			input: "# pint ignore/line",
			output: []comments.Comment{
				{Type: comments.IgnoreLineType},
			},
		},
		{
			input: "# pint ignore/begin here",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: unexpected comment suffix: "here"`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  24,
									},
								},
								FirstColumn: 1,
								LastColumn:  24,
							},
						},
					},
				},
			},
		},
		{
			input: "# pint ignore/begin",
			output: []comments.Comment{
				{Type: comments.IgnoreBeginType},
			},
		},
		{
			input: "# pint ignore/end here",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: unexpected comment suffix: "here"`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  22,
									},
								},
								FirstColumn: 1,
								LastColumn:  22,
							},
						},
					},
				},
			},
		},
		{
			input: "# pint ignore/end",
			output: []comments.Comment{
				{Type: comments.IgnoreEndType},
			},
		},
		{
			input: "#   pint ignore/next-line\there",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: unexpected comment suffix: "here"`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  30,
									},
								},
								FirstColumn: 1,
								LastColumn:  30,
							},
						},
					},
				},
			},
		},
		{
			input: "# pint ignore/next-line",
			output: []comments.Comment{
				{Type: comments.IgnoreNextLineType},
			},
		},
		{
			input: "#   pint file/owner",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: missing file/owner value`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  19,
									},
								},
								FirstColumn: 1,
								LastColumn:  19,
							},
						},
					},
				},
			},
		},
		{
			input: "# pint file/owner bob and alice",
			output: []comments.Comment{
				{
					Type: comments.FileOwnerType,
					Value: comments.Owner{
						Name: "bob and alice",
						Line: 1,
					},
				},
			},
		},
		{
			input: "#   pint rule/owner",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: missing rule/owner value`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  19,
									},
								},
								FirstColumn: 1,
								LastColumn:  19,
							},
						},
					},
				},
			},
		},
		{
			input: "# pint rule/owner bob and alice",
			output: []comments.Comment{
				{
					Type: comments.RuleOwnerType,
					Value: comments.Owner{
						Name: "bob and alice",
					},
				},
			},
		},
		{
			input: "#   pint file/disable",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: missing file/disable value`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  21,
									},
								},
								FirstColumn: 1,
								LastColumn:  21,
							},
						},
					},
				},
			},
		},
		{
			input: `# pint file/disable promql/series(http_errors_total{label="this has spaces"})`,
			output: []comments.Comment{
				{
					Type:  comments.FileDisableType,
					Value: comments.Disable{Match: `promql/series(http_errors_total{label="this has spaces"})`},
				},
			},
		},
		{
			input: "#   pint disable",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: missing disable value`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  16,
									},
								},
								FirstColumn: 1,
								LastColumn:  16,
							},
						},
					},
				},
			},
		},
		{
			input: `# pint disable promql/series(http_errors_total{label="this has spaces"})`,
			output: []comments.Comment{
				{
					Type:  comments.DisableType,
					Value: comments.Disable{Match: `promql/series(http_errors_total{label="this has spaces"})`},
				},
			},
		},
		{
			input: `# pint disable promql/series(http_errors_total{label="this has spaces and a # symbol"})`,
			output: []comments.Comment{
				{
					Type:  comments.DisableType,
					Value: comments.Disable{Match: `promql/series(http_errors_total{label="this has spaces and a # symbol"})`},
				},
			},
		},
		{
			input: "#   pint file/snooze",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: missing file/snooze value`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  20,
									},
								},
								FirstColumn: 1,
								LastColumn:  20,
							},
						},
					},
				},
			},
		},
		{
			input: "#   pint file/snooze 2023-12-31",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: invalid snooze comment, expected '$TIME $MATCH' got "2023-12-31"`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  31,
									},
								},
								FirstColumn: 1,
								LastColumn:  31,
							},
						},
					},
				},
			},
		},
		{
			input: "#   pint file/snooze abc",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: invalid snooze comment, expected '$TIME $MATCH' got "abc"`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  24,
									},
								},
								FirstColumn: 1,
								LastColumn:  24,
							},
						},
					},
				},
			},
		},
		{
			input: `# pint file/snooze 2023-1231 promql/series(http_errors_total{label="this has spaces"})`,
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{Err: comments.CommentError{
						Diagnostic: diags.Diagnostic{
							Message: fmt.Sprintf("This comment is not a valid pint control comment: invalid snooze timestamp: %s", errUntil("2023-1231")),
							Pos: diags.PositionRanges{
								{
									Line:        1,
									FirstColumn: 1,
									LastColumn:  86,
								},
							},
							FirstColumn: 1,
							LastColumn:  86,
						},
					}},
				},
			},
		},
		{
			input: `# pint file/snooze 2023-12-31 promql/series(http_errors_total{label="this has spaces"})`,
			output: []comments.Comment{
				{
					Type: comments.FileSnoozeType,
					Value: comments.Snooze{
						Until: parseUntil("2023-12-31T00:00:00Z"),
						Match: `promql/series(http_errors_total{label="this has spaces"})`,
					},
				},
			},
		},
		{
			input: "#   pint snooze",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: missing snooze value`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  15,
									},
								},
								FirstColumn: 1,
								LastColumn:  15,
							},
						},
					},
				},
			},
		},
		{
			input: "#   pint snooze 2023-12-31",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: invalid snooze comment, expected '$TIME $MATCH' got "2023-12-31"`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  26,
									},
								},
								FirstColumn: 1,
								LastColumn:  26,
							},
						},
					},
				},
			},
		},
		{
			input: "#   pint snooze abc",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: invalid snooze comment, expected '$TIME $MATCH' got "abc"`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  19,
									},
								},
								FirstColumn: 1,
								LastColumn:  19,
							},
						},
					},
				},
			},
		},
		{
			input: `# pint snooze 2023-1231 promql/series(http_errors_total{label="this has spaces"})`,
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{Err: comments.CommentError{
						Diagnostic: diags.Diagnostic{
							Message: fmt.Sprintf("This comment is not a valid pint control comment: invalid snooze timestamp: %s", errUntil("2023-1231")),
							Pos: diags.PositionRanges{
								{
									Line:        1,
									FirstColumn: 1,
									LastColumn:  81,
								},
							},
							FirstColumn: 1,
							LastColumn:  81,
						},
					}},
				},
			},
		},
		{
			input: `# pint snooze 2023-12-31 promql/series(http_errors_total{label="this has spaces"})`,
			output: []comments.Comment{
				{
					Type: comments.SnoozeType,
					Value: comments.Snooze{
						Until: parseUntil("2023-12-31T00:00:00Z"),
						Match: `promql/series(http_errors_total{label="this has spaces"})`,
					},
				},
			},
		},
		{
			input: `# pint snooze 2023-12-31 promql/series(http_errors_total{label="this has    spaces"})`,
			output: []comments.Comment{
				{
					Type: comments.SnoozeType,
					Value: comments.Snooze{
						Until: parseUntil("2023-12-31T00:00:00Z"),
						Match: `promql/series(http_errors_total{label="this has    spaces"})`,
					},
				},
			},
		},
		{
			input: "#   pint rule/set",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: missing rule/set value`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 1,
										LastColumn:  17,
									},
								},
								FirstColumn: 1,
								LastColumn:  17,
							},
						},
					},
				},
			},
		},
		{
			input: "# pint rule/set bob and alice",
			output: []comments.Comment{
				{
					Type:  comments.RuleSetType,
					Value: comments.RuleSet{Value: "bob and alice"},
				},
			},
		},
		{
			input: "code # pint disable xxx  \ncode # alice\n",
			output: []comments.Comment{
				{
					Type:   comments.DisableType,
					Value:  comments.Disable{Match: "xxx"},
					Offset: len("code "),
				},
			},
		},
		{
			input: "code # pint disable xxx yyy \n # pint\tfile/owner bob",
			output: []comments.Comment{
				{
					Type:   comments.DisableType,
					Value:  comments.Disable{Match: "xxx yyy"},
					Offset: len("code "),
				},
				{
					Type: comments.FileOwnerType,
					Value: comments.Owner{
						Name: "bob",
						Line: 2,
					},
					Offset: 1,
				},
			},
		},
		{
			input: "# pint rule/set promql/series(found) min-age foo",
			output: []comments.Comment{
				{
					Type:  comments.RuleSetType,
					Value: comments.RuleSet{Value: "promql/series(found) min-age foo"},
				},
			},
		},
		{
			input: "{#- comment #} # pint ignore/line",
			output: []comments.Comment{
				{
					Type:   comments.IgnoreLineType,
					Offset: len("{#- comment #} "),
				},
			},
		},
		{
			input: "{# comment #} # pint ignore/line",
			output: []comments.Comment{
				{
					Type:   comments.IgnoreLineType,
					Offset: len("{# comment #} "),
				},
			},
		},
		{
			input: "#pint # pint # pint boo # pint ignore/line",
			output: []comments.Comment{
				{
					Type:   comments.IgnoreLineType,
					Offset: len("#pint # pint # pint boo "),
				},
			},
		},
		{
			input: "{# comment #} # pint ignore/line # pint ignore/file",
			output: []comments.Comment{
				{
					Type: comments.InvalidComment,
					Value: comments.Invalid{
						Err: comments.CommentError{
							Diagnostic: diags.Diagnostic{
								Message: `This comment is not a valid pint control comment: unexpected comment suffix: "# pint ignore/file"`,
								Pos: diags.PositionRanges{
									{
										Line:        1,
										FirstColumn: 15,
										LastColumn:  51,
									},
								},
								FirstColumn: 1,
								LastColumn:  51,
							},
						},
					},
					Offset: 14,
				},
			},
		},
		{
			input: "{#- JIRA-12345: foo<->bar example comment ' -#} # pint ignore/line",
			output: []comments.Comment{
				{
					Type:   comments.IgnoreLineType,
					Offset: len("{#- JIRA-12345: foo<->bar example comment ' -#} "),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			output := comments.Parse(1, tc.input)
			require.Equal(t, tc.output, output)
		})
	}
}

func TestCommentValueString(t *testing.T) {
	type testCaseT struct {
		comment  comments.CommentValue
		expected string
	}

	parseUntil := func(s string) time.Time {
		until, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return until
	}

	testCases := []testCaseT{
		{
			comment: comments.Invalid{Err: comments.CommentError{
				Diagnostic: diags.Diagnostic{
					Message: "foo bar",
					Pos: diags.PositionRanges{
						{
							Line:        1,
							FirstColumn: 1,
							LastColumn:  100,
						},
					},
					FirstColumn: 1,
					LastColumn:  100,
				},
			}},
			expected: "foo bar",
		},
		{
			comment: comments.Invalid{Err: comments.CommentError{
				Diagnostic: diags.Diagnostic{
					Message: "foo bar",
					Pos: diags.PositionRanges{
						{
							Line:        1,
							FirstColumn: 1,
							LastColumn:  100,
						},
					},
					FirstColumn: 1,
					LastColumn:  100,
				},
			}},
			expected: "foo bar",
		},
		{
			comment:  comments.Owner{Name: "bob & alice"},
			expected: "bob & alice",
		},
		{
			comment:  comments.Disable{Match: `promql/series({code="500"})`},
			expected: `promql/series({code="500"})`,
		},
		{
			comment:  comments.RuleSet{Value: "bob & alice"},
			expected: "bob & alice",
		},
		{
			comment:  comments.Snooze{Match: `promql/series({code="500"})`, Until: parseUntil("2023-11-28T00:00:00Z")},
			expected: `2023-11-28T00:00:00Z promql/series({code="500"})`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			require.Equal(t, tc.expected, tc.comment.String())
		})
	}
}
