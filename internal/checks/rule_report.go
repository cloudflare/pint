package checks

import (
	"context"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
)

const (
	ReportCheckName = "rule/report"
)

func NewReportCheck(c string, s Severity) ReportCheck {
	return ReportCheck{comment: c, severity: s}
}

type ReportCheck struct {
	comment  string
	severity Severity
}

func (c ReportCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        false,
		AlwaysEnabled: false,
	}
}

func (c ReportCheck) String() string {
	return ReportCheckName
}

func (c ReportCheck) Reporter() string {
	return ReportCheckName
}

func (c ReportCheck) Check(_ context.Context, entry discovery.Entry, _ []discovery.Entry) (problems []Problem) {
	name := entry.Rule.NameNode()
	problems = append(problems, Problem{
		Anchor:   AnchorAfter,
		Lines:    entry.Rule.Lines,
		Reporter: c.Reporter(),
		Summary:  "problem reported by config rule",
		Details:  "",
		Severity: c.severity,
		Diagnostics: []diags.Diagnostic{
			{
				Message:     c.comment,
				Pos:         name.Pos,
				FirstColumn: 1,
				LastColumn:  len(name.Value),
				Kind:        diags.Issue,
			},
		},
	})
	return problems
}
