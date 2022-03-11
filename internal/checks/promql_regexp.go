package checks

import (
	"context"
	"fmt"
	"regexp/syntax"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/cloudflare/pint/internal/parser"
)

const (
	RegexpCheckName = "promql/regexp"
)

func NewRegexpCheck() RegexpCheck {
	return RegexpCheck{}
}

type RegexpCheck struct{}

func (c RegexpCheck) String() string {
	return RegexpCheckName
}

func (c RegexpCheck) Reporter() string {
	return RegexpCheckName
}

func (c RegexpCheck) Check(ctx context.Context, rule parser.Rule) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	for _, selector := range getSelectors(expr.Query) {
		for _, lm := range selector.LabelMatchers {
			if s := lm.GetRegexString(); s != "" {
				var isUseful bool
				r, _ := syntax.Parse(s, syntax.Perl)
				for _, s := range r.Sub {
					switch s.Op {
					case syntax.OpBeginText:
						continue
					case syntax.OpEndText:
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
					var text string
					var op labels.MatchType
					switch lm.Type {
					case labels.MatchRegexp:
						op = labels.MatchEqual
					case labels.MatchNotRegexp:
						op = labels.MatchNotEqual
					}
					text = fmt.Sprintf(`unnecessary regexp match on static string %s, use %s%s%q instead`, lm, lm.Name, op, lm.Value)
					problems = append(problems, Problem{
						Fragment: selector.String(),
						Lines:    expr.Lines(),
						Reporter: c.Reporter(),
						Text:     text,
						Severity: Bug,
					})
				}
			}
		}
	}

	return
}
