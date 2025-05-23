package utils

import (
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/parser/posrange"
)

var guaranteedLabelsMatches = []labels.MatchType{labels.MatchEqual, labels.MatchRegexp}

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
	Fragment posrange.PositionRange
}

type Join struct {
	Src Source
}

// FIXME remove Selector/Call/Aggregation?
// Use a single parser.Node instead?
type Source struct {
	Selector         *promParser.VectorSelector // Vector selector used for this source.
	Call             *promParser.Call           // Most outer call used inside this source.
	Aggregation      *promParser.AggregateExpr  // Most outer aggregation expression used inside this source.
	ExcludeReason    map[string]ExcludedLabel   // Reason why a label was excluded
	Operation        string
	IsDeadReason     string
	Returns          promParser.ValueType
	Joins            []Join   // Any other sources this source joins with.
	Unless           []Join   // Any other sources this source is suppressed by.
	IncludedLabels   []string // Labels that are included by filters, they will be present if exist on source series (by).
	ExcludedLabels   []string // Labels guaranteed to be excluded from the results (without).
	GuaranteedLabels []string // Labels guaranteed to be present on the results (matchers).
	Position         posrange.PositionRange
	IsDeadPosition   posrange.PositionRange
	ReturnedNumber   float64 // If AlwaysReturns=true this is the number that's returned
	Type             SourceType
	FixedLabels      bool // Labels are fixed and only allowed labels can be present.
	IsDead           bool // True if this source cannot be reached and is dead code.
	AlwaysReturns    bool // True if this source always returns results.
	KnownReturn      bool // True if we always know the return value.
	IsConditional    bool // True if this source is guarded by 'foo > 5' or other condition.
	IsReturnBool     bool // True if this source uses the 'bool' modifier.
}

func (s Source) Fragment(expr string) string {
	switch {
	case s.Type == FuncSource && s.Call != nil:
		return GetQueryFragment(expr, s.Call.PosRange)
	case s.Call != nil:
		return GetQueryFragment(expr, s.Call.PosRange)
	case s.Type == AggregateSource && s.Aggregation != nil:
		return GetQueryFragment(expr, s.Aggregation.PosRange)
	case s.Selector != nil:
		return GetQueryFragment(expr, s.Selector.PosRange)
	default:
		return ""
	}
}

func (s Source) CanHaveLabel(name string) bool {
	if slices.Contains(s.ExcludedLabels, name) {
		return false
	}
	if slices.Contains(s.IncludedLabels, name) {
		return true
	}
	if slices.Contains(s.GuaranteedLabels, name) {
		return true
	}
	return !s.FixedLabels
}

func (s Source) LabelExcludeReason(name string) ExcludedLabel {
	if el, ok := s.ExcludeReason[name]; ok {
		return el
	}
	return s.ExcludeReason[""]
}

type Visitor func(s Source)

func (s Source) WalkSources(fn Visitor) {
	fn(s)
	for _, j := range s.Joins {
		j.Src.WalkSources(fn)
	}
	for _, u := range s.Unless {
		u.Src.WalkSources(fn)
	}
}

func LabelsSource(expr string, node promParser.Node) (src []Source) {
	return walkNode(expr, node)
}

func walkNode(expr string, node promParser.Node) (src []Source) {
	var s Source
	switch n := node.(type) {
	case *promParser.AggregateExpr:
		src = append(src, walkAggregation(expr, n)...)

	case *promParser.BinaryExpr:
		src = append(src, parseBinOps(expr, n)...)

	case *promParser.Call:
		src = append(src, parseCall(expr, n)...)

	case *promParser.MatrixSelector:
		for _, s := range walkNode(expr, n.VectorSelector) {
			s.Returns = promParser.ValueTypeMatrix
			src = append(src, s)
		}

	case *promParser.SubqueryExpr:
		src = append(src, walkNode(expr, n.Expr)...)

	case *promParser.NumberLiteral:
		s.Type = NumberSource
		s.Returns = promParser.ValueTypeScalar
		s.KnownReturn = true
		s.ReturnedNumber = n.Val
		s.FixedLabels = true
		s.AlwaysReturns = true
		s = excludeAllLabels(s, "This query returns a number value with no labels.", n.PosRange, nil)
		s.Position = n.PosRange
		src = append(src, s)

	case *promParser.ParenExpr:
		src = append(src, walkNode(expr, n.Expr)...)

	case *promParser.StringLiteral:
		s.Type = StringSource
		s.Returns = promParser.ValueTypeString
		s.FixedLabels = true
		s.AlwaysReturns = true
		s = excludeAllLabels(s, "This query returns a string value with no labels.", n.PosRange, nil)
		s.Position = n.PosRange
		src = append(src, s)

	case *promParser.UnaryExpr:
		src = append(src, walkNode(expr, n.Expr)...)

	case *promParser.StepInvariantExpr:
		// Not possible to get this from the parser.

	case *promParser.VectorSelector:
		s.Type = SelectorSource
		s.Returns = promParser.ValueTypeVector
		s.Selector = n
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, n)...)
		for _, name := range labelsWithEmptyValueSelector(n) {
			s = excludeLabel(
				s,
				fmt.Sprintf("Query uses `{%s=\"\"}` selector which will filter out any time series with the `%s` label set.", name, name),
				n.PosRange,
				name,
			)
		}
		s.Position = n.PosRange
		src = append(src, s)

	default:
		// unhandled type
	}
	return src
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

func includeLabel(s Source, names ...string) Source {
	s.ExcludedLabels = removeFromSlice(s.ExcludedLabels, names...)
	for _, name := range names {
		delete(s.ExcludeReason, name)
	}
	s.IncludedLabels = appendToSlice(s.IncludedLabels, names...)
	return s
}

// Include labels that were not already excluded.
func maybeIncludeLabel(s Source, names ...string) Source {
	for _, name := range names {
		if !slices.Contains(s.ExcludedLabels, name) {
			s.IncludedLabels = appendToSlice(s.IncludedLabels, names...)
		}
	}
	return s
}

func restrictIncludedLabels(s Source, names []string) Source {
	if len(names) == 0 {
		s.IncludedLabels = nil
		return s
	}

	todo := []string{}
	for _, name := range s.IncludedLabels {
		if !slices.Contains(names, name) {
			todo = append(todo, name)
		}
	}
	s.IncludedLabels = removeFromSlice(s.IncludedLabels, todo...)
	return s
}

func guaranteeLabel(s Source, names ...string) Source {
	s.ExcludedLabels = removeFromSlice(s.ExcludedLabels, names...)
	for _, name := range names {
		delete(s.ExcludeReason, name)
	}
	s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, names...)
	return s
}

func restrictGuaranteedLabels(s Source, names []string) Source {
	if len(names) == 0 {
		s.GuaranteedLabels = nil
		return s
	}

	todo := []string{}
	for _, name := range s.GuaranteedLabels {
		if !slices.Contains(names, name) {
			todo = append(todo, name)
		}
	}
	s.GuaranteedLabels = removeFromSlice(s.GuaranteedLabels, todo...)
	return s
}

func excludeLabel(s Source, reason string, fragment posrange.PositionRange, names ...string) Source {
	s.ExcludedLabels = appendToSlice(s.ExcludedLabels, names...)
	el := ExcludedLabel{
		Reason:   reason,
		Fragment: fragment,
	}
	for _, name := range names {
		s.ExcludeReason = setInMap(s.ExcludeReason, name, el)
	}
	s.IncludedLabels = removeFromSlice(s.IncludedLabels, names...)
	s.GuaranteedLabels = removeFromSlice(s.GuaranteedLabels, names...)
	return s
}

func excludeAllLabels(s Source, reason string, fragment posrange.PositionRange, except []string) Source {
	el := ExcludedLabel{
		Reason:   reason,
		Fragment: fragment,
	}
	// Everything that was included until now but will be removed needs an explicit stamp to mark it as gone.
	for _, name := range s.IncludedLabels {
		if slices.Contains(except, name) {
			continue
		}
		if s.CanHaveLabel(name) {
			s.ExcludeReason = setInMap(s.ExcludeReason, name, el)
		}
	}
	for _, name := range s.GuaranteedLabels {
		if slices.Contains(except, name) {
			continue
		}
		if s.CanHaveLabel(name) {
			s.ExcludeReason = setInMap(s.ExcludeReason, name, el)
		}
	}

	s.ExcludeReason = setInMap(s.ExcludeReason, "", el)

	s = restrictGuaranteedLabels(s, except)
	s = restrictIncludedLabels(s, except)
	return s
}

func setInMap(dst map[string]ExcludedLabel, key string, val ExcludedLabel) map[string]ExcludedLabel {
	if dst == nil {
		return map[string]ExcludedLabel{key: val}
	}
	dst[key] = val
	return dst
}

func labelsFromSelectors(matches []labels.MatchType, selector *promParser.VectorSelector) (names []string) {
	if selector == nil {
		return nil
	}
	// Any label used in positive filters is gurnateed to be present.
	for _, lm := range selector.LabelMatchers {
		if lm.Name == labels.MetricName {
			continue
		}
		if !slices.Contains(matches, lm.Type) {
			continue
		}
		names = appendToSlice(names, lm.Name)
	}
	return names
}

func labelsWithEmptyValueSelector(selector *promParser.VectorSelector) (names []string) {
	for _, lm := range selector.LabelMatchers {
		if lm.Name == labels.MetricName {
			continue
		}
		if lm.Type == labels.MatchEqual && lm.Value == "" {
			names = appendToSlice(names, lm.Name)
		}
	}
	return names
}

func GetQueryFragment(expr string, pos posrange.PositionRange) string {
	return expr[pos.Start:pos.End]
}

func walkAggregation(expr string, n *promParser.AggregateExpr) (src []Source) {
	var s Source
	switch n.Op {
	case promParser.SUM:
		for _, s = range parseAggregation(expr, n) {
			s.Aggregation = n
			s.Operation = "sum"
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s = excludeLabel(s, "Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.MIN:
		for _, s = range parseAggregation(expr, n) {
			s.Aggregation = n
			s.Operation = "min"
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s = excludeLabel(s, "Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.MAX:
		for _, s = range parseAggregation(expr, n) {
			s.Aggregation = n
			s.Operation = "max"
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s = excludeLabel(s, "Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.AVG:
		for _, s = range parseAggregation(expr, n) {
			s.Aggregation = n
			s.Operation = "avg"
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s = excludeLabel(s, "Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.GROUP:
		for _, s = range parseAggregation(expr, n) {
			s.Aggregation = n
			s.Operation = "group"
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s = excludeLabel(s, "Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.STDDEV:
		for _, s = range parseAggregation(expr, n) {
			s.Aggregation = n
			s.Operation = "stddev"
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s = excludeLabel(s, "Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.STDVAR:
		for _, s = range parseAggregation(expr, n) {
			s.Aggregation = n
			s.Operation = "stdvar"
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s = excludeLabel(s, "Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.COUNT:
		for _, s = range parseAggregation(expr, n) {
			s.Aggregation = n
			s.Operation = "count"
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s = excludeLabel(s, "Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.COUNT_VALUES:
		for _, s = range parseAggregation(expr, n) {
			s.Aggregation = n
			s.Operation = "count_values"
			// Param is the label to store the count value in.
			s = includeLabel(s, n.Param.(*promParser.StringLiteral).Val)
			s = guaranteeLabel(s, n.Param.(*promParser.StringLiteral).Val)
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s = excludeLabel(s, "Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.QUANTILE:
		for _, s = range parseAggregation(expr, n) {
			s.Aggregation = n
			s.Operation = "quantile"
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s = excludeLabel(s, "Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.TOPK:
		for _, s = range walkNode(expr, n.Expr) {
			s.Type = AggregateSource
			s.Aggregation = n
			s.Operation = "topk"
			src = append(src, s)
		}
	case promParser.BOTTOMK:
		for _, s = range walkNode(expr, n.Expr) {
			s.Type = AggregateSource
			s.Aggregation = n
			s.Operation = "bottomk"
			src = append(src, s)
		}
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
	return src
}

func parseAggregation(expr string, n *promParser.AggregateExpr) (src []Source) {
	var s Source
	for _, s = range walkNode(expr, n.Expr) {
		if n.Without {
			s = excludeLabel(
				s,
				fmt.Sprintf("Query is using aggregation with `without(%s)`, all labels included inside `without(...)` will be removed from the results.",
					strings.Join(n.Grouping, ", ")),
				FindPosition(expr, n.PosRange, "without"),
				n.Grouping...,
			)
		} else {
			if len(n.Grouping) == 0 {
				s = excludeAllLabels(
					s,
					"Query is using aggregation that removes all labels.",
					FindPosition(expr, n.PosRange, "sum"),
					nil,
				)
			} else {
				// Check if source of labels already fixes them.
				if !s.FixedLabels {
					s = maybeIncludeLabel(s, n.Grouping...)
				}
				for _, name := range n.Grouping {
					if !s.CanHaveLabel(name) {
						el := s.LabelExcludeReason(name)
						s = excludeLabel(s, el.Reason, el.Fragment, name)
					}
				}
				s = excludeAllLabels(
					s,
					fmt.Sprintf("Query is using aggregation with `by(%s)`, only labels included inside `by(...)` will be present on the results.",
						strings.Join(n.Grouping, ", ")),
					FindPosition(expr, n.PosRange, "by"),
					n.Grouping,
				)
			}
			s.FixedLabels = true
		}
		s.Type = AggregateSource
		s.Returns = promParser.ValueTypeVector
		src = append(src, s)
	}
	return src
}

func parsePromQLFunc(s Source, expr string, n *promParser.Call) Source {
	switch n.Func.Name {
	case "abs", "sgn", "acos", "acosh", "asin", "asinh", "atan", "atanh", "cos", "cosh", "sin", "sinh", "tan", "tanh":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)

	case "ceil", "floor", "round":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)

	case "changes", "resets":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)

	case "clamp", "clamp_max", "clamp_min":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)

	case "absent", "absent_over_time":
		s.Returns = promParser.ValueTypeVector
		s.FixedLabels = true
		names := labelsFromSelectors([]labels.MatchType{labels.MatchEqual}, s.Selector)
		s = excludeAllLabels(
			s,
			fmt.Sprintf(`The [%s()](https://prometheus.io/docs/prometheus/latest/querying/functions/#%s) function is used to check if provided query doesn't match any time series.
You will only get any results back if the metric selector you pass doesn't match anything.
Since there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.
This means that the only labels you can get back from absent call are the ones you pass to it.
If you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `+"`up`"+` metric instead.`,
				n.Func.Name, n.Func.Name),
			FindPosition(expr, n.PosRange, n.Func.Name),
			names,
		)
		for _, name := range names {
			s = includeLabel(s, name)
			s = guaranteeLabel(s, name)
		}

	case "avg_over_time", "count_over_time", "last_over_time", "max_over_time", "min_over_time", "present_over_time", "quantile_over_time", "stddev_over_time", "stdvar_over_time", "sum_over_time":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)

	case "days_in_month", "day_of_month", "day_of_week", "day_of_year", "hour", "minute", "month", "year":
		s.Returns = promParser.ValueTypeVector
		// No labels if we don't pass any arguments.
		// Otherwise no change to labels.
		if len(s.Call.Args) == 0 {
			s.FixedLabels = true
			s.AlwaysReturns = true
			s = excludeAllLabels(
				s,
				fmt.Sprintf("Calling `%s()` with no arguments will return an empty time series with no labels.",
					n.Func.Name),
				n.PosRange,
				nil,
			)
		} else {
			s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)
		}

	case "deg", "rad", "ln", "log10", "log2", "sqrt", "exp":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)

	case "delta", "idelta", "increase", "deriv", "irate", "rate":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)

	case "histogram_avg", "histogram_count", "histogram_sum", "histogram_stddev", "histogram_stdvar", "histogram_fraction", "histogram_quantile":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)

	case "holt_winters", "predict_linear":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)

	case "label_replace", "label_join":
		// One label added to the results.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, n.Args[1].(*promParser.StringLiteral).Val)

	case "pi":
		s.Returns = promParser.ValueTypeScalar
		s.FixedLabels = true
		s.AlwaysReturns = true
		s = excludeAllLabels(
			s,
			fmt.Sprintf("Calling `%s()` will return a scalar value with no labels.", n.Func.Name),
			n.PosRange,
			nil,
		)

	case "scalar":
		s.Returns = promParser.ValueTypeScalar
		s.FixedLabels = true
		s.AlwaysReturns = true
		s = excludeAllLabels(
			s,
			fmt.Sprintf("Calling `%s()` will return a scalar value with no labels.", n.Func.Name),
			FindPosition(expr, n.PositionRange(), n.Func.Name),
			nil,
		)

	case "sort", "sort_desc":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector

	case "time":
		s.Returns = promParser.ValueTypeScalar
		s.FixedLabels = true
		s.AlwaysReturns = true
		s = excludeAllLabels(
			s,
			fmt.Sprintf("Calling `%s()` will return a scalar value with no labels.", n.Func.Name),
			n.PosRange,
			nil,
		)

	case "timestamp":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		s = guaranteeLabel(s, labelsFromSelectors(guaranteedLabelsMatches, s.Selector)...)

	case "vector":
		s.Returns = promParser.ValueTypeVector
		s.FixedLabels = true
		s.AlwaysReturns = true
		for _, vs := range walkNode(expr, n.Args[0]) {
			if vs.KnownReturn {
				s.ReturnedNumber = vs.ReturnedNumber
				s.KnownReturn = true
			}
		}
		s = excludeAllLabels(
			s,
			fmt.Sprintf("Calling `%s()` will return a vector value with no labels.", n.Func.Name),
			FindPosition(expr, n.PosRange, n.Func.Name),
			nil,
		)

	default:
		// Unsupported function
		s.Returns = promParser.ValueTypeNone
		s.Call = nil
	}
	return s
}

func parseCall(expr string, n *promParser.Call) (src []Source) {
	var vt promParser.ValueType
	for i, e := range n.Args {
		if i >= len(n.Func.ArgTypes) {
			vt = n.Func.ArgTypes[len(n.Func.ArgTypes)-1]
		} else {
			vt = n.Func.ArgTypes[i]
		}

		switch vt {
		case promParser.ValueTypeVector, promParser.ValueTypeMatrix:
			for _, es := range walkNode(expr, e) {
				es.Type = FuncSource
				es.Operation = n.Func.Name
				es.Call = n
				es.Position = e.PositionRange()
				src = append(src, parsePromQLFunc(es, expr, n))
			}
		case promParser.ValueTypeNone, promParser.ValueTypeScalar, promParser.ValueTypeString:
		}
	}

	if len(src) == 0 {
		var s Source
		s.Type = FuncSource
		s.Operation = n.Func.Name
		s.Call = n
		s.Position = n.PosRange
		src = append(src, parsePromQLFunc(s, expr, n))
	}

	return src
}

func parseBinOps(expr string, n *promParser.BinaryExpr) (src []Source) {
	var s Source
	switch {

	// foo{} + 1
	// 1 + foo{}
	// foo{} > 1
	// 1 > foo{}
	case n.VectorMatching == nil:
		lhs := walkNode(expr, n.LHS)
		rhs := walkNode(expr, n.RHS)
		for _, ls := range lhs {
			ls.IsConditional, ls.IsReturnBool = checkConditions(ls, n.Op, n.ReturnBool)
			for _, rs := range rhs {
				rs.IsConditional, rs.IsReturnBool = checkConditions(rs, n.Op, n.ReturnBool)
				var side Source
				switch {
				case ls.Returns == promParser.ValueTypeVector, ls.Returns == promParser.ValueTypeMatrix:
					// Use labels from LHS
					side = ls
				case rs.Returns == promParser.ValueTypeVector, rs.Returns == promParser.ValueTypeMatrix:
					// Use labels from RHS
					side = rs
				default:
					side = ls
				}
				if ls.AlwaysReturns && rs.AlwaysReturns && ls.KnownReturn && rs.KnownReturn {
					// Both sides always return something
					side.ReturnedNumber, side.IsDead, side.IsDeadReason, side.IsDeadPosition = calculateStaticReturn(
						expr,
						ls, rs,
						n.Op,
						ls.IsDead,
					)
				}
				src = append(src, side)
			}
		}

		// foo{} +               bar{}
		// foo{} + on(...)       bar{}
		// foo{} + ignoring(...) bar{}
		// foo{} /               bar{}
	case n.VectorMatching.Card == promParser.CardOneToOne:
		rhs := walkNode(expr, n.RHS)
		for _, s = range walkNode(expr, n.LHS) {
			if n.VectorMatching.On {
				s.FixedLabels = true
				s = excludeAllLabels(
					s,
					fmt.Sprintf(
						"Query is using %s vector matching with `on(%s)`, only labels included inside `on(...)` will be present on the results.",
						n.VectorMatching.Card, strings.Join(n.VectorMatching.MatchingLabels, ", "),
					),
					FindPosition(expr, n.PositionRange(), "on"),
					n.VectorMatching.MatchingLabels,
				)
				s = includeLabel(s, n.VectorMatching.MatchingLabels...)
			} else {
				s = excludeLabel(
					s,
					fmt.Sprintf(
						"Query is using %s vector matching with `ignoring(%s)`, all labels included inside `ignoring(...)` will be removed on the results.",
						n.VectorMatching.Card, strings.Join(n.VectorMatching.MatchingLabels, ", "),
					),
					FindPosition(expr, n.PositionRange(), "ignoring"),
					n.VectorMatching.MatchingLabels...,
				)
				for _, rs := range rhs {
					rs.IsConditional, rs.IsReturnBool = checkConditions(rs, n.Op, n.ReturnBool)
					if s.AlwaysReturns && rs.AlwaysReturns && s.KnownReturn && rs.KnownReturn {
						// Both sides always return something
						s.ReturnedNumber, s.IsDead, s.IsDeadReason, s.IsDeadPosition = calculateStaticReturn(
							expr,
							s, rs,
							n.Op,
							s.IsDead,
						)
					}
				}
			}
			if s.Operation == "" {
				s.Operation = n.VectorMatching.Card.String()
			}
			for _, rs := range rhs {
				if ok, s, pos := canJoin(s, rs, n.VectorMatching); !ok {
					rs.IsDead = true
					rs.IsDeadReason = s
					rs.IsDeadPosition = pos
				}
				s.Joins = append(s.Joins, Join{
					Src: rs,
				})
			}
			s.IsConditional, s.IsReturnBool = checkConditions(s, n.Op, n.ReturnBool)
			src = append(src, s)
		}

		// foo{} + on(...)       group_right(...) bar{}
		// foo{} + ignoring(...) group_right(...) bar{}
	case n.VectorMatching.Card == promParser.CardOneToMany:
		lhs := walkNode(expr, n.LHS)
		for _, s = range walkNode(expr, n.RHS) {
			s = includeLabel(s, n.VectorMatching.Include...)
			// If we have:
			// foo * on(instance) group_left(a,b) bar{x="y"}
			// then only group_left() labels will be included.
			if n.VectorMatching.On {
				s = includeLabel(s, n.VectorMatching.MatchingLabels...)
			}
			if s.Operation == "" {
				s.Operation = n.VectorMatching.Card.String()
			}
			for _, ls := range lhs {
				if ok, s, pos := canJoin(s, ls, n.VectorMatching); !ok {
					ls.IsDead = true
					ls.IsDeadReason = s
					ls.IsDeadPosition = pos
				}
				s.Joins = append(s.Joins, Join{
					Src: ls,
				})
			}
			s.IsConditional, s.IsReturnBool = checkConditions(s, n.Op, n.ReturnBool)
			src = append(src, s)
		}

		// foo{} + on(...)       group_left(...) bar{}
		// foo{} + ignoring(...) group_left(...) bar{}
	case n.VectorMatching.Card == promParser.CardManyToOne:
		rhs := walkNode(expr, n.RHS)
		for _, s = range walkNode(expr, n.LHS) {
			s = includeLabel(s, n.VectorMatching.Include...)
			if n.VectorMatching.On {
				s = includeLabel(s, n.VectorMatching.MatchingLabels...)
			}
			if s.Operation == "" {
				s.Operation = n.VectorMatching.Card.String()
			}
			for _, rs := range rhs {
				if ok, s, pos := canJoin(s, rs, n.VectorMatching); !ok {
					rs.IsDead = true
					rs.IsDeadReason = s
					rs.IsDeadPosition = pos
				}
				s.Joins = append(s.Joins, Join{
					Src: rs,
				})
			}
			s.IsConditional, s.IsReturnBool = checkConditions(s, n.Op, n.ReturnBool)
			src = append(src, s)
		}

		// foo{} and on(...)       bar{}
		// foo{} and ignoring(...) bar{}
		// foo{} unless bar{}
	case n.VectorMatching.Card == promParser.CardManyToMany:
		var lhsCanBeEmpty bool // true if any of the LHS query can produce empty results.
		rhs := walkNode(expr, n.RHS)
		for _, s = range walkNode(expr, n.LHS) {
			var rhsConditional bool
			if n.VectorMatching.On {
				s = includeLabel(s, n.VectorMatching.MatchingLabels...)
			}
			if s.Operation == "" {
				s.Operation = n.VectorMatching.Card.String()
			}
			if !s.AlwaysReturns || s.IsConditional {
				lhsCanBeEmpty = true
			}
			for _, rs := range rhs {
				isConditional, _ := checkConditions(rs, n.Op, n.ReturnBool)
				if isConditional {
					rhsConditional = true
				}
				if ok, s, pos := canJoin(s, rs, n.VectorMatching); !ok {
					rs.IsDead = true
					rs.IsDeadReason = s
					rs.IsDeadPosition = pos
				}
				switch {
				case n.Op == promParser.LUNLESS:
					if n.VectorMatching.On && len(n.VectorMatching.MatchingLabels) == 0 && rs.AlwaysReturns && !rs.IsConditional {
						s.IsDead = true
						s.IsDeadReason = "this query will never return anything because the `unless` query always returns something"
						s.IsDeadPosition = rs.Position
					}
					s.Unless = append(s.Unless, Join{
						Src: rs,
					})
				case n.Op != promParser.LOR:
					s.Joins = append(s.Joins, Join{
						Src: rs,
					})
				}
			}
			if n.Op == promParser.LAND && rhsConditional {
				s.IsConditional = true
			}
			src = append(src, s)
		}
		if n.Op == promParser.LOR {
			for _, s = range rhs {
				if s.Operation == "" {
					s.Operation = n.VectorMatching.Card.String()
				}
				// If LHS can NOT be empty then RHS is dead code.
				if !lhsCanBeEmpty {
					s.IsDead = true
					s.IsDeadReason = "the left hand side always returs something and so the right hand side is never used"
					s.IsDeadPosition = s.Position
				}
				src = append(src, s)
			}
		}
	}
	return src
}

func checkConditions(s Source, op promParser.ItemType, isBool bool) (isConditional, isReturnBool bool) {
	if !s.IsConditional && isBool {
		isReturnBool = isBool
	}
	if s.IsConditional {
		isConditional = s.IsConditional
	} else {
		isConditional = op.IsComparisonOperator()
	}
	return isConditional, isReturnBool
}

func canJoin(ls, rs Source, vm *promParser.VectorMatching) (bool, string, posrange.PositionRange) {
	var side string
	if vm.Card == promParser.CardOneToMany {
		side = "left"
	} else {
		side = "right"
	}

	switch {
	case vm.On && len(vm.MatchingLabels) == 0: // ls on() unless rs
		return true, "", posrange.PositionRange{}
	case vm.On: // ls on(...) unless rs
		for _, name := range vm.MatchingLabels {
			if ls.CanHaveLabel(name) && !rs.CanHaveLabel(name) {
				return false, fmt.Sprintf("The %s hand side will never be matched because it doesn't have the `%s` label from `on(...)`. %s",
					side, name, rs.LabelExcludeReason(name).Reason), rs.LabelExcludeReason(name).Fragment
			}
		}
	default: // ls unless rs
		for _, name := range ls.GuaranteedLabels {
			if ls.CanHaveLabel(name) && !rs.CanHaveLabel(name) {
				return false, fmt.Sprintf("The %s hand side will never be matched because it doesn't have the `%s` label while the left hand side will. %s",
					side, name, rs.LabelExcludeReason(name).Reason), rs.LabelExcludeReason(name).Fragment
			}
		}
	}
	return true, "", posrange.PositionRange{}
}

func ftos(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func calculateStaticReturn(expr string, ls, rs Source, op promParser.ItemType, isDead bool) (float64, bool, string, posrange.PositionRange) {
	lf := ls.Fragment(expr)
	rf := rs.Fragment(expr)
	var cmpPrefix string
	if lf != "" && rf != "" {
		cmpPrefix = fmt.Sprintf("`%s %s %s` always evaluates to", lf, op, rf)
	} else {
		cmpPrefix = "this query always evaluates to"
	}
	cmpSuffix := "which is not possible, so it will never return anything"
	switch op {
	case promParser.EQLC:
		if ls.ReturnedNumber != rs.ReturnedNumber {
			return ls.ReturnedNumber,
				true,
				fmt.Sprintf("%s `%s == %s` %s", cmpPrefix, ftos(ls.ReturnedNumber), ftos(rs.ReturnedNumber), cmpSuffix),
				ls.Position
		}
	case promParser.NEQ:
		if ls.ReturnedNumber == rs.ReturnedNumber {
			return ls.ReturnedNumber,
				true,
				fmt.Sprintf("%s `%s != %s` %s", cmpPrefix, ftos(ls.ReturnedNumber), ftos(rs.ReturnedNumber), cmpSuffix),
				ls.Position
		}
	case promParser.LTE:
		if ls.ReturnedNumber > rs.ReturnedNumber {
			return ls.ReturnedNumber,
				true,
				fmt.Sprintf("%s `%s <= %s` %s", cmpPrefix, ftos(ls.ReturnedNumber), ftos(rs.ReturnedNumber), cmpSuffix),
				ls.Position
		}
	case promParser.LSS:
		if ls.ReturnedNumber >= rs.ReturnedNumber {
			return ls.ReturnedNumber,
				true,
				fmt.Sprintf("%s `%s < %s` %s", cmpPrefix, ftos(ls.ReturnedNumber), ftos(rs.ReturnedNumber), cmpSuffix),
				ls.Position
		}
	case promParser.GTE:
		if ls.ReturnedNumber < rs.ReturnedNumber {
			return ls.ReturnedNumber,
				true,
				fmt.Sprintf("%s `%s >= %s` %s", cmpPrefix, ftos(ls.ReturnedNumber), ftos(rs.ReturnedNumber), cmpSuffix),
				ls.Position
		}
	case promParser.GTR:
		if ls.ReturnedNumber <= rs.ReturnedNumber {
			return ls.ReturnedNumber,
				true,
				fmt.Sprintf("%s `%s > %s` %s", cmpPrefix, ftos(ls.ReturnedNumber), ftos(rs.ReturnedNumber), cmpSuffix),
				ls.Position
		}
	case promParser.ADD:
		return ls.ReturnedNumber + rs.ReturnedNumber, isDead, "", ls.IsDeadPosition
	case promParser.SUB:
		return ls.ReturnedNumber - rs.ReturnedNumber, isDead, "", ls.IsDeadPosition
	case promParser.MUL:
		return ls.ReturnedNumber * rs.ReturnedNumber, isDead, "", ls.IsDeadPosition
	case promParser.DIV:
		return ls.ReturnedNumber / rs.ReturnedNumber, isDead, "", ls.IsDeadPosition
	case promParser.MOD:
		return math.Mod(ls.ReturnedNumber, rs.ReturnedNumber), isDead, "", ls.IsDeadPosition
	case promParser.POW:
		return math.Pow(ls.ReturnedNumber, rs.ReturnedNumber), isDead, "", ls.IsDeadPosition
	}
	return ls.ReturnedNumber, isDead, "", ls.IsDeadPosition
}

// FIXME sum() on ().
func FindPosition(expr string, within posrange.PositionRange, fn string) posrange.PositionRange {
	re := regexp.MustCompile("(?i)(" + regexp.QuoteMeta(fn) + ")[ \n\t]*\\(")
	idx := re.FindStringSubmatchIndex(GetQueryFragment(expr, within))
	if idx == nil {
		return within
	}
	return posrange.PositionRange{
		Start: within.Start + posrange.Pos(idx[0]),
		End:   within.Start + posrange.Pos(idx[1]-1),
	}
}
