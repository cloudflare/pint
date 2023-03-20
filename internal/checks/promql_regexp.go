package checks

import (
	"context"
	"fmt"
	"regexp/syntax"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
)

const (
	RegexpCheckName = "promql/regexp"
)

func NewRegexpCheck() RegexpCheck {
	return RegexpCheck{}
}

type RegexpCheck struct{}

func (c RegexpCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: false}
}

func (c RegexpCheck) String() string {
	return RegexpCheckName
}

func (c RegexpCheck) Reporter() string {
	return RegexpCheckName
}

func (c RegexpCheck) Check(_ context.Context, _ string, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	done := map[string]struct{}{}
	for _, selector := range getSelectors(expr.Query) {
		if _, ok := done[selector.String()]; ok {
			continue
		}
		done[selector.String()] = struct{}{}
		for _, lm := range selector.LabelMatchers {
			if re := lm.GetRegexString(); re != "" {
				var isUseful bool
				var beginText, endText int
				r, _ := syntax.Parse(re, syntax.Perl)
				for _, s := range r.Sub {
					// nolint: exhaustive
					switch s.Op {
					case syntax.OpBeginText:
						beginText++
						continue
					case syntax.OpEndText:
						endText++
						continue
					case syntax.OpLiteral:
						continue
					case syntax.OpEmptyMatch:
						continue
					default:
						isUseful = true
					}
				}
				if !isUseful {
					var op labels.MatchType
					// nolint: exhaustive
					switch lm.Type {
					case labels.MatchRegexp:
						op = labels.MatchEqual
					case labels.MatchNotRegexp:
						op = labels.MatchNotEqual
					}
					problems = append(problems, Problem{
						Fragment: selector.String(),
						Lines:    expr.Lines(),
						Reporter: c.Reporter(),
						Text:     fmt.Sprintf(`unnecessary regexp match on static string %s, use %s%s%q instead`, lm, lm.Name, op, lm.Value),
						Severity: Bug,
					})
				}
				if beginText > 1 || endText > 1 {
					problems = append(problems, Problem{
						Fragment: selector.String(),
						Lines:    expr.Lines(),
						Reporter: c.Reporter(),
						Text: fmt.Sprintf(`prometheus regexp matchers are automatically fully anchored so match for %s will result in %s%s"^%s$", remove regexp anchors ^ and/or $`,
							lm, lm.Name, lm.Type, lm.Value,
						),
						Severity: Bug,
					})
				}
			}
		}
	}

	return problems
}
