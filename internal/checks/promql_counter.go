package checks

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	CounterCheckName    = "promql/counter"
	CounterCheckDetails = `[Counters](https://prometheus.io/docs/concepts/metric_types/#counter) track the number of events over time and so the value of a counter can only grow and never decrease.
This means that the absolute value of a counter doesn't matter, it will be a random number that depends on the number of events that happened since your application was started.
To use the value of a counter in PromQL you most likely want to calculate the rate of events using the [rate()](https://prometheus.io/docs/prometheus/latest/querying/functions/#rate) function, or any other function that is safe to use with counters.
Once you calculate the rate you can use that result in other functions or aggregations that are not counter safe, like [sum()](https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators).`
)

func NewCounterCheck(prom *promapi.FailoverGroup) CounterCheck {
	return CounterCheck{prom: prom}
}

type CounterCheck struct {
	prom *promapi.FailoverGroup
}

func (c CounterCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        true,
		AlwaysEnabled: false,
	}
}

func (c CounterCheck) String() string {
	return fmt.Sprintf("%s(%s)", CounterCheckName, c.prom.Name())
}

func (c CounterCheck) Reporter() string {
	return CounterCheckName
}

func (c CounterCheck) Check(ctx context.Context, entry discovery.Entry, _ []discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()

	if expr.SyntaxError != nil {
		return problems
	}

	done := map[string]struct{}{}

LOOP:
	for _, vs := range parser.WalkDownExpr[*promParser.VectorSelector](expr.Query) {
		if vs.Parent == nil {
			// This might be a counter but there's no parent so we have something like `expr: foo`.
			// We're only testing for the existence of foo in alerts OR copying it via recording rules.
			continue LOOP
		}

		for _, call := range parser.WalkUpExpr[*promParser.Call](vs.Parent) {
			if fn := call.Expr.(*promParser.Call); c.isSafeFunc(fn.Func.Name) {
				// This might be a counter but it's wrapped in one of the functions that make it
				// safe to use.
				continue LOOP
			}
		}

		for _, aggr := range parser.WalkUpExpr[*promParser.AggregateExpr](vs.Parent) {
			if ag := aggr.Expr.(*promParser.AggregateExpr); ag.Op == promParser.COUNT || ag.Op == promParser.GROUP {
				// This might be a counter but it's wrapped in count() or group() call so it's safe to use.
				continue LOOP
			}
		}

		for _, binSide := range parser.WalkUpParent[*promParser.BinaryExpr](vs) {
			if binExp := binSide.Parent.Expr.(*promParser.BinaryExpr); binExp.Op == promParser.LUNLESS {
				// We're inside a binary expression with `foo unless bar`.
				// Check which side we're at, if it's the RHS then it's safe to use counters directly.
				if binExp.RHS.String() == binSide.Expr.String() {
					continue LOOP
				}
			}
		}

		selector := vs.Expr.(*promParser.VectorSelector)
		if _, ok := done[selector.Name]; ok {
			// This selector was already checked, skip it.
			continue LOOP
		}

		metadata, err := c.prom.Metadata(ctx, selector.Name)
		if err != nil {
			if errors.Is(err, promapi.ErrUnsupported) {
				c.prom.DisableCheck(promapi.APIPathMetadata, c.Reporter())
				return problems
			}
			problems = append(problems, problemFromError(err, entry.Rule, c.Reporter(), c.prom.Name(), Warning))
			continue LOOP
		}
		if len(metadata.Metadata) == 0 {
			// No metadata so we don't know what type it uses.
			continue LOOP
		}
		for _, m := range metadata.Metadata {
			if m.Type != v1.MetricTypeCounter {
				// There's metadata with non-counter type, so it's not always a counter.
				continue LOOP
			}
		}
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    expr.Value.Pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "direct counter read",
			Details:  CounterCheckDetails,
			Severity: Warning,
			Diagnostics: []diags.Diagnostic{
				{
					Message: fmt.Sprintf("`%s` is a counter according to metrics metadata from %s, it can be dangarous to use its value directly.",
						selector.Name,
						promText(c.prom.Name(), metadata.URI),
					),
					Pos:         expr.Value.Pos,
					FirstColumn: int(selector.PosRange.Start) + 1,
					LastColumn:  int(selector.PosRange.End),
				},
			},
		})

		done[selector.Name] = struct{}{}
	}

	return problems
}

func (c CounterCheck) isSafeFunc(name string) bool {
	switch name {
	case "absent", "absent_over_time", "present_over_time":
		return true
	case "changes", "resets":
		return true
	case "count_over_time":
		return true
	case "increase":
		return true
	case "irate", "rate":
		return true
	case "timestamp":
		return true
	default:
		return false
	}
}
