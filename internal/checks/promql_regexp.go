package checks

import (
	"context"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	RegexpCheckName = "promql/regexp"

	RegexpCheckDetails = `See [Prometheus documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) for details on how vector selectors work.`
)

func NewRegexpCheck() RegexpCheck {
	return RegexpCheck{}
}

type RegexpCheck struct{}

func (c RegexpCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		IsOnline: false,
	}
}

func (c RegexpCheck) String() string {
	return RegexpCheckName
}

func (c RegexpCheck) Reporter() string {
	return RegexpCheckName
}

func (c RegexpCheck) Check(_ context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	done := map[string]struct{}{}
	for _, selector := range utils.HasVectorSelector(expr.Query) {
		if _, ok := done[selector.String()]; ok {
			continue
		}

		good := make([]*labels.Matcher, 0, len(selector.LabelMatchers))
		bad := make([]badMatcher, 0, len(selector.LabelMatchers))

		var name string
		for _, lm := range selector.LabelMatchers {
			if lm.Name == model.MetricNameLabel && lm.Type == labels.MatchEqual {
				name = lm.Value
				break
			}
		}
		done[selector.String()] = struct{}{}
		for _, lm := range selector.LabelMatchers {
			if lm.Type != labels.MatchRegexp && lm.Type != labels.MatchNotRegexp {
				good = append(good, lm)
				continue
			}

			// We follow Prometheus FastRegexMatcher logic here.
			// If the matcher string is a literal match then we keep it as is.
			// If it's not then it's a regexp match and we need to wrap it in ^...$.
			re := lm.GetRegexString()
			if regexp.QuoteMeta(re) != re {
				re = "^(?:" + re + ")$"
			}

			var hasFlags, isUseful, isWildcard, isLiteral, isBad bool
			var beginText, endText int
			r, _ := syntax.Parse(re, syntax.Perl)
			for _, s := range r.Sub {
				// If effective flags are different from default flags then we assume regexp is useful.
				// It could be case sensitive match.
				if s.Flags > 0 && s.Flags != syntax.Perl {
					hasFlags = true
				}
				// nolint: exhaustive
				switch s.Op {
				case syntax.OpBeginText:
					beginText++
				case syntax.OpEndText:
					endText++
				case syntax.OpLiteral:
					isLiteral = true
				case syntax.OpEmptyMatch:
					// pass
				case syntax.OpStar:
					isWildcard = true
				case syntax.OpPlus:
					isWildcard = true
					if !isUseful {
						isUseful = lm.Type == labels.MatchRegexp
					}
				default:
					isUseful = true
				}
			}
			if hasFlags && !isWildcard {
				isUseful = true
			}
			if isLiteral && isWildcard {
				isUseful = true
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
				bad = append(bad, badMatcher{lm: lm, op: op, isWildcard: isWildcard})
				isBad = true
			}
			if beginText > 1 || endText > 1 {
				bad = append(bad, badMatcher{lm: lm, badAnchor: true})
				isBad = true
			}
			if !isBad {
				good = append(good, lm)
			}
		}
		for _, b := range bad {
			var text string
			switch {
			case b.badAnchor:
				text = fmt.Sprintf("Prometheus regexp matchers are automatically fully anchored so match for `%s` will result in `%s%s\"^%s$\"`, remove regexp anchors `^` and/or `$`.",
					b.lm, b.lm.Name, b.lm.Type, b.lm.Value,
				)
			case b.isWildcard && b.op == labels.MatchEqual:
				text = fmt.Sprintf("Unnecessary wildcard regexp, simply use `%s` if you want to match on all `%s` values.",
					makeLabel(name, good...), b.lm.Name)
			case b.isWildcard && b.op == labels.MatchNotEqual:
				text = fmt.Sprintf("Unnecessary wildcard regexp, simply use `%s` if you want to match on all time series for `%s` without the `%s` label.",
					makeLabel(name, slices.Concat(good, []*labels.Matcher{{Type: labels.MatchEqual, Name: b.lm.Name, Value: ""}})...), name, b.lm.Name)
			default:
				text = fmt.Sprintf("Unnecessary regexp match on static string `%s`, use `%s%s%q` instead.",
					b.lm, b.lm.Name, b.op, b.lm.Value)

			}
			problems = append(problems, Problem{
				Lines:    expr.Value.Lines,
				Reporter: c.Reporter(),
				Text:     text,
				Details:  RegexpCheckDetails,
				Severity: Bug,
			})
		}
	}

	return problems
}

type badMatcher struct {
	lm         *labels.Matcher
	op         labels.MatchType
	isWildcard bool
	badAnchor  bool
}

func makeLabel(name string, matchers ...*labels.Matcher) string {
	filtered := make([]*labels.Matcher, 0, len(matchers))
	for _, m := range matchers {
		if m.Type == labels.MatchEqual && m.Name == labels.MetricName {
			continue
		}
		filtered = append(filtered, m)
	}
	if len(filtered) == 0 {
		return name
	}

	var b strings.Builder
	b.WriteString(name)
	b.WriteRune('{')
	for i, m := range filtered {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(m.String())
	}
	b.WriteRune('}')
	return b.String()
}
