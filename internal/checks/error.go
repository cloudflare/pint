package checks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cloudflare/pint/internal/comments"
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

func NewErrorCheck(entry discovery.Entry) ErrorCheck {
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
		IsOnline:      false,
		AlwaysEnabled: true,
	}
}

func (c ErrorCheck) String() string {
	return c.problem.Reporter
}

func (c ErrorCheck) Reporter() string {
	return c.problem.Reporter
}

func (c ErrorCheck) Check(_ context.Context, _ discovery.Path, _ parser.Rule, _ []discovery.Entry) (problems []Problem) {
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
		slog.Debug("ignore/file report", slog.Any("err", ignoreErr))
		return Problem{
			Lines: parser.LineRange{
				First: ignoreErr.Line,
				Last:  ignoreErr.Line,
			},
			Reporter: ignoreFileReporter,
			Text:     ignoreErr.Error(),
			Severity: Information,
		}

	case errors.As(err, &commentErr):
		slog.Debug("invalid comment report", slog.Any("err", commentErr))
		return Problem{
			Lines: parser.LineRange{
				First: commentErr.Line,
				Last:  commentErr.Line,
			},
			Reporter: pintCommentReporter,
			Text:     "This comment is not a valid pint control comment: " + commentErr.Error(),
			Severity: Warning,
		}

	case errors.As(err, &ownerErr):
		slog.Debug("invalid owner report", slog.Any("err", ownerErr))
		return Problem{
			Lines: parser.LineRange{
				First: ownerErr.Line,
				Last:  ownerErr.Line,
			},
			Reporter: discovery.RuleOwnerComment,
			Text:     fmt.Sprintf("This file is set as owned by `%s` but `%s` doesn't match any of the allowed owner values.", ownerErr.Name, ownerErr.Name),
			Severity: Bug,
		}

	case errors.As(err, &parseErr):
		slog.Debug("parse error", slog.Any("err", parseErr))
		return Problem{
			Lines: parser.LineRange{
				First: parseErr.Line,
				Last:  parseErr.Line,
			},
			Reporter: yamlParseReporter,
			Text:     parseErr.Err.Error(),
			Details: `pint cannot read this file because YAML parser returned an error.
This usually means that you have an indention error or the file doesn't have the YAML structure required by Prometheus for [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) and [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) rules.
If this file is a template that will be rendered into valid YAML then you can instruct pint to ignore some lines using comments, see [pint docs](https://cloudflare.github.io/pint/ignoring.html).
`,
			Severity: Fatal,
		}

	default:
		slog.Debug("rule error report", slog.Any("err", rule.Error.Err))
		details := yamlDetails
		if rule.Error.Details != "" {
			details = rule.Error.Details
		}
		return Problem{
			Lines: parser.LineRange{
				First: rule.Error.Line,
				Last:  rule.Error.Line,
			},
			Reporter: yamlParseReporter,
			Text:     fmt.Sprintf("This rule is not a valid Prometheus rule: `%s`.", rule.Error.Err.Error()),
			Details:  details,
			Severity: Fatal,
		}
	}
}
