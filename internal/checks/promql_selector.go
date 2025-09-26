package checks

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	SelectorCheckName = "promql/selector"
)

func NewSelectorCheck(keyRe, callRe *TemplatedRegexp, requiredName, comment string, severity Severity) SelectorCheck {
	return SelectorCheck{
		keyRe:        keyRe,
		callRe:       callRe,
		requiredName: requiredName,
		comment:      comment,
		severity:     severity,
	}
}

type SelectorCheck struct {
	keyRe        *TemplatedRegexp
	callRe       *TemplatedRegexp
	comment      string
	requiredName string
	severity     Severity
}

func (c SelectorCheck) Meta() CheckMeta {
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

func (c SelectorCheck) String() string {
	if c.callRe != nil {
		return fmt.Sprintf("%s(%s:%s:%s)", SelectorCheckName, c.callRe.anchored, c.keyRe.anchored, c.requiredName)
	}
	return fmt.Sprintf("%s(%s:%s)", SelectorCheckName, c.keyRe.anchored, c.requiredName)
}

func (c SelectorCheck) Reporter() string {
	return SelectorCheckName
}

func (c SelectorCheck) Check(_ context.Context, entry discovery.Entry, _ []discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError != nil {
		return problems
	}

	keyRe := c.keyRe.MustExpand(entry.Rule)

	var callRe *regexp.Regexp
	if c.callRe != nil {
		callRe = c.callRe.MustExpand(entry.Rule)
	}

	for _, src := range utils.LabelsSource(expr.Value.Value, expr.Query.Expr) {
		src.WalkSources(func(s utils.Source, _ *utils.Join, _ *utils.Unless) {
			problems = append(problems, c.checkSource(keyRe, callRe, s, expr.Value.Pos)...)
		})
	}

	return problems
}

func (c SelectorCheck) findSelector(callRe *regexp.Regexp, s utils.Source) (*promParser.VectorSelector, *promParser.Call) {
	var call *promParser.Call
	for i := len(s.Operations) - 1; i >= 0; i-- {
		op := s.Operations[i]
		if callRe != nil && call == nil {
			if cl, ok := op.Node.(*promParser.Call); ok {
				if callRe.MatchString(cl.Func.Name) {
					call = cl
					continue
				}
			}
		}
		if vs, ok := op.Node.(*promParser.VectorSelector); ok {
			if callRe == nil || call != nil {
				return vs, call
			}
		}
	}
	return nil, nil
}

func (c SelectorCheck) checkSource(keyRe, callRe *regexp.Regexp, s utils.Source, pos diags.PositionRanges) (problems []Problem) {
	vs, call := c.findSelector(callRe, s)
	if vs == nil {
		return problems
	}

	if vs.Name != "" && !keyRe.MatchString(vs.Name) {
		return problems
	}
	for _, lm := range vs.LabelMatchers {
		if lm.Name == labels.MetricName && !keyRe.MatchString(lm.Value) {
			return problems
		}
	}

	if !slices.ContainsFunc(vs.LabelMatchers, func(lm *labels.Matcher) bool {
		return lm.Name == c.requiredName
	}) {
		prefix := "This vector selector"
		if call != nil {
			prefix = fmt.Sprintf("Vector selectors inside the `%s()` function", call.Func.Name)
		}
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    pos.Lines(),
			Reporter: c.Reporter(),
			Summary:  "required matcher missing",
			Details:  maybeComment(c.comment),
			Diagnostics: []diags.Diagnostic{
				{
					Pos:         pos,
					FirstColumn: int(vs.PosRange.Start) + 1,
					LastColumn:  int(vs.PosRange.End),
					Message: fmt.Sprintf(
						"%s must specify `%s` label. Please add a `{%s=\"...\"} matcher.",
						prefix, c.requiredName, c.requiredName),
					Kind: diags.Issue,
				},
			},
			Severity: c.severity,
		})
	}
	return problems
}
