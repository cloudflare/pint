package reporter

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
)

type Report struct {
	Path          discovery.Path
	Owner         string
	ModifiedLines []int
	Rule          parser.Rule
	Problem       checks.Problem
}

func (r Report) isEqual(nr Report) bool {
	if nr.Path.SymlinkTarget != r.Path.SymlinkTarget {
		return false
	}
	if nr.Path.Name != r.Path.Name {
		return false
	}
	if nr.Owner != r.Owner {
		return false
	}
	if r.Problem.Lines.First != nr.Problem.Lines.First {
		return false
	}
	if r.Problem.Lines.Last != nr.Rule.Lines.Last {
		return false
	}
	if !nr.Rule.IsSame(r.Rule) {
		return false
	}
	if nr.Problem.Reporter != r.Problem.Reporter {
		return false
	}
	if nr.Problem.Text != r.Problem.Text {
		return false
	}
	if nr.Problem.Severity != r.Problem.Severity {
		return false
	}
	return true
}

type Summary struct {
	reports        []Report
	OfflineChecks  int64
	OnlineChecks   int64
	Duration       time.Duration
	TotalEntries   int
	CheckedEntries int64
}

func NewSummary(reports []Report) Summary {
	return Summary{reports: reports}
}

func (s *Summary) Report(reps ...Report) {
	for _, r := range reps {
		if !s.hasReport(r) {
			s.reports = append(s.reports, r)
		}
	}
}

func (s Summary) hasReport(r Report) bool {
	for _, er := range s.reports {
		if er.isEqual(r) {
			return true
		}
	}
	return false
}

func (s *Summary) SortReports() {
	sort.SliceStable(s.reports, func(i, j int) bool {
		if s.reports[i].Path.SymlinkTarget != s.reports[j].Path.SymlinkTarget {
			return s.reports[i].Path.SymlinkTarget < s.reports[j].Path.SymlinkTarget
		}
		if s.reports[i].Path.Name != s.reports[j].Path.Name {
			return s.reports[i].Path.Name < s.reports[j].Path.Name
		}
		if s.reports[i].Problem.Lines.First != s.reports[j].Problem.Lines.First {
			return s.reports[i].Problem.Lines.First < s.reports[j].Problem.Lines.First
		}
		if s.reports[i].Problem.Reporter != s.reports[j].Problem.Reporter {
			return s.reports[i].Problem.Reporter < s.reports[j].Problem.Reporter
		}
		return s.reports[i].Problem.Text < s.reports[j].Problem.Text
	})
}

func (s Summary) Reports() (reports []Report) {
	return s.reports
}

func (s Summary) HasFatalProblems() bool {
	for _, r := range s.Reports() {
		if r.Problem.Severity == checks.Fatal {
			return true
		}
	}
	return false
}

func (s Summary) CountBySeverity() map[checks.Severity]int {
	m := map[checks.Severity]int{}
	for _, report := range s.Reports() {
		if _, ok := m[report.Problem.Severity]; !ok {
			m[report.Problem.Severity] = 0
		}
		m[report.Problem.Severity]++
	}
	return m
}

type Reporter interface {
	Submit(Summary) error
}

func Submit(ctx context.Context, s Summary, c Commenter) error {
	dsts, err := c.Destinations(ctx)
	if err != nil {
		return err
	}

	for _, dst := range dsts {
		if err = updateDestination(ctx, s, c, dst); err != nil {
			return err
		}
	}

	return nil
}

func updateDestination(ctx context.Context, s Summary, c Commenter, dst any) error {
	slog.Info("Listing existing comments", slog.String("reporter", c.Describe()))
	existingComments, err := c.List(ctx, dst)
	if err != nil {
		return err
	}

	var created int
	var ok bool
	var errs []error
	pendingComments := makeComments(s)
	for _, pending := range pendingComments {
		for _, existing := range existingComments {
			if c.IsEqual(existing, pending) {
				slog.Debug("Comment already exists",
					slog.String("reporter", c.Describe()),
					slog.String("path", pending.path),
					slog.Int("line", pending.line),
				)
				goto NEXTCreate
			}
		}
		slog.Debug("Comment doesn't exist yet",
			slog.String("reporter", c.Describe()),
			slog.String("path", pending.path),
			slog.Int("line", pending.line),
		)

		ok, err = c.CanCreate(created)
		if err != nil {
			errs = append(errs, err)
		}
		if !ok {
			slog.Debug("Cannot create new comment",
				slog.String("reporter", c.Describe()),
				slog.String("path", pending.path),
				slog.Int("line", pending.line),
			)
			goto NEXTCreate
		}

		if err := c.Create(ctx, dst, pending); err != nil {
			slog.Error("Failed to create a new comment",
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
			if c.IsEqual(existing, pending) {
				goto NEXTDelete
			}
		}
		if err := c.Delete(ctx, dst, existing); err != nil {
			slog.Error("Failed to delete a stale comment",
				slog.String("reporter", c.Describe()),
				slog.String("path", existing.path),
				slog.Int("line", existing.line),
				slog.Any("err", err),
			)
			errs = append(errs, err)
		}
	NEXTDelete:
	}

	slog.Info("Creating report summary",
		slog.String("reporter", c.Describe()),
		slog.Int("reports", len(s.reports)),
		slog.Int("online", int(s.OnlineChecks)),
		slog.Int("offline", int(s.OnlineChecks)),
		slog.String("duration", output.HumanizeDuration(s.Duration)),
		slog.Int("entries", s.TotalEntries),
		slog.Int("checked", int(s.CheckedEntries)),
	)
	if err := c.Summary(ctx, dst, s, errs); err != nil {
		return err
	}

	return nil
}
