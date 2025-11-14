package checks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	yamlParseReporter   = "yaml/parse"
	ignoreFileReporter  = "ignore/file"
	pintCommentReporter = "pint/comment"

	yamlDetails = `This Prometheus rule is not valid.
This usually means that it's missing some required fields.`
)

func NewErrorCheck(entry *discovery.Entry) ErrorCheck {
	return ErrorCheck{
		problem: parseRuleError(entry.Rule, entry.PathError),
	}
}

type ErrorCheck struct {
	problem Problem
}

func (c ErrorCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
			discovery.Removed,
		},
		Online:        false,
		AlwaysEnabled: true,
	}
}

func (c ErrorCheck) String() string {
	return c.problem.Reporter
}

func (c ErrorCheck) Reporter() string {
	return c.problem.Reporter
}

func (c ErrorCheck) Check(_ context.Context, _ *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	problems = append(problems, c.problem)
	return problems
}

func parseRuleError(rule parser.Rule, err error) Problem {
	var commentErr comments.CommentError
	var ownerErr comments.OwnerError
	var ignoreErr discovery.FileIgnoreError
	var parseErr parser.ParseError

	switch {
	case errors.As(err, &ignoreErr):
		slog.LogAttrs(context.Background(), slog.LevelDebug, "ignore/file report", slog.Any("err", ignoreErr))
		return Problem{
			Anchor: AnchorAfter,
			Lines: diags.LineRange{
				First: ignoreErr.Diagnostic.Pos.Lines().First,
				Last:  ignoreErr.Diagnostic.Pos.Lines().Last,
			},
			Reporter: ignoreFileReporter,
			Summary:  ignoreErr.Error(),
			Details:  "",
			Severity: Information,
			Diagnostics: []diags.Diagnostic{
				ignoreErr.Diagnostic,
			},
		}

	case errors.As(err, &commentErr):
		slog.LogAttrs(context.Background(), slog.LevelDebug, "invalid comment report", slog.Any("err", commentErr))
		return Problem{
			Anchor: AnchorAfter,
			Lines: diags.LineRange{
				First: commentErr.Diagnostic.Pos.Lines().First,
				Last:  commentErr.Diagnostic.Pos.Lines().Last,
			},
			Reporter: pintCommentReporter,
			Summary:  "invalid comment",
			Details:  "",
			Severity: Warning,
			Diagnostics: []diags.Diagnostic{
				commentErr.Diagnostic,
			},
		}

	case errors.As(err, &ownerErr):
		slog.LogAttrs(context.Background(), slog.LevelDebug, "invalid owner report", slog.Any("err", ownerErr))
		return Problem{
			Anchor: AnchorAfter,
			Lines: diags.LineRange{
				First: ownerErr.Diagnostic.Pos.Lines().First,
				Last:  ownerErr.Diagnostic.Pos.Lines().Last,
			},
			Reporter: discovery.RuleOwnerComment,
			Summary:  "invalid owner",
			Details:  "",
			Severity: Bug,
			Diagnostics: []diags.Diagnostic{
				ownerErr.Diagnostic,
			},
		}

	case errors.As(err, &parseErr):
		slog.LogAttrs(context.Background(), slog.LevelDebug, "parse error", slog.Any("err", parseErr))
		return Problem{
			Anchor: AnchorAfter,
			Lines: diags.LineRange{
				First: parseErr.Line,
				Last:  parseErr.Line,
			},
			Reporter: yamlParseReporter,
			Summary:  parseErr.Err.Error(),
			Details: `pint cannot read this file because YAML parser returned an error.
This usually means that you have an indention error or the file doesn't have the YAML structure required by Prometheus for [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) and [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) rules.
If this file is a template that will be rendered into valid YAML then you can instruct pint to ignore some lines using comments, see [pint docs](https://cloudflare.github.io/pint/ignoring.html).
`,
			Severity:    Fatal,
			Diagnostics: nil,
		}

	default:
		slog.LogAttrs(context.Background(), slog.LevelDebug, "rule error report", slog.Any("err", rule.Error.Err))
		details := yamlDetails
		if rule.Error.Details != "" {
			details = rule.Error.Details
		}
		return Problem{
			Anchor: AnchorAfter,
			Lines: diags.LineRange{
				First: rule.Error.Line,
				Last:  rule.Error.Line,
			},
			Reporter:    yamlParseReporter,
			Summary:     fmt.Sprintf("This rule is not a valid Prometheus rule: `%s`.", rule.Error.Err.Error()),
			Details:     details,
			Severity:    Fatal,
			Diagnostics: nil,
		}
	}
}
