package checks

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"

	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/parser/posrange"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/source"
)

const (
	NaNCheckName = "promql/nan"

	NaNCheckDetails = `Division by zero produces non-finite values in PromQL:

- ` + "`0 / 0`" + ` returns ` + "`NaN`" + `
- ` + "`1 / 0`" + ` returns ` + "`+Inf`" + `
- ` + "`x % 0`" + ` returns ` + "`NaN`" + `

Even if only one out of many input series is ` + "`NaN`" + ` or ` + "`Inf`" + `, the entire aggregation result
becomes ` + "`NaN`" + ` or ` + "`Inf`" + `, making all other valid series irrelevant.
To avoid this, filter out zero-valued divisor series before division, for example:
` + "`sum(foo / (bar > 0))`" + `
See [promql/nan](https://cloudflare.github.io/pint/checks/promql/nan.html) for more details.`
)

func NewNaNCheck() NaNCheck {
	return NaNCheck{}
}

type NaNCheck struct{}

func (c NaNCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		AlwaysEnabled: false,
	}
}

func (c NaNCheck) String() string {
	return NaNCheckName
}

func (c NaNCheck) Reporter() string {
	return NaNCheckName
}

func (c NaNCheck) Check(_ context.Context, entry *discovery.Entry, entries []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError() != nil {
		return problems
	}
	recordingRulesByName := c.collectRecordingRulesByName(entries)

	for _, src := range expr.Source() {
		src.WalkSources(func(s *source.Source, _ *source.Join, _ *source.Unless) {
			if s.Type != source.AggregateSource {
				return
			}
			// These aggregations combine input values into a single result,
			// so non-finite inputs can invalidate or skew the output.
			if aggr, ok := source.MostOuterOperation[*promParser.AggregateExpr](s); ok {
				switch aggr.Op {
				case promParser.SUM,
					promParser.AVG,
					promParser.STDDEV,
					promParser.STDVAR:
				default:
					return
				}
			}
			if divisorPos, ok := c.findUnsafeDivisor(s, true); ok {
				divisor := source.GetQueryFragment(expr.Value.Value, divisorPos)
				msg := "This aggregation can return NaN or Inf if any series for `" + divisor + "` evaluates to zero."
				problems = append(problems, c.reportFor(expr, msg, divisorPos))
				return
			}
			problems = append(problems, c.checkRecordingRules(expr, s, recordingRulesByName)...)
		})
	}

	return problems
}

// Look through recording rules that feed into this aggregation.
func (c NaNCheck) checkRecordingRules(
	expr *parser.PromQLExpr,
	s *source.Source,
	recordingRulesByName map[string][]*parser.PromQLExpr,
) (problems []Problem) {
	names := c.collectSelectorNames(s)
	if len(names) == 0 {
		return problems
	}

	for _, name := range slices.Sorted(maps.Keys(names)) {
		visited := map[string]bool{}
		if !c.recordingRuleHasUnsafeDivision(name, recordingRulesByName, visited) {
			continue
		}
		namePos := c.findNamePosition(expr.Value.Value, s.Position, name)
		msg := "This aggregation can return NaN or Inf because recording rule `" + name + "` can produce it."
		problems = append(problems, c.reportFor(expr, msg, namePos))
	}

	return problems
}

func (c NaNCheck) collectRecordingRulesByName(entries []*discovery.Entry) map[string][]*parser.PromQLExpr {
	recordingRulesByName := map[string][]*parser.PromQLExpr{}
	for _, e := range entries {
		if e.State == discovery.Removed {
			continue
		}
		if e.PathError != nil {
			continue
		}
		if e.Rule.Error.Err != nil {
			continue
		}
		if e.Rule.RecordingRule == nil {
			continue
		}
		if e.Rule.RecordingRule.Expr.SyntaxError() != nil {
			continue
		}
		name := e.Rule.RecordingRule.Record.Value
		recordingRulesByName[name] = append(recordingRulesByName[name], &e.Rule.RecordingRule.Expr)
	}
	return recordingRulesByName
}

func (c NaNCheck) recordingRuleHasUnsafeDivision(
	name string,
	recordingRulesByName map[string][]*parser.PromQLExpr,
	visited map[string]bool,
) bool {
	if visited[name] {
		return false
	}
	visited[name] = true

	for _, expr := range recordingRulesByName[name] {
		if c.sourceHasUnsafeDivision(expr) {
			return true
		}
		for next := range c.collectExprSelectorNames(expr) {
			if c.recordingRuleHasUnsafeDivision(next, recordingRulesByName, visited) {
				return true
			}
		}
	}

	return false
}

func (c NaNCheck) collectExprSelectorNames(expr *parser.PromQLExpr) map[string]bool {
	names := map[string]bool{}
	for _, src := range expr.Source() {
		for name := range c.collectSelectorNames(src) {
			names[name] = true
		}
	}
	return names
}

func (c NaNCheck) collectSelectorNames(s *source.Source) map[string]bool {
	names := map[string]bool{}
	s.WalkSources(func(inner *source.Source, _ *source.Join, _ *source.Unless) {
		if vs, ok := source.MostOuterOperation[*promParser.VectorSelector](inner); ok {
			names[vs.Name] = true
		}
	})
	return names
}

// findUnsafeDivisor reports whether s can produce NaN/Inf from division or
// modulo. On success it returns the position of the unsafe divisor.
//
// skipTopLevelJoin is true when checking an aggregation expression directly,
// so joins at depth 0 are ignored because they happen between aggregation
// results, for example `sum(foo) / sum(bar)`.
//
// skipTopLevelJoin is false when checking a recording rule used by an
// aggregation. In that case a top-level `foo / bar` in the recording rule is
// still unsafe, because the aggregation using the recording rule will consume
// its result.
func (c NaNCheck) findUnsafeDivisor(s *source.Source, skipTopLevelJoin bool) (posrange.PositionRange, bool) {
	for _, j := range s.Joins {
		if j.Op != promParser.DIV && j.Op != promParser.MOD {
			continue
		}
		// sum(foo) / sum(bar) - division between aggregation results, not inside
		// an aggregation input.
		if skipTopLevelJoin && j.Depth == 0 {
			continue
		}
		// Guarded: sum(foo / (bar > 0)), but not with bool modifier.
		if c.safeDivisorSource(j.Src) {
			continue
		}
		if vs, ok := source.MostOuterOperation[*promParser.VectorSelector](j.Src); ok {
			return vs.PosRange, true
		}
		return j.Src.Position, true
	}
	for _, ind := range s.Indirect {
		if ind.Op != promParser.DIV && ind.Op != promParser.MOD {
			continue
		}
		// sum(1 / bar) - scalar on the left, vector divides and can be zero.
		if ind.Side == source.LHS {
			if c.safeDivisorSource(s) {
				continue
			}
			vs, _ := source.MostOuterOperation[*promParser.VectorSelector](s)
			return vs.PosRange, true
		}
		if c.safeDivisorSource(ind.Src) {
			continue
		}
		return ind.Src.Position, true
	}
	return posrange.PositionRange{}, false
}

func (c NaNCheck) sourceHasUnsafeDivision(expr *parser.PromQLExpr) bool {
	for _, src := range expr.Source() {
		var found bool
		src.WalkSources(func(s *source.Source, _ *source.Join, _ *source.Unless) {
			if _, ok := c.findUnsafeDivisor(s, false); ok {
				found = true
			}
		})
		if found {
			return true
		}
	}
	return false
}

func (c NaNCheck) findNamePosition(expr string, within posrange.PositionRange, name string) posrange.PositionRange {
	fragment := source.GetQueryFragment(expr, within)
	idx := strings.Index(fragment, name)
	if idx < 0 {
		return within
	}
	return posrange.PositionRange{
		Start: within.Start + posrange.Pos(idx),
		End:   within.Start + posrange.Pos(idx+len(name)),
	}
}

func (c NaNCheck) reportFor(expr *parser.PromQLExpr, message string, highlight posrange.PositionRange) Problem {
	firstCol := int(highlight.Start) + 1
	lastCol := int(highlight.End)
	return Problem{
		Anchor:   AnchorAfter,
		Lines:    expr.Value.Pos.Lines(),
		Reporter: c.Reporter(),
		Summary:  "unsafe division in aggregation",
		Details:  NaNCheckDetails,
		Diagnostics: []diags.Diagnostic{
			{
				Message:     message,
				Pos:         expr.Value.Pos,
				Expr:        expr.Query().Expr,
				FirstColumn: firstCol,
				LastColumn:  lastCol,
				Kind:        diags.Issue,
			},
		},
		Severity: Warning,
	}
}

func (c NaNCheck) excludesZero(cond source.Condition) bool {
	if !cond.Present {
		return false
	}
	if !cond.KnownValue {
		return false
	}
	switch cond.Op {
	case promParser.GTR:
		return cond.Value >= 0
	case promParser.GTE:
		return cond.Value > 0
	case promParser.LSS:
		return cond.Value <= 0
	case promParser.LTE:
		return cond.Value < 0
	case promParser.NEQ:
		return cond.Value == 0
	default:
		return false
	}
}

func (c NaNCheck) parseClampArgs(op source.Operation) ([]float64, bool) {
	values := make([]float64, 0, len(op.Arguments))
	for _, arg := range op.Arguments {
		v, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return nil, false
		}
		values = append(values, v)
	}

	return values, true
}

func (c NaNCheck) clampExcludesZero(s *source.Source) bool {
	for _, op := range s.Operations {
		if op.Operation != "clamp" && op.Operation != "clamp_max" && op.Operation != "clamp_min" {
			continue
		}
		args, ok := c.parseClampArgs(op)
		if !ok {
			return false
		}

		switch op.Operation {
		case "clamp_min":
			return args[0] > 0
		case "clamp_max":
			return args[0] < 0
		case "clamp":
			return args[0] > 0 || args[1] < 0
		}
	}

	return false
}

func (c NaNCheck) safeDivisorSource(s *source.Source) bool {
	if c.excludesZero(s.Condition) && !s.ReturnInfo.IsReturnBool {
		return true
	}
	if s.ReturnInfo.KnownReturn && s.ReturnInfo.ReturnedNumber != 0 {
		return true
	}
	return c.clampExcludesZero(s)
}
