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
	ErrorCheckName = "error"

	yamlParseReporter   = "yaml/parse"
	ignoreFileReporter  = "ignore/file"
	pintCommentReporter = "pint/comment"

	yamlDetails = `This Prometheus rule is not valid.
This usually means that it's missing some required fields.`
)

func NewErrorCheck(err error) ErrorCheck {
	return ErrorCheck{err: err}
}

type ErrorCheck struct {
	err error
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
	return ErrorCheckName
}

func (c ErrorCheck) Reporter() string {
	return ErrorCheckName
}

func (c ErrorCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	var commentErr comments.CommentError
	var ignoreErr discovery.FileIgnoreError

	switch {
	case errors.As(c.err, &ignoreErr):
		slog.Debug("ignore/file report", slog.Any("err", ignoreErr))
		problems = append(problems, Problem{
			Lines: parser.LineRange{
				First: ignoreErr.Line,
				Last:  ignoreErr.Line,
			},
			Reporter: ignoreFileReporter,
			Text:     ignoreErr.Error(),
			Severity: Information,
		})

	case errors.As(c.err, &commentErr):
		slog.Debug("invalid comment report", slog.Any("err", commentErr))
		problems = append(problems, Problem{
			Lines: parser.LineRange{
				First: commentErr.Line,
				Last:  commentErr.Line,
			},
			Reporter: pintCommentReporter,
			Text:     "This comment is not a valid pint control comment: " + commentErr.Error(),
			Severity: Warning,
		})

	case c.err != nil:
		slog.Debug("yaml syntax report", slog.Any("err", c.err))
		problems = append(problems, Problem{
			Lines: parser.LineRange{
				First: 1,
				Last:  1,
			},
			Reporter: yamlParseReporter,
			Text:     fmt.Sprintf("YAML parser returned an error when reading this file: `%s`.", c.err),
			Details: `pint cannot read this file because YAML parser returned an error.
This usually means that you have an indention error or the file doesn't have the YAML structure required by Prometheus for [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) and [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) rules.
If this file is a template that will be rendered into valid YAML then you can instruct pint to ignore some lines using comments, see [pint docs](https://cloudflare.github.io/pint/ignoring.html).
`,
			Severity: Fatal,
		})

	case rule.Error.Err != nil:
		slog.Debug("rule error report", slog.Any("err", rule.Error.Err))
		details := yamlDetails
		if rule.Error.Details != "" {
			details = rule.Error.Details
		}
		problems = append(problems, Problem{
			Lines: parser.LineRange{
				First: rule.Error.Line,
				Last:  rule.Error.Line,
			},
			Reporter: yamlParseReporter,
			Text:     fmt.Sprintf("This rule is not a valid Prometheus rule: `%s`.", rule.Error.Err.Error()),
			Details:  details,
			Severity: Fatal,
		})
	}

	return problems
}
