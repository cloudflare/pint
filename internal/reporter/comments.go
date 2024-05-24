package reporter

import (
	"context"
	"slices"
	"strings"

	"github.com/cloudflare/pint/internal/checks"
)

type PendingCommentV2 struct {
	path   string
	text   string
	line   int
	anchor checks.Anchor
}

type ExistingCommentV2 struct {
	meta any
	path string
	text string
	line int
}

type Commenter interface {
	Describe() string
	Destinations(context.Context) ([]any, error)
	Summary(context.Context, any, Summary, []error) error
	List(context.Context, any) ([]ExistingCommentV2, error)
	Create(context.Context, any, PendingCommentV2) error
	Delete(context.Context, any, ExistingCommentV2) error
	CanCreate(int) (bool, error)
	IsEqual(ExistingCommentV2, PendingCommentV2) bool
}

func makeComments(summary Summary) (comments []PendingCommentV2) {
	var buf strings.Builder
	for _, reports := range dedupReports(summary.reports) {
		mergeDetails := identicalDetails(reports)

		buf.Reset()

		buf.WriteString(problemIcon(reports[0].Problem.Severity))
		buf.WriteString(" **")
		buf.WriteString(reports[0].Problem.Severity.String())
		buf.WriteString("** reported by [pint](https://cloudflare.github.io/pint/) **")
		buf.WriteString(reports[0].Problem.Reporter)
		buf.WriteString("** check.\n\n")
		for _, report := range reports {
			buf.WriteString("------\n\n")
			buf.WriteString(report.Problem.Text)
			buf.WriteString("\n\n")
			if !mergeDetails && report.Problem.Details != "" {
				buf.WriteString(report.Problem.Details)
				buf.WriteString("\n\n")
			}
			if report.Path.SymlinkTarget != report.Path.Name {
				buf.WriteString(":leftwards_arrow_with_hook: This problem was detected on a symlinked file ")
				buf.WriteRune('`')
				buf.WriteString(report.Path.Name)
				buf.WriteString("`.\n\n")
			}
		}
		if mergeDetails && reports[0].Problem.Details != "" {
			buf.WriteString("------\n\n")
			buf.WriteString(reports[0].Problem.Details)
			buf.WriteString("\n\n")
		}
		buf.WriteString("------\n\n")
		buf.WriteString(":information_source: To see documentation covering this check and instructions on how to resolve it [click here](https://cloudflare.github.io/pint/checks/")
		buf.WriteString(reports[0].Problem.Reporter)
		buf.WriteString(".html).\n")

		line := reports[0].Problem.Lines.Last
		for i := reports[0].Problem.Lines.Last; i >= reports[0].Problem.Lines.First; i-- {
			if slices.Contains(reports[0].ModifiedLines, i) {
				line = i
				break
			}
		}

		comments = append(comments, PendingCommentV2{
			anchor: reports[0].Problem.Anchor,
			path:   reports[0].Path.SymlinkTarget,
			line:   line,
			text:   buf.String(),
		})
	}
	return comments
}

func dedupReports(src []Report) (dst [][]Report) {
	for _, report := range src {
		index := -1
		for i, d := range dst {
			if d[0].Problem.Severity != report.Problem.Severity {
				continue
			}
			if d[0].Problem.Reporter != report.Problem.Reporter {
				continue
			}
			if d[0].Path.SymlinkTarget != report.Path.SymlinkTarget {
				continue
			}
			if d[0].Problem.Lines.First != report.Problem.Lines.First {
				continue
			}
			if d[0].Problem.Lines.Last != report.Problem.Lines.Last {
				continue
			}
			if d[0].Problem.Anchor != report.Problem.Anchor {
				continue
			}
			index = i
			break
		}
		if index < 0 {
			dst = append(dst, []Report{report})
			continue
		}
		// Skip this report if we have exact same message already
		if dst[index][0].Problem.Text == report.Problem.Text && dst[index][0].Problem.Details == report.Problem.Details {
			continue
		}
		dst[index] = append(dst[index], report)
	}
	return dst
}

func identicalDetails(src []Report) bool {
	if len(src) <= 1 {
		return false
	}
	var details string
	for _, report := range src {
		if details == "" {
			details = report.Problem.Details
			continue
		}
		if details != report.Problem.Details {
			return false
		}
	}
	return true
}

func problemIcon(s checks.Severity) string {
	// nolint:exhaustive
	switch s {
	case checks.Warning:
		return ":warning:"
	case checks.Information:
		return ":information_source:"
	default:
		return ":stop_sign:"
	}
}
