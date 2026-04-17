package checks

import (
	"context"
	"time"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
)

const (
	GroupIntervalCheckName = "group/interval"
)

func NewGroupIntervalCheck() GroupIntervalCheck {
	return GroupIntervalCheck{}
}

type GroupIntervalCheck struct{}

func (c GroupIntervalCheck) Meta() CheckMeta {
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

func (c GroupIntervalCheck) String() string {
	return GroupIntervalCheckName
}

func (c GroupIntervalCheck) Reporter() string {
	return GroupIntervalCheckName
}

func (c GroupIntervalCheck) Check(_ context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	if entry.Group.Interval == nil {
		return nil
	}

	if entry.Group.Interval.Value <= time.Minute*5 {
		return nil
	}

	if entry.Rule.AlertingRule != nil && entry.Rule.AlertingRule.KeepFiringFor != nil && entry.Rule.AlertingRule.KeepFiringFor.ParseError == nil {
		if entry.Rule.AlertingRule.KeepFiringFor.Value >= entry.Group.Interval.Value {
			return nil
		}
	}

	problems = append(problems, Problem{
		Anchor:   AnchorAfter,
		Lines:    entry.Group.Interval.Pos.Lines(),
		Reporter: c.Reporter(),
		Summary:  "interval too long",
		Details:  "",
		Severity: Warning,
		Diagnostics: []diags.Diagnostic{
			{
				Message:     "Using group interval > 5m will cause gaps in recording rule results and flapping alerts.",
				Pos:         entry.Group.Interval.Pos,
				FirstColumn: 1,
				LastColumn:  entry.Group.Interval.Pos.Len(),
				Kind:        diags.Issue,
			},
		},
	})

	return problems
}
