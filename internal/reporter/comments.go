package reporter

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/output"
)

type PendingComment struct {
	path         string
	text         string
	line         int
	anchor       checks.Anchor
	modifiedLine bool
}

type ExistingComment struct {
	meta any
	path string
	text string
	line int
}

type Commenter interface {
	Describe() string
	Destinations(context.Context) ([]any, error)
	Summary(context.Context, any, Summary, []PendingComment, []error) error
	List(context.Context, any) ([]ExistingComment, error)
	Create(context.Context, any, PendingComment) error
	Delete(context.Context, any, ExistingComment) error
	CanCreate(int) bool
	CanDelete(ExistingComment) bool
	IsEqual(any, ExistingComment, PendingComment) bool
}

func NewCommentReporter(c Commenter, showDuplicates bool) CommentReporter {
	return CommentReporter{
		c:              c,
		showDuplicates: showDuplicates,
	}
}

type CommentReporter struct {
	c              Commenter
	showDuplicates bool
}

func (cr CommentReporter) Submit(ctx context.Context, summary Summary) (err error) {
	return Submit(ctx, summary, cr.c, cr.showDuplicates)
}

func makeComments(summary Summary, showDuplicates bool) (comments []PendingComment) {
	var buf strings.Builder
	var content string
	var err error
	for _, reports := range dedupReports(summary.reports, showDuplicates) {
		if reports[0].Problem.Anchor == checks.AnchorAfter {
			content, err = readFile(reports[0].Path.Name)
			if err != nil {
				content = ""
			}
		}

		buf.Reset()

		buf.WriteString(problemIcon(reports[0].Problem.Severity))
		buf.WriteString(" **")
		buf.WriteString(reports[0].Problem.Severity.String())
		buf.WriteString("** reported by [pint](https://cloudflare.github.io/pint/) **")
		buf.WriteString(reports[0].Problem.Reporter)
		buf.WriteString("** check.\n\n")
		for _, report := range reports {
			if len(report.Problem.Diagnostics) > 0 && content != "" {
				buf.WriteString("<details>\n")
				buf.WriteString("<summary>")
				buf.WriteString(report.Problem.Summary)
				buf.WriteString("</summary>\n\n")
				for _, diag := range report.Problem.Diagnostics {
					buf.WriteString("```yaml\n")
					buf.WriteString(diags.InjectDiagnostics(
						content,
						[]diags.Diagnostic{
							{
								Message:     "",
								Pos:         diag.Pos,
								FirstColumn: diag.FirstColumn,
								LastColumn:  diag.LastColumn,
								Kind:        diag.Kind,
							},
						},
						output.None,
					))
					buf.WriteString("```\n\n")
					buf.WriteString(diag.Message)
					buf.WriteString("\n\n")

				}
				if report.Problem.Details != "" {
					buf.WriteString(report.Problem.Details)
					buf.WriteString("\n\n")
				}
				buf.WriteString("</details>\n\n")
			} else {
				buf.WriteString("------\n\n")
				buf.WriteString(report.Problem.Summary)
				buf.WriteString("\n\n")
				if report.Problem.Details != "" {
					buf.WriteString("<details>\n")
					buf.WriteString("<summary>More information</summary>\n")
					buf.WriteString(report.Problem.Details)
					buf.WriteString("\n</details>\n\n")
				}
			}
			if report.Path.SymlinkTarget != report.Path.Name {
				buf.WriteString(":leftwards_arrow_with_hook: This problem was detected on a symlinked file ")
				buf.WriteRune('`')
				buf.WriteString(report.Path.Name)
				buf.WriteString("`.\n\n")
			}
		}
		if !showDuplicates && len(reports[0].Duplicates) > 0 {
			buf.WriteString("------\n\n")
			buf.WriteString("The same issue was reported ")
			buf.WriteString(strconv.Itoa(len(reports[0].Duplicates)))
			buf.WriteString(" more time(s), duplicates where suppressed.\n\n")
			buf.WriteString("<details>\n")
			buf.WriteString("<summary>Show affected rules</summary>\n\n")
			for _, dup := range reports[0].Duplicates {
				buf.WriteString("- `")
				buf.WriteString(dup.Rule.Name())
				buf.WriteString("` at `")
				buf.WriteString(dup.Path.Name)
				buf.WriteRune(':')
				buf.WriteString(strconv.Itoa(dup.Problem.Lines.First))
				buf.WriteString("`\n")
			}
			buf.WriteString("\n</details>\n\n")
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

		comments = append(comments, PendingComment{
			anchor:       reports[0].Problem.Anchor,
			path:         reports[0].Path.SymlinkTarget,
			line:         line,
			text:         buf.String(),
			modifiedLine: slices.Contains(reports[0].ModifiedLines, line),
		})
	}
	return comments
}

func dedupReports(src []Report, showDuplicates bool) (dst [][]Report) {
	for _, report := range src {
		if !showDuplicates && report.IsDuplicate {
			continue
		}

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
		if dst[index][0].Problem.Summary == report.Problem.Summary && dst[index][0].Problem.Details == report.Problem.Details {
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

func errsToComment(errs []error) string {
	var buf strings.Builder
	buf.WriteString("There were some errors when pint was trying to create a report.\n")
	buf.WriteString("Some review comments might be outdated or missing.\n")
	buf.WriteString("List of all errors:\n\n")
	for _, err := range errs {
		buf.WriteString("- `")
		buf.WriteString(err.Error())
		buf.WriteString("`\n")
	}
	return buf.String()
}

func Submit(ctx context.Context, s Summary, c Commenter, showDuplicates bool) error {
	slog.LogAttrs(ctx, slog.LevelInfo, "Will now report problems", slog.String("reporter", c.Describe()))
	dsts, err := c.Destinations(ctx)
	if err != nil {
		return err
	}

	for _, dst := range dsts {
		slog.LogAttrs(ctx, slog.LevelInfo, "Found a report destination", slog.String("reporter", c.Describe()), slog.Any("dst", dst))
		if err = updateDestination(ctx, s, c, dst, showDuplicates); err != nil {
			return err
		}
	}

	slog.LogAttrs(ctx, slog.LevelInfo, "Finished reporting problems", slog.String("reporter", c.Describe()))
	return nil
}

func updateDestination(ctx context.Context, s Summary, c Commenter, dst any, showDuplicates bool) (err error) {
	slog.LogAttrs(ctx, slog.LevelInfo, "Listing existing comments", slog.String("reporter", c.Describe()))
	existingComments, err := c.List(ctx, dst)
	if err != nil {
		return err
	}

	var created int
	var errs []error
	pendingComments := makeComments(s, showDuplicates)
	for _, pending := range pendingComments {
		slog.LogAttrs(ctx, slog.LevelDebug, "Got pending comment",
			slog.String("reporter", c.Describe()),
			slog.String("path", pending.path),
			slog.Int("line", pending.line),
			slog.String("msg", pending.text),
		)
		for _, existing := range existingComments {
			if c.IsEqual(dst, existing, pending) {
				slog.LogAttrs(ctx, slog.LevelDebug, "Comment already exists",
					slog.String("reporter", c.Describe()),
					slog.String("path", pending.path),
					slog.Int("line", pending.line),
				)
				goto NEXTCreate
			}
		}
		slog.LogAttrs(ctx, slog.LevelDebug, "Comment doesn't exist yet and needs to be created",
			slog.String("reporter", c.Describe()),
			slog.String("path", pending.path),
			slog.Int("line", pending.line),
		)

		if !c.CanCreate(created) {
			slog.LogAttrs(ctx, slog.LevelDebug, "Cannot create new comment",
				slog.String("reporter", c.Describe()),
				slog.String("path", pending.path),
				slog.Int("line", pending.line),
			)
			goto NEXTCreate
		}

		slog.LogAttrs(ctx, slog.LevelInfo, "Creating a new comment",
			slog.String("reporter", c.Describe()),
			slog.String("path", pending.path),
			slog.Int("line", pending.line),
		)
		if err := c.Create(ctx, dst, pending); err != nil {
			slog.LogAttrs(ctx, slog.LevelError, "Failed to create a new comment",
				slog.String("reporter", c.Describe()),
				slog.String("path", pending.path),
				slog.Int("line", pending.line),
				slog.Any("err", err),
			)
			return err
		}
		created++
	NEXTCreate:
	}

	for _, existing := range existingComments {
		for _, pending := range pendingComments {
			if c.IsEqual(dst, existing, pending) {
				goto NEXTDelete
			}
		}
		if !c.CanDelete(existing) {
			goto NEXTDelete
		}
		slog.LogAttrs(ctx, slog.LevelInfo, "Trying to delete a stale existing comment",
			slog.String("path", existing.path),
			slog.Int("line", existing.line),
		)
		if err := c.Delete(ctx, dst, existing); err != nil {
			slog.LogAttrs(ctx, slog.LevelError, "Failed to delete a stale comment",
				slog.String("reporter", c.Describe()),
				slog.String("path", existing.path),
				slog.Int("line", existing.line),
				slog.Any("err", err),
			)
			errs = append(errs, err)
		}
	NEXTDelete:
	}

	slog.LogAttrs(ctx, slog.LevelInfo, "Creating report summary",
		slog.String("reporter", c.Describe()),
		slog.Int("reports", len(s.reports)),
		slog.Int("online", int(s.OnlineChecks)),
		slog.Int("offline", int(s.OnlineChecks)),
		slog.String("duration", output.HumanizeDuration(s.Duration)),
		slog.Int("entries", s.TotalEntries),
		slog.Int("checked", int(s.CheckedEntries)),
	)
	if err := c.Summary(ctx, dst, s, pendingComments, errs); err != nil {
		return err
	}

	return nil
}

func makePrometheusDetailsComment(s Summary) string {
	pds := s.GetPrometheusDetails()
	if len(pds) == 0 {
		return ""
	}

	var buf strings.Builder
	buf.WriteString(`Some checks were disabled because one or more configured Prometheus server doesn't seem to support all required Prometheus APIs.
This usually means that you're running pint against a service like Thanos or Mimir that allows to query metrics but doesn't implement all APIs documented [here](https://prometheus.io/docs/prometheus/latest/querying/api/).
Since pint uses many of these API endpoint for querying information needed to run online checks only a real Prometheus server will allow it to run all of these checks.
Below is the list of checks that were disabled for each Prometheus server defined in pint config file.

`)
	for _, pd := range pds {
		buf.WriteString("- `")
		buf.WriteString(pd.Name)
		buf.WriteString("`\n")
		for _, dc := range pd.DisabledChecks {
			buf.WriteString("  - `")
			buf.WriteString(dc.API)
			buf.WriteString("` is unsupported, disabled checks:\n")
			for _, name := range dc.Checks {
				buf.WriteString("    - [")
				buf.WriteString(name)
				buf.WriteString("](https://cloudflare.github.io/pint/checks/")
				buf.WriteString(name)
				buf.WriteString(".html)\n")
			}
		}
	}
	return buf.String()
}
