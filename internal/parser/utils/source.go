package utils

import (
	"fmt"
	"slices"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/parser/posrange"
)

type SourceType int

const (
	UnknownSource SourceType = iota
	NumberSource
	StringSource
	SelectorSource
	FuncSource
	AggregateSource
)

type ExcludedLabel struct {
	Reason   string
	Fragment string
}

type Source struct {
	Selector         *promParser.VectorSelector
	Call             *promParser.Call
	ExcludeReason    map[string]ExcludedLabel // Reason why a label was excluded
	Operation        string
	Returns          promParser.ValueType
	IncludedLabels   []string // Labels that are included by filters, they will be present if exist on source series (by).
	ExcludedLabels   []string // Labels guaranteed to be excluded from the results (without).
	GuaranteedLabels []string // Labels guaranteed to be present on the results (matchers).
	alternatives     []Source // Alternative lable sources
	Type             SourceType
	FixedLabels      bool // Labels are fixed and only allowed labels can be present.
}

func LabelsSource(expr string, node promParser.Node) (src []Source) {
	s := walkNode(expr, node)
	src = make([]Source, 0, len(s.alternatives)+1)
	src = append(src, s)
	src = append(src, s.alternatives...)
	return src
}

func walkNode(expr string, node promParser.Node) (s Source) {
	switch n := node.(type) {
	case *promParser.AggregateExpr:
		switch n.Op {
		case promParser.SUM:
			s = parseAggregation(expr, n)
			s.Operation = "sum"
		case promParser.MIN:
			s = parseAggregation(expr, n)
			s.Operation = "min"
		case promParser.MAX:
			s = parseAggregation(expr, n)
			s.Operation = "max"
		case promParser.AVG:
			s = parseAggregation(expr, n)
			s.Operation = "avg"
		case promParser.GROUP:
			s = parseAggregation(expr, n)
			s.Operation = "group"
		case promParser.STDDEV:
			s = parseAggregation(expr, n)
			s.Operation = "stddev"
		case promParser.STDVAR:
			s = parseAggregation(expr, n)
			s.Operation = "stdvar"
		case promParser.COUNT:
			s = parseAggregation(expr, n)
			s.Operation = "count"
		case promParser.COUNT_VALUES:
			s = parseAggregation(expr, n)
			s.Operation = "count_values"
			// Param is the label to store the count value in.
			s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, n.Param.(*promParser.StringLiteral).Val)
			s.IncludedLabels = appendToSlice(s.IncludedLabels, n.Param.(*promParser.StringLiteral).Val)
			s.ExcludedLabels = removeFromSlice(s.ExcludedLabels, n.Param.(*promParser.StringLiteral).Val)
			delete(s.ExcludeReason, n.Param.(*promParser.StringLiteral).Val)
		case promParser.QUANTILE:
			s = parseAggregation(expr, n)
			s.Operation = "quantile"
		case promParser.TOPK:
			s = walkNode(expr, n.Expr)
			s.Type = AggregateSource
			s.Operation = "topk"
		case promParser.BOTTOMK:
			s = walkNode(expr, n.Expr)
			s.Type = AggregateSource
			s.Operation = "bottomk"
			/*
				TODO these are experimental and promParser.EnableExperimentalFunctions must be set to true to enable parsing of these.
					case promParser.LIMITK:
						s = walkNode(expr, n.Expr)
						s.Type = AggregateSource
						s.Operation = "limitk"
					case promParser.LIMIT_RATIO:
						s = walkNode(expr, n.Expr)
						s.Type = AggregateSource
						s.Operation = "limit_ratio"
			*/
		}

	case *promParser.BinaryExpr:
		s = parseBinOps(expr, n)

	case *promParser.Call:
		s = parseCall(expr, n)

	case *promParser.MatrixSelector:
		s = walkNode(expr, n.VectorSelector)

	case *promParser.SubqueryExpr:
		s = walkNode(expr, n.Expr)

	case *promParser.NumberLiteral:
		s.Type = NumberSource
		s.Returns = promParser.ValueTypeScalar
		s.FixedLabels = true

	case *promParser.ParenExpr:
		s = walkNode(expr, n.Expr)

	case *promParser.StringLiteral:
		s.Type = StringSource
		s.Returns = promParser.ValueTypeString
		s.FixedLabels = true

	case *promParser.UnaryExpr:
		s = walkNode(expr, n.Expr)

	case *promParser.StepInvariantExpr:
		// Not possible to get this from the parser.

	case *promParser.VectorSelector:
		s.Type = SelectorSource
		s.Returns = promParser.ValueTypeVector
		s.Selector = n
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	default:
		// unhandled type
	}
	return s
}

func removeFromSlice(sl []string, s ...string) []string {
	for _, v := range s {
		idx := slices.Index(sl, v)
		if idx >= 0 {
			if len(sl) == 1 {
				return nil
			}
			sl = slices.Delete(sl, idx, idx+1)
		}
	}
	return sl
}

func appendToSlice(dst []string, values ...string) []string {
	for _, v := range values {
		if !slices.Contains(dst, v) {
			dst = append(dst, v)
		}
	}
	return dst
}

func setInMap(dst map[string]ExcludedLabel, key string, val ExcludedLabel) map[string]ExcludedLabel {
	if dst == nil {
		dst = map[string]ExcludedLabel{}
	}
	dst[key] = val
	return dst
}

func guaranteedLabelsFromSelector(selector *promParser.VectorSelector) (names []string) {
	// Any label used in positive filters is gurnateed to be present.
	for _, lm := range selector.LabelMatchers {
		if lm.Name == labels.MetricName {
			continue
		}
		if lm.Type == labels.MatchEqual || lm.Type == labels.MatchRegexp {
			names = appendToSlice(names, lm.Name)
		}
	}
	return names
}

func getQueryFragment(expr string, pos posrange.PositionRange) string {
	return expr[pos.Start:pos.End]
}

func parseAggregation(expr string, n *promParser.AggregateExpr) (s Source) {
	s = walkNode(expr, n.Expr)
	if n.Without {
		s.ExcludedLabels = appendToSlice(s.ExcludedLabels, n.Grouping...)
		s.IncludedLabels = removeFromSlice(s.IncludedLabels, n.Grouping...)
		s.GuaranteedLabels = removeFromSlice(s.GuaranteedLabels, n.Grouping...)
		for _, name := range n.Grouping {
			s.ExcludeReason = setInMap(
				s.ExcludeReason,
				name,
				ExcludedLabel{
					Reason: fmt.Sprintf("Query is using aggregation with `without(%s)`, all labels included inside `without(...)` will be removed from the results.",
						strings.Join(n.Grouping, ", ")),
					Fragment: getQueryFragment(expr, n.PosRange),
				},
			)
		}
	} else {
		s.FixedLabels = true
		if len(n.Grouping) == 0 {
			s.IncludedLabels = nil
			s.GuaranteedLabels = nil
			s.ExcludeReason = setInMap(
				s.ExcludeReason,
				"",
				ExcludedLabel{
					Reason:   "Query is using aggregation that removes all labels.",
					Fragment: getQueryFragment(expr, n.PosRange),
				},
			)
		} else {
			s.IncludedLabels = appendToSlice(s.IncludedLabels, n.Grouping...)
			for _, name := range n.Grouping {
				s.ExcludedLabels = removeFromSlice(s.ExcludedLabels, name)
			}
			s.ExcludeReason = setInMap(
				s.ExcludeReason,
				"",
				ExcludedLabel{
					Reason: fmt.Sprintf("Query is using aggregation with `by(%s)`, only labels included inside `by(...)` will be present on the results.",
						strings.Join(n.Grouping, ", ")),
					Fragment: getQueryFragment(expr, n.PosRange),
				},
			)
		}
	}
	s.Type = AggregateSource
	s.Returns = promParser.ValueTypeVector
	s.Call = nil
	return s
}

func parseCall(expr string, n *promParser.Call) (s Source) {
	s.Type = FuncSource
	s.Operation = n.Func.Name
	s.Call = n

	var vt promParser.ValueType
	for i, e := range n.Args {
		if i >= len(n.Func.ArgTypes) {
			vt = n.Func.ArgTypes[len(n.Func.ArgTypes)-1]
		} else {
			vt = n.Func.ArgTypes[i]
		}

		// nolint: exhaustive
		switch vt {
		case promParser.ValueTypeVector, promParser.ValueTypeMatrix:
			s.Selector = walkNode(expr, e).Selector
		}
	}

	switch n.Func.Name {
	case "abs", "sgn", "acos", "acosh", "asin", "asinh", "atan", "atanh", "cos", "cosh", "sin", "sinh", "tan", "tanh":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	case "ceil", "floor", "round":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	case "changes", "resets":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	case "clamp", "clamp_max", "clamp_min":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	case "absent", "absent_over_time":
		s.Returns = promParser.ValueTypeVector
		s.FixedLabels = true
		for _, lm := range s.Selector.LabelMatchers {
			if lm.Name == labels.MetricName {
				continue
			}
			if lm.Type == labels.MatchEqual {
				s.IncludedLabels = appendToSlice(s.IncludedLabels, lm.Name)
				s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, lm.Name)
			}
		}

	case "avg_over_time", "count_over_time", "last_over_time", "max_over_time", "min_over_time", "present_over_time", "quantile_over_time", "stddev_over_time", "stdvar_over_time", "sum_over_time":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	case "days_in_month", "day_of_month", "day_of_week", "day_of_year", "hour", "minute", "month", "year":
		s.Returns = promParser.ValueTypeVector
		// No labels if we don't pass any arguments.
		// Otherwise no change to labels.
		if len(s.Call.Args) == 0 {
			s.FixedLabels = true
		} else {
			s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)
		}

	case "deg", "rad", "ln", "log10", "log2", "sqrt", "exp":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	case "delta", "idelta", "increase", "deriv", "irate", "rate":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	case "histogram_avg", "histogram_count", "histogram_sum", "histogram_stddev", "histogram_stdvar", "histogram_fraction", "histogram_quantile":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	case "holt_winters", "predict_linear":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	case "label_replace", "label_join":
		// One label added to the results.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, s.Call.Args[1].(*promParser.StringLiteral).Val)

	case "pi":
		s.Returns = promParser.ValueTypeScalar
		s.FixedLabels = true

	case "scalar":
		s.Returns = promParser.ValueTypeScalar
		s.FixedLabels = true

	case "sort", "sort_desc":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector

	case "time":
		s.Returns = promParser.ValueTypeScalar
		s.FixedLabels = true

	case "timestamp":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, guaranteedLabelsFromSelector(s.Selector)...)

	case "vector":
		s.Returns = promParser.ValueTypeVector
		s.FixedLabels = true

	default:
		// Unsupported function
		s.Returns = promParser.ValueTypeNone
		s.Call = nil
	}

	return s
}

func parseBinOps(expr string, n *promParser.BinaryExpr) (s Source) {
	switch {
	case n.VectorMatching == nil:
		s = walkNode(expr, n.LHS)
		if s.Returns == promParser.ValueTypeScalar || s.Returns == promParser.ValueTypeString {
			s = walkNode(expr, n.RHS)
		}

		// foo{} +               bar{}
		// foo{} + on(...)       bar{}
		// foo{} + ignoring(...) bar{}
	case n.VectorMatching.Card == promParser.CardOneToOne:
		s = walkNode(expr, n.LHS)
		if n.VectorMatching.On {
			s.FixedLabels = true
			s.IncludedLabels = appendToSlice(s.IncludedLabels, n.VectorMatching.MatchingLabels...)
			s.ExcludedLabels = removeFromSlice(s.ExcludedLabels, n.VectorMatching.MatchingLabels...)
			for _, name := range n.VectorMatching.MatchingLabels {
				delete(s.ExcludeReason, name)
			}
			s.ExcludeReason = setInMap(
				s.ExcludeReason,
				"",
				ExcludedLabel{
					Reason: fmt.Sprintf(
						"Query is using %s vector matching with `on(%s)`, only labels included inside `on(...)` will be present on the results.",
						n.VectorMatching.Card, strings.Join(n.VectorMatching.MatchingLabels, ", "),
					),
					Fragment: getQueryFragment(
						expr,
						posrange.PositionRange{
							Start: n.LHS.PositionRange().Start,
							End:   n.RHS.PositionRange().End,
						},
					),
				},
			)
		} else {
			s.IncludedLabels = removeFromSlice(s.IncludedLabels, n.VectorMatching.MatchingLabels...)
			s.GuaranteedLabels = removeFromSlice(s.GuaranteedLabels, n.VectorMatching.MatchingLabels...)
			s.ExcludedLabels = appendToSlice(s.ExcludedLabels, n.VectorMatching.MatchingLabels...)
			for _, name := range n.VectorMatching.MatchingLabels {
				s.ExcludeReason = setInMap(
					s.ExcludeReason,
					name,
					ExcludedLabel{
						Reason: fmt.Sprintf(
							"Query is using %s vector matching with `ignoring(%s)`, all labels included inside `ignoring(...)` will be removed on the results.",
							n.VectorMatching.Card, strings.Join(n.VectorMatching.MatchingLabels, ", "),
						),
						Fragment: getQueryFragment(
							expr,
							posrange.PositionRange{
								Start: n.LHS.PositionRange().Start,
								End:   n.RHS.PositionRange().End,
							},
						),
					},
				)
			}
		}

		// foo{} + on(...)       group_left(...) bar{}
		// foo{} + ignoring(...) group_left(...) bar{}
	case n.VectorMatching.Card == promParser.CardOneToMany:
		s = walkNode(expr, n.RHS)
		s.IncludedLabels = appendToSlice(s.IncludedLabels, n.VectorMatching.Include...)
		if n.VectorMatching.On {
			s.IncludedLabels = appendToSlice(s.IncludedLabels, n.VectorMatching.MatchingLabels...)
			for _, name := range n.VectorMatching.MatchingLabels {
				delete(s.ExcludeReason, name)
			}
		}
		s.ExcludedLabels = removeFromSlice(s.ExcludedLabels, n.VectorMatching.Include...)
		for _, name := range n.VectorMatching.Include {
			delete(s.ExcludeReason, name)
		}

		// foo{} + on(...)       group_right(...) bar{}
		// foo{} + ignoring(...) group_right(...) bar{}
	case n.VectorMatching.Card == promParser.CardManyToOne:
		s = walkNode(expr, n.LHS)
		s.IncludedLabels = appendToSlice(s.IncludedLabels, n.VectorMatching.Include...)
		if n.VectorMatching.On {
			s.IncludedLabels = appendToSlice(s.IncludedLabels, n.VectorMatching.MatchingLabels...)
			for _, name := range n.VectorMatching.MatchingLabels {
				delete(s.ExcludeReason, name)
			}
		}
		s.ExcludedLabels = removeFromSlice(s.ExcludedLabels, n.VectorMatching.Include...)
		for _, name := range n.VectorMatching.Include {
			delete(s.ExcludeReason, name)
		}

		// foo{} and on(...)       bar{}
		// foo{} and ignoring(...) bar{}
	case n.VectorMatching.Card == promParser.CardManyToMany:
		s = walkNode(expr, n.LHS)
		if n.VectorMatching.On {
			s.IncludedLabels = appendToSlice(s.IncludedLabels, n.VectorMatching.MatchingLabels...)
			for _, name := range n.VectorMatching.MatchingLabels {
				delete(s.ExcludeReason, name)
			}
		}
		if n.Op == promParser.LOR {
			s.alternatives = append(s.alternatives, walkNode(expr, n.RHS))
		}
	}

	if n.VectorMatching != nil && s.Operation == "" {
		s.Operation = n.VectorMatching.Card.String()
	}

	return s
}
