package checks

import (
	"context"
	"fmt"
	"slices"

	"github.com/prometheus/common/model"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	FragileCheckName = "promql/fragile"

	FragileCheckSamplingDetails = `Alerts are identified by labels, two alerts with identical sets of labels are identical.
If two alerts have the same name but the rest of labels isn't 100% identical then they are two different alerts.
If the same alert query returns results that over time have different labels on them then previous alert instances will resolve and new alerts will be fired.
This can happen when using one of the aggregation operation like topk or bottomk as they can return a different time series each time they are evaluated.`
	FragileCheckPartialData = `This alerting rule performs arithmetic operation on results of two aggregations, this might cause false positive alerts when Prometheus restarts.
When Prometheus is started it doesn't scrape all targets at once, it spreads it over the first scrape interval.
Until it finishes scraping all target queries that use aggregation will return results calculated from only a subset of targets.
If each of these aggregates comes from a different scrape job then one aggregate might have data from more targets then the other.
The easiest way to avoid such issues is to add ` + "`for: 2m` to you alerting rule."
)

func NewFragileCheck() FragileCheck {
	return FragileCheck{}
}

type FragileCheck struct{}

func (c FragileCheck) Meta() CheckMeta {
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

func (c FragileCheck) String() string {
	return FragileCheckName
}

func (c FragileCheck) Reporter() string {
	return FragileCheckName
}

func (c FragileCheck) Check(_ context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	if entry.Rule.AlertingRule != nil {
		for _, src := range utils.LabelsSource(expr.Value.Value, expr.Query.Expr) {
			problems = append(problems, c.checkTopK(expr, src)...)
			problems = append(problems, c.checkPartialData(expr, src, entry.Rule.AlertingRule.For)...)
		}
	}

	return problems
}

func (c FragileCheck) checkTopK(expr parser.PromQLExpr, src utils.Source) (problems []Problem) {
	if src.Type != utils.AggregateSource {
		return problems
	}
	if src.FixedLabels && len(src.TransformedLabels(utils.PossibleLabel)) == 0 {
		return problems
	}
	if !slices.Contains([]string{"topk", "bottomk", "limit", "limit_ratio"}, src.Operation()) {
		return problems
	}
	problems = append(problems, Problem{
		Anchor:   AnchorAfter,
		Lines:    expr.Value.Pos.Lines(),
		Reporter: c.Reporter(),
		Summary:  "fragile query",
		Details:  FragileCheckSamplingDetails,
		Severity: Warning,
		Diagnostics: []diags.Diagnostic{
			{
				Message:     fmt.Sprintf("Using `%s` to select time series might return different set of time series on every query, which would cause flapping alerts.", src.Operation()),
				Pos:         expr.Value.Pos,
				FirstColumn: int(src.Position.Start) + 1,
				LastColumn:  int(src.Position.End),
				Kind:        diags.Issue,
			},
		},
	})
	return problems
}

func (c FragileCheck) checkPartialData(expr parser.PromQLExpr, src utils.Source, forVal *parser.YamlNode) (problems []Problem) {
	if src.Type != utils.AggregateSource {
		return problems
	}
	if !src.IsConditional {
		return problems
	}

	if forVal != nil {
		forDur, _ := model.ParseDuration(forVal.Value)
		if forDur > 0 {
			return problems
		}
	}

	for _, j := range src.Joins {
		// Only look for joins that are aggregations.
		if j.Src.Type != utils.AggregateSource {
			continue
		}
		// Ignore joins that are not conditional and instead are used to add labels.
		if len(j.AddedLabels) > 0 && !j.Src.IsConditional {
			continue
		}
		if j.Depth > 0 {
			continue
		}

		switch j.Op {
		case promParser.ADD:
		case promParser.SUB:
		case promParser.MUL:
		case promParser.DIV:
		case promParser.MOD:
		case promParser.POW:
		default:
			continue
		}

		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    expr.Value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "fragile query",
			Details:  FragileCheckPartialData,
			Severity: Warning,
			Diagnostics: []diags.Diagnostic{
				{
					Message:     "This query can cause false positives when Prometheus restarts, add `for` option to avoid that.",
					Pos:         expr.Value.Pos,
					FirstColumn: int(j.Src.Position.Start) + 1,
					LastColumn:  int(j.Src.Position.End),
					Kind:        diags.Issue,
				},
			},
		})
	}
	return problems
}
