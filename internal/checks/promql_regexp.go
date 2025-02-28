package checks

import (
	"context"
	"fmt"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"unicode"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser/posrange"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"
)

const (
	RegexpCheckName = "promql/regexp"

	RegexpCheckDetails = `See [Prometheus documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-series-selectors) for details on how vector selectors work.`
)

type PromqlRegexpSettings struct {
	Smelly        *bool `hcl:"smelly,optional" json:"smelly,omitempty"`
	smellyEnabled bool
}

func (c *PromqlRegexpSettings) Validate() error {
	c.smellyEnabled = true
	if c.Smelly != nil {
		c.smellyEnabled = *c.Smelly
	}
	return nil
}

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
		Online:        false,
		AlwaysEnabled: false,
	}
}

func (c RegexpCheck) String() string {
	return RegexpCheckName
}

func (c RegexpCheck) Reporter() string {
	return RegexpCheckName
}

func (c RegexpCheck) Check(ctx context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return nil
	}

	var settings *PromqlRegexpSettings
	if s := ctx.Value(SettingsKey(c.Reporter())); s != nil {
		settings = s.(*PromqlRegexpSettings)
	}
	if settings == nil {
		settings = &PromqlRegexpSettings{}
		_ = settings.Validate()
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
				re = "^(?s:" + re + ")$"
			}

			var hasFlags, isUseful, isWildcard, isLiteral, isBad, isSmelly, hasNonDigits bool
			var beginText, endText int
			var lastOp syntax.Op
			r, _ := syntax.Parse(re, syntax.Perl)
			for _, s := range r.Sub {
				// If effective flags are different from default flags then we assume regexp is useful.
				// It could be case sensitive match.
				if s.Flags > 0 && s.Flags != syntax.Perl {
					hasFlags = true
				}
				if isOpSmelly(s.Op, lastOp) && hasNonDigits {
					isSmelly = true
				}
				// nolint: exhaustive
				switch s.Op {
				case syntax.OpBeginText:
					beginText++
				case syntax.OpEndText:
					endText++
				case syntax.OpLiteral:
					for _, r := range s.Rune {
						if !unicode.IsDigit(r) {
							hasNonDigits = true
						}
					}
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
				lastOp = s.Op
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
				bad = append(bad, badMatcher{pos: selector.PosRange, lm: lm, op: op, isWildcard: isWildcard})
				isBad = true
			}
			if beginText > 1 || endText > 1 {
				bad = append(bad, badMatcher{pos: selector.PosRange, lm: lm, badAnchor: true})
				isBad = true
			}
			if settings.smellyEnabled && isSmelly {
				bad = append(bad, badMatcher{pos: selector.PosRange, lm: lm, isSmelly: true})
			}
			if !isBad {
				good = append(good, lm)
			}
		}
		for _, b := range bad {
			var summary, text string
			switch {
			case b.badAnchor:
				summary = "redundant regexp anchors"
				text = fmt.Sprintf("Prometheus regexp matchers are automatically fully anchored so match for `%s` will result in `%s%s\"^%s$\"`, remove regexp anchors `^` and/or `$`.",
					b.lm, b.lm.Name, b.lm.Type, b.lm.Value,
				)
			case b.isWildcard && b.op == labels.MatchEqual:
				summary = "unnecessary wildcard regexp"
				text = fmt.Sprintf("Use `%s` if you want to match on all `%s` values.",
					makeLabel(name, good...), b.lm.Name)
			case b.isWildcard && b.op == labels.MatchNotEqual:
				summary = "unnecessary negative wildcard regexp"
				text = fmt.Sprintf("Use `%s` if you want to match on all time series for `%s` without the `%s` label.",
					makeLabel(name, slices.Concat(good, []*labels.Matcher{{Type: labels.MatchEqual, Name: b.lm.Name, Value: ""}})...), name, b.lm.Name)
			case b.isSmelly:
				summary = "smelly regexp selector"
				text = fmt.Sprintf("`{%s}` looks like a smelly selector that tries to extract substrings from the value, please consider breaking down the value of this label into multiple smaller labels", b.lm.String())
			default:
				summary = "redundant regexp"
				text = fmt.Sprintf("Unnecessary regexp match on static string `%s`, use `%s%s%q` instead.",
					b.lm, b.lm.Name, b.op, b.lm.Value)

			}
			problems = append(problems, Problem{
				Lines:    expr.Value.Lines,
				Reporter: c.Reporter(),
				Summary:  summary,
				Details:  RegexpCheckDetails,
				Severity: Warning,
				Diagnostics: []output.Diagnostic{
					{
						Message:     text,
						Line:        expr.Value.Lines.First,
						FirstColumn: expr.Value.Column + int(b.pos.Start),
						LastColumn:  expr.Value.Column + int(b.pos.End) - 1,
					},
				},
			})
		}
	}

	return problems
}

type badMatcher struct {
	lm         *labels.Matcher
	pos        posrange.PositionRange
	op         labels.MatchType
	isWildcard bool
	isSmelly   bool
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

func isOpSmelly(a, b syntax.Op) bool {
	if a == syntax.OpLiteral && (b == syntax.OpStar || b == syntax.OpPlus) {
		return true
	}
	if b == syntax.OpLiteral && (a == syntax.OpStar || a == syntax.OpPlus) {
		return true
	}
	return false
}
