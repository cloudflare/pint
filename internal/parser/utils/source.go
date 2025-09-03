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

type SourceType uint8

const (
	UnknownSource SourceType = iota
	NumberSource
	StringSource
	SelectorSource
	FuncSource
	AggregateSource
)

// Used for test snapshots.
func (st SourceType) MarshalYAML() (any, error) {
	var name string
	switch st { // nolint: exhaustive
	case NumberSource:
		name = "number"
	case StringSource:
		name = "string"
	case SelectorSource:
		name = "selector"
	case FuncSource:
		name = "function"
	case AggregateSource:
		name = "aggregation"
	}
	return name, nil
}

type LabelPromiseType uint8

const (
	ImpossibleLabel LabelPromiseType = iota
	PossibleLabel
	GuaranteedLabel
)

// Used for test snapshots.
func (lpt LabelPromiseType) MarshalYAML() (any, error) {
	var name string
	switch lpt {
	case ImpossibleLabel:
		name = "excluded"
	case PossibleLabel:
		name = "included"
	case GuaranteedLabel:
		name = "guaranteed"
	}
	return name, nil
}

type LabelTransform struct {
	Reason   string
	Kind     LabelPromiseType
	Fragment posrange.PositionRange
}

type DeadInfo struct {
	Reason   string
	Fragment posrange.PositionRange
}

type ReturnInfo struct {
	LogicalExpr    string
	ValuePosition  posrange.PositionRange
	ReturnedNumber float64 // If AlwaysReturns=true this is the number that's returned
	AlwaysReturns  bool    // True if this source always returns results.
	KnownReturn    bool    // True if we always know the return value.
	IsReturnBool   bool    // True if this source uses the 'bool' modifier.
}

type SourceOperation struct {
	Node      promParser.Node
	Operation string
	Arguments []string
}

// Used for test snapshots.
func (so SourceOperation) MarshalYAML() (any, error) {
	y := map[string]any{
		"op":   so.Operation,
		"node": fmt.Sprintf("[%T] %s", so.Node, so.Node.String()),
	}
	if so.Arguments != nil {
		y["args"] = so.Arguments
	}
	return y, nil
}

type SourceOperations []SourceOperation

func MostOuterOperation[T promParser.Node](s Source) (T, bool) {
	for i := len(s.Operations) - 1; i >= 0; i-- {
		op := s.Operations[i]
		if o, ok := op.Node.(T); ok {
			return o, true
		}
	}
	return *new(T), false
}

func newSource() Source {
	return Source{ // nolint: exhaustruct
		Labels: map[string]LabelTransform{},
	}
}

type Join struct {
	On          []string
	Ignoring    []string
	AddedLabels []string
	Src         Source              // The source we're joining with.
	Op          promParser.ItemType // The binary operation used for this join.
	Depth       int                 // Zero if this is a direct join, non-zero otherwise. sum(foo * bar) would be in-direct join.
}

type Unless struct {
	On       []string
	Ignoring []string
	Src      Source
}

type Source struct {
	Labels        map[string]LabelTransform
	DeadInfo      *DeadInfo
	Returns       promParser.ValueType
	Operations    SourceOperations
	Joins         []Join   // Any other sources this source joins with.
	Unless        []Unless // Any other sources this source is suppressed by.
	ReturnInfo    ReturnInfo
	Position      posrange.PositionRange
	Type          SourceType
	FixedLabels   bool // Labels are fixed and only allowed labels can be present.
	IsConditional bool // True if this source is guarded by 'foo > 5' or other condition.
}

func (s Source) Operation() string {
	if len(s.Operations) > 0 {
		return s.Operations[len(s.Operations)-1].Operation
	}
	return ""
}

func (s Source) CanHaveLabel(name string) bool {
	if v, ok := s.Labels[name]; ok {
		if v.Kind == ImpossibleLabel {
			return false
		}
		if v.Kind == PossibleLabel || v.Kind == GuaranteedLabel {
			return true
		}
	}
	return !s.FixedLabels
}

func (s Source) TransformedLabels(kinds ...LabelPromiseType) []string {
	names := make([]string, 0, len(s.Labels))
	for name, l := range s.Labels {
		if slices.Contains(kinds, l.Kind) {
			names = append(names, name)
		}
	}
	return names
}

func (s Source) LabelExcludeReason(name string) (string, posrange.PositionRange) {
	if l, ok := s.Labels[name]; ok && l.Kind == ImpossibleLabel {
		return l.Reason, l.Fragment
	}
	return s.Labels[""].Reason, s.Labels[""].Fragment
}

func (s *Source) excludeAllLabels(reason string, fragment posrange.PositionRange, except []string) {
	// Everything that was included until now but will be removed needs an explicit stamp to mark it as gone.
	for name, l := range s.Labels {
		if slices.Contains(except, name) {
			continue
		}
		if l.Kind == PossibleLabel || l.Kind == GuaranteedLabel {
			s.Labels[name] = LabelTransform{
				Kind:     ImpossibleLabel,
				Reason:   reason,
				Fragment: fragment,
			}
		}
	}
	// Mark except labels as possible, unless they are already guaranteed.
	for _, name := range except {
		if l, ok := s.Labels[name]; ok && l.Kind == GuaranteedLabel {
			continue
		}

		// We have grouping labels set, if they are possible mark them as such, if not mark as impossible.
		if s.CanHaveLabel(name) {
			s.Labels[name] = LabelTransform{
				Kind:     PossibleLabel,
				Reason:   reason,
				Fragment: fragment,
			}
		} else {
			r, f := s.LabelExcludeReason(name)
			s.Labels[name] = LabelTransform{
				Kind:     ImpossibleLabel,
				Reason:   r,
				Fragment: f,
			}
		}

	}
	s.Labels[""] = LabelTransform{
		Kind:     ImpossibleLabel,
		Reason:   reason,
		Fragment: fragment,
	}
	s.FixedLabels = true
}

func (s *Source) excludeLabel(reason string, fragment posrange.PositionRange, names ...string) {
	for _, name := range names {
		s.Labels[name] = LabelTransform{
			Kind:     ImpossibleLabel,
			Reason:   reason,
			Fragment: fragment,
		}
	}
}

func (s *Source) includeLabel(reason string, fragment posrange.PositionRange, names ...string) {
	for _, name := range names {
		if l, ok := s.Labels[name]; ok && l.Kind == GuaranteedLabel {
			continue
		}
		s.Labels[name] = LabelTransform{
			Kind:     PossibleLabel,
			Reason:   reason,
			Fragment: fragment,
		}
	}
}

func (s *Source) guaranteeLabel(reason string, fragment posrange.PositionRange, names ...string) {
	for _, name := range names {
		s.Labels[name] = LabelTransform{
			Kind:     GuaranteedLabel,
			Reason:   reason,
			Fragment: fragment,
		}
	}
}

type Visitor func(s Source, j *Join, u *Unless)

func innerWalk(fn Visitor, s Source, j *Join, u *Unless) {
	fn(s, j, u)
	for _, j := range s.Joins {
		innerWalk(fn, j.Src, &j, nil)
	}
	for _, u := range s.Unless {
		innerWalk(fn, u.Src, nil, &u)
	}
}

func (s Source) WalkSources(fn Visitor) {
	innerWalk(fn, s, nil, nil)
}

func LabelsSource(expr string, node promParser.Node) (src []Source) {
	return walkNode(expr, node)
}

func walkNode(expr string, node promParser.Node) (src []Source) {
	s := newSource()
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
		s.ReturnInfo.KnownReturn = true
		s.ReturnInfo.ReturnedNumber = n.Val
		s.ReturnInfo.AlwaysReturns = true
		s.ReturnInfo.ValuePosition = n.PosRange
		s.excludeAllLabels("This query returns a number value with no labels.", n.PosRange, nil)
		s.Position = n.PosRange
		src = append(src, s)

	case *promParser.ParenExpr:
		src = append(src, walkNode(expr, n.Expr)...)

	case *promParser.StringLiteral:
		s.Type = StringSource
		s.Returns = promParser.ValueTypeString
		s.ReturnInfo.AlwaysReturns = true
		s.excludeAllLabels("This query returns a string value with no labels.", n.PosRange, nil)
		s.Position = n.PosRange
		src = append(src, s)

	case *promParser.UnaryExpr:
		src = append(src, walkNode(expr, n.Expr)...)

	case *promParser.StepInvariantExpr:
		// Not possible to get this from the parser.

	case *promParser.VectorSelector:
		s.Type = SelectorSource
		s.Returns = promParser.ValueTypeVector
		s.Operations = append(s.Operations, SourceOperation{
			Operation: "",
			Node:      n,
			Arguments: nil,
		})
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, n)...,
		)
		for _, name := range labelsWithEmptyValueSelector(n) {
			s.excludeLabel(
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

func appendToSlice(dst []string, values ...string) []string {
	for _, v := range values {
		if !slices.Contains(dst, v) {
			dst = append(dst, v)
		}
	}
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
	s := newSource()

	var args []string
	if n.Param != nil {
		args = append(args, n.Param.String())
	}

	switch n.Op {
	case promParser.SUM:
		for _, s = range parseAggregation(expr, n) {
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "sum",
				Node:      n,
				Arguments: args,
			})
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s.excludeLabel("Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.MIN:
		for _, s = range parseAggregation(expr, n) {
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "min",
				Node:      n,
				Arguments: args,
			})
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s.excludeLabel("Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.MAX:
		for _, s = range parseAggregation(expr, n) {
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "max",
				Node:      n,
				Arguments: args,
			})
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s.excludeLabel("Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.AVG:
		for _, s = range parseAggregation(expr, n) {
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "avg",
				Node:      n,
				Arguments: args,
			})
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s.excludeLabel("Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.GROUP:
		for _, s = range parseAggregation(expr, n) {
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "group",
				Node:      n,
				Arguments: args,
			})
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s.excludeLabel("Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.STDDEV:
		for _, s = range parseAggregation(expr, n) {
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "stddev",
				Node:      n,
				Arguments: args,
			})
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s.excludeLabel("Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.STDVAR:
		for _, s = range parseAggregation(expr, n) {
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "stdvar",
				Node:      n,
				Arguments: args,
			})
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s.excludeLabel("Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.COUNT:
		for _, s = range parseAggregation(expr, n) {
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "count",
				Node:      n,
				Arguments: args,
			})
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s.excludeLabel("Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.COUNT_VALUES:
		for _, s = range parseAggregation(expr, n) {
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "count_values",
				Node:      n,
				Arguments: args,
			})
			// Param is the label to store the count value in.
			s.guaranteeLabel(
				"This label will be added to the results by the count_values() call.",
				n.PosRange,
				n.Param.(*promParser.StringLiteral).Val,
			)
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s.excludeLabel("Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.QUANTILE:
		for _, s = range parseAggregation(expr, n) {
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "quantile",
				Node:      n,
				Arguments: args,
			})
			if n.Without || !slices.Contains(n.Grouping, labels.MetricName) {
				s.excludeLabel("Aggregation removes metric name.", n.PosRange, labels.MetricName)
			}
			src = append(src, s)
		}
	case promParser.TOPK:
		for _, s = range walkNode(expr, n.Expr) {
			for i := range s.Joins {
				s.Joins[i].Depth++
			}
			s.Type = AggregateSource
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "topk",
				Node:      n,
				Arguments: args,
			})
			src = append(src, s)
		}
	case promParser.BOTTOMK:
		for _, s = range walkNode(expr, n.Expr) {
			for i := range s.Joins {
				s.Joins[i].Depth++
			}
			s.Type = AggregateSource
			s.Operations = append(s.Operations, SourceOperation{
				Operation: "bottomk",
				Node:      n,
				Arguments: args,
			})
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
	s := newSource()
	for _, s = range walkNode(expr, n.Expr) {
		// If we have sum(foo * bar) then we start with:
		// - source: foo
		//   joins: bar
		// Then we parse aggregation and end up with:
		// - source: sum(foo)
		//   joins: bar
		// Which is misleading and wrong, so we bump depth value to know about it.
		for i := range s.Joins {
			s.Joins[i].Depth++
		}

		if n.Without {
			s.excludeLabel(
				fmt.Sprintf("Query is using aggregation with `without(%s)`, all labels included inside `without(...)` will be removed from the results.",
					strings.Join(n.Grouping, ", ")),
				FindPosition(expr, n.PosRange, "without"),
				n.Grouping...,
			)
		} else {
			if len(n.Grouping) == 0 {
				s.excludeAllLabels(
					"Query is using aggregation that removes all labels.",
					FindPosition(expr, n.PosRange, "sum"),
					nil,
				)
			} else {
				s.excludeAllLabels(
					fmt.Sprintf("Query is using aggregation with `by(%s)`, only labels included inside `by(...)` will be present on the results.",
						strings.Join(n.Grouping, ", ")),
					FindPosition(expr, n.PosRange, "by"),
					n.Grouping,
				)
			}
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
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, vs)...,
		)

	case "ceil", "floor", "round":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, vs)...,
		)

	case "changes", "resets":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, vs)...,
		)

	case "clamp", "clamp_max", "clamp_min":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, vs)...,
		)

	case "absent", "absent_over_time":
		s.Returns = promParser.ValueTypeVector
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		names := labelsFromSelectors([]labels.MatchType{labels.MatchEqual}, vs)
		s.excludeAllLabels(
			fmt.Sprintf(`The [%s()](https://prometheus.io/docs/prometheus/latest/querying/functions/#%s) function is used to check if provided query doesn't match any time series.
You will only get any results back if the metric selector you pass doesn't match anything.
Since there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.
This means that the only labels you can get back from absent call are the ones you pass to it.
If you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `+"`up`"+` metric instead.`,
				n.Func.Name, n.Func.Name),
			FindPosition(expr, n.PosRange, n.Func.Name),
			names,
		)
		s.guaranteeLabel(
			fmt.Sprintf("All labels passed to %s() call will be present on the results if the query doesn't match anything.", n.Func.Name),
			n.PosRange,
			names...,
		)

	case "avg_over_time", "count_over_time", "last_over_time", "max_over_time", "min_over_time", "present_over_time", "quantile_over_time", "stddev_over_time", "stdvar_over_time", "sum_over_time":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, vs)...,
		)

	case "days_in_month", "day_of_month", "day_of_week", "day_of_year", "hour", "minute", "month", "year":
		s.Returns = promParser.ValueTypeVector
		// No labels if we don't pass any arguments.
		// Otherwise no change to labels.
		if len(n.Args) == 0 {
			s.ReturnInfo.AlwaysReturns = true
			s.excludeAllLabels(
				fmt.Sprintf("Calling `%s()` with no arguments will return an empty time series with no labels.",
					n.Func.Name),
				n.PosRange,
				nil,
			)
		} else {
			vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
			s.guaranteeLabel(
				"Query will only return series where these labels are present.",
				n.PosRange,
				labelsFromSelectors(guaranteedLabelsMatches, vs)...,
			)
		}

	case "deg", "rad", "ln", "log10", "log2", "sqrt", "exp":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, vs)...,
		)
	case "delta", "idelta", "increase", "deriv", "irate", "rate":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, vs)...,
		)

	case "histogram_avg", "histogram_count", "histogram_sum", "histogram_stddev", "histogram_stdvar", "histogram_fraction", "histogram_quantile":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, vs)...,
		)

	case "holt_winters", "predict_linear":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, vs)...,
		)
	case "label_replace", "label_join":
		// One label added to the results.
		s.Returns = promParser.ValueTypeVector
		s.guaranteeLabel(
			fmt.Sprintf("This label will be added to the result by %s() call.", n.Func.Name),
			n.PosRange,
			n.Args[1].(*promParser.StringLiteral).Val,
		)

	case "pi":
		s.Returns = promParser.ValueTypeScalar
		s.ReturnInfo.AlwaysReturns = true
		s.ReturnInfo.KnownReturn = true
		s.ReturnInfo.ReturnedNumber = math.Pi
		s.ReturnInfo.ValuePosition = n.PosRange
		s.excludeAllLabels(
			fmt.Sprintf("Calling `%s()` will return a scalar value with no labels.", n.Func.Name),
			n.PosRange,
			nil,
		)

	case "scalar":
		s.Returns = promParser.ValueTypeScalar
		s.ReturnInfo.AlwaysReturns = true
		s.excludeAllLabels(
			fmt.Sprintf("Calling `%s()` will return a scalar value with no labels.", n.Func.Name),
			FindPosition(expr, n.PositionRange(), n.Func.Name),
			nil,
		)

	case "sort", "sort_desc":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector

	case "time":
		s.Returns = promParser.ValueTypeScalar
		s.ReturnInfo.AlwaysReturns = true
		s.ReturnInfo.ValuePosition = n.PosRange
		s.excludeAllLabels(
			fmt.Sprintf("Calling `%s()` will return a scalar value with no labels.", n.Func.Name),
			n.PosRange,
			nil,
		)

	case "timestamp":
		// No change to labels.
		s.Returns = promParser.ValueTypeVector
		vs, _ := MostOuterOperation[*promParser.VectorSelector](s)
		s.guaranteeLabel(
			"Query will only return series where these labels are present.",
			n.PosRange,
			labelsFromSelectors(guaranteedLabelsMatches, vs)...,
		)

	case "vector":
		s.Returns = promParser.ValueTypeVector
		s.ReturnInfo.AlwaysReturns = true
		s.ReturnInfo.ValuePosition = n.PosRange
		for _, vs := range walkNode(expr, n.Args[0]) {
			if vs.ReturnInfo.KnownReturn {
				s.ReturnInfo.ReturnedNumber = vs.ReturnInfo.ReturnedNumber
				s.ReturnInfo.KnownReturn = true
			}
		}
		s.excludeAllLabels(
			fmt.Sprintf("Calling `%s()` will return a vector value with no labels.", n.Func.Name),
			FindPosition(expr, n.PosRange, n.Func.Name),
			nil,
		)

	default:
		// Unsupported function
		return Source{} // nolint: exhaustruct
	}
	return s
}

func parseCall(expr string, n *promParser.Call) (src []Source) {
	var args []string
	var exprs []promParser.Expr

	var vt promParser.ValueType
	for i, e := range n.Args {
		if i >= len(n.Func.ArgTypes) {
			vt = n.Func.ArgTypes[len(n.Func.ArgTypes)-1]
		} else {
			vt = n.Func.ArgTypes[i]
		}

		switch vt {
		case promParser.ValueTypeVector, promParser.ValueTypeMatrix:
			exprs = append(exprs, e)
		case promParser.ValueTypeNone, promParser.ValueTypeScalar, promParser.ValueTypeString:
			args = append(args, e.String())
		}
	}

	for _, e := range exprs {
		for _, es := range walkNode(expr, e) {
			es.Type = FuncSource
			es.Operations = append(es.Operations, SourceOperation{
				Operation: n.Func.Name,
				Node:      n,
				Arguments: args,
			})
			es.Position = e.PositionRange()
			src = append(src, parsePromQLFunc(es, expr, n))
		}
	}

	if len(src) == 0 {
		s := Source{ // nolint: exhaustruct
			Labels: map[string]LabelTransform{},
			Type:   FuncSource,
			Operations: SourceOperations{
				{
					Operation: n.Func.Name,
					Node:      n,
					Arguments: args,
				},
			},
			Position: n.PosRange,
		}
		src = append(src, parsePromQLFunc(s, expr, n))
	}

	return src
}

func parseBinOps(expr string, n *promParser.BinaryExpr) (src []Source) {
	switch {
	// foo{} + 1
	// 1 + foo{}
	// foo{} > 1
	// 1 > foo{}
	case n.VectorMatching == nil:
		lhs := walkNode(expr, n.LHS)
		rhs := walkNode(expr, n.RHS)
		for _, ls := range lhs {
			ls.IsConditional, ls.ReturnInfo.IsReturnBool = checkConditions(ls, n.Op, n.ReturnBool)
			for _, rs := range rhs {
				rs.IsConditional, rs.ReturnInfo.IsReturnBool = checkConditions(rs, n.Op, n.ReturnBool)
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
				if ls.ReturnInfo.AlwaysReturns && rs.ReturnInfo.AlwaysReturns && ls.ReturnInfo.KnownReturn && rs.ReturnInfo.KnownReturn {
					// Both sides always return something
					side.ReturnInfo, side.DeadInfo = calculateStaticReturn(expr, ls, rs, n)
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
		for _, ls := range walkNode(expr, n.LHS) {
			if n.VectorMatching.On {
				ls.excludeAllLabels(
					fmt.Sprintf(
						"Query is using %s vector matching with `on(%s)`, only labels included inside `on(...)` will be present on the results.",
						n.VectorMatching.Card, strings.Join(n.VectorMatching.MatchingLabels, ", "),
					),
					FindPosition(expr, n.PositionRange(), "on"),
					n.VectorMatching.MatchingLabels,
				)
			} else {
				ls.excludeLabel(
					fmt.Sprintf(
						"Query is using %s vector matching with `ignoring(%s)`, all labels included inside `ignoring(...)` will be removed on the results.",
						n.VectorMatching.Card, strings.Join(n.VectorMatching.MatchingLabels, ", "),
					),
					FindPosition(expr, n.PositionRange(), "ignoring"),
					n.VectorMatching.MatchingLabels...,
				)
				for _, rs := range rhs {
					rs.IsConditional, rs.ReturnInfo.IsReturnBool = checkConditions(rs, n.Op, n.ReturnBool)
					if ls.ReturnInfo.AlwaysReturns && rs.ReturnInfo.AlwaysReturns && ls.ReturnInfo.KnownReturn && rs.ReturnInfo.KnownReturn {
						// Both sides always return something
						ls.ReturnInfo, ls.DeadInfo = calculateStaticReturn(expr, ls, rs, n)
					}
				}
			}
			for _, rs := range rhs {
				if ok, s, pos := canJoin(ls, rs, n.VectorMatching); !ok {
					rs.DeadInfo = &DeadInfo{
						Reason:   s,
						Fragment: pos,
					}
				}
				ls.Joins = append(ls.Joins, Join{
					Src:         rs,
					Op:          n.Op,
					Depth:       0,
					On:          onLabels(n.VectorMatching),
					Ignoring:    ignoringLabels(n.VectorMatching),
					AddedLabels: n.VectorMatching.Include,
				})
			}
			ls.IsConditional, ls.ReturnInfo.IsReturnBool = checkConditions(ls, n.Op, n.ReturnBool)
			src = append(src, ls)
		}

		// foo{} + on(...)       group_right(...) bar{}
		// foo{} + ignoring(...) group_right(...) bar{}
	case n.VectorMatching.Card == promParser.CardOneToMany:
		lhs := walkNode(expr, n.LHS)
		for _, rs := range walkNode(expr, n.RHS) {
			rs.includeLabel(
				fmt.Sprintf(
					"Query is using %s vector matching with `group_right(%s)`, all labels included inside `group_right(...)` will be include on the results.",
					n.VectorMatching.Card, strings.Join(n.VectorMatching.Include, ", "),
				),
				FindPosition(expr, n.PositionRange(), "group_right"),
				n.VectorMatching.Include...,
			)
			// If we have:
			// foo * on(instance) group_left(a,b) bar{x="y"}
			// then only group_left() labels will be included.
			if n.VectorMatching.On {
				rs.includeLabel(
					fmt.Sprintf(
						"Query is using %s vector matching with `on(%s)`, labels included inside `on(...)` will be present on the results.",
						n.VectorMatching.Card, strings.Join(n.VectorMatching.MatchingLabels, ", "),
					),
					FindPosition(expr, n.PositionRange(), "on"),
					n.VectorMatching.MatchingLabels...,
				)
			}
			for _, ls := range lhs {
				if ok, s, pos := canJoin(rs, ls, n.VectorMatching); !ok {
					ls.DeadInfo = &DeadInfo{
						Reason:   s,
						Fragment: pos,
					}
				}
				rs.Joins = append(rs.Joins, Join{
					Src:         ls,
					Op:          n.Op,
					Depth:       0,
					On:          onLabels(n.VectorMatching),
					Ignoring:    ignoringLabels(n.VectorMatching),
					AddedLabels: n.VectorMatching.Include,
				})
			}
			rs.IsConditional, rs.ReturnInfo.IsReturnBool = checkConditions(rs, n.Op, n.ReturnBool)
			src = append(src, rs)
		}

		// foo{} + on(...)       group_left(...) bar{}
		// foo{} + ignoring(...) group_left(...) bar{}
	case n.VectorMatching.Card == promParser.CardManyToOne:
		rhs := walkNode(expr, n.RHS)
		for _, ls := range walkNode(expr, n.LHS) {
			ls.includeLabel(
				fmt.Sprintf(
					"Query is using %s vector matching with `group_left(%s)`, all labels included inside `group_left(...)` will be include on the results.",
					n.VectorMatching.Card, strings.Join(n.VectorMatching.Include, ", "),
				),
				FindPosition(expr, n.PositionRange(), "group_left"),
				n.VectorMatching.Include...,
			)
			if n.VectorMatching.On {
				ls.includeLabel(
					fmt.Sprintf(
						"Query is using %s vector matching with `on(%s)`, labels included inside `on(...)` will be present on the results.",
						n.VectorMatching.Card, strings.Join(n.VectorMatching.MatchingLabels, ", "),
					),
					FindPosition(expr, n.PositionRange(), "on"),
					n.VectorMatching.MatchingLabels...,
				)
			}
			for _, rs := range rhs {
				if ok, s, pos := canJoin(ls, rs, n.VectorMatching); !ok {
					rs.DeadInfo = &DeadInfo{
						Reason:   s,
						Fragment: pos,
					}
				}
				ls.Joins = append(ls.Joins, Join{
					Src:         rs,
					Op:          n.Op,
					Depth:       0,
					On:          onLabels(n.VectorMatching),
					Ignoring:    ignoringLabels(n.VectorMatching),
					AddedLabels: n.VectorMatching.Include,
				})
			}
			ls.IsConditional, ls.ReturnInfo.IsReturnBool = checkConditions(ls, n.Op, n.ReturnBool)
			src = append(src, ls)
		}

		// foo{} and on(...)       bar{}
		// foo{} and ignoring(...) bar{}
		// foo{} unless bar{}
	case n.VectorMatching.Card == promParser.CardManyToMany:
		var lhsCanBeEmpty bool // true if any of the LHS query can produce empty results.
		rhs := walkNode(expr, n.RHS)
		for _, ls := range walkNode(expr, n.LHS) {
			var rhsConditional bool
			if n.VectorMatching.On {
				ls.includeLabel(
					fmt.Sprintf(
						"Query is using %s vector matching with `on(%s)`, labels included inside `on(...)` will be present on the results.",
						n.VectorMatching.Card, strings.Join(n.VectorMatching.MatchingLabels, ", "),
					),
					FindPosition(expr, n.PositionRange(), "on"),
					n.VectorMatching.MatchingLabels...,
				)
			}
			if !ls.ReturnInfo.AlwaysReturns || ls.IsConditional {
				lhsCanBeEmpty = true
			}
			for _, rs := range rhs {
				isConditional, _ := checkConditions(rs, n.Op, n.ReturnBool)
				if isConditional {
					rhsConditional = true
				}
				if ok, s, pos := canJoin(ls, rs, n.VectorMatching); !ok {
					rs.DeadInfo = &DeadInfo{
						Reason:   s,
						Fragment: pos,
					}
				}
				switch {
				case n.Op == promParser.LUNLESS:
					if n.VectorMatching.On && len(n.VectorMatching.MatchingLabels) == 0 && rs.ReturnInfo.AlwaysReturns && !rs.IsConditional {
						ls.DeadInfo = &DeadInfo{
							Reason:   "This query will never return anything because the `unless` query always returns something.",
							Fragment: rs.Position,
						}
					}
					ls.Unless = append(ls.Unless, Unless{
						Src:      rs,
						On:       onLabels(n.VectorMatching),
						Ignoring: ignoringLabels(n.VectorMatching),
					})
				case n.Op != promParser.LOR:
					ls.Joins = append(ls.Joins, Join{
						Src:         rs,
						Op:          n.Op,
						Depth:       0,
						On:          onLabels(n.VectorMatching),
						Ignoring:    ignoringLabels(n.VectorMatching),
						AddedLabels: n.VectorMatching.Include,
					})
				}
			}
			if n.Op == promParser.LAND && rhsConditional {
				ls.IsConditional = true
			}
			src = append(src, ls)
		}
		if n.Op == promParser.LOR {
			for _, rs := range rhs {
				// If LHS can NOT be empty then RHS is dead code.
				if !lhsCanBeEmpty {
					rs.DeadInfo = &DeadInfo{
						Reason:   "The left hand side always returs something and so the right hand side is never used.",
						Fragment: rs.Position,
					}
				}
				src = append(src, rs)
			}
		}
	}
	return src
}

func onLabels(vm *promParser.VectorMatching) []string {
	if vm.On {
		return vm.MatchingLabels
	}
	return nil
}

func ignoringLabels(vm *promParser.VectorMatching) []string {
	if !vm.On {
		return vm.MatchingLabels
	}
	return nil
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
				reason, fragment := rs.LabelExcludeReason(name)
				return false, fmt.Sprintf("The %s hand side will never be matched because it doesn't have the `%s` label from `on(...)`. %s",
					side, name, reason), fragment
			}
		}
	default: // ls unless rs
		for name, l := range ls.Labels {
			if l.Kind != GuaranteedLabel {
				continue
			}
			if ls.CanHaveLabel(name) && !rs.CanHaveLabel(name) {
				reason, fragment := rs.LabelExcludeReason(name)
				return false, fmt.Sprintf("The %s hand side will never be matched because it doesn't have the `%s` label while the left hand side will. %s",
					side, name, reason), fragment
			}
		}
	}
	return true, "", posrange.PositionRange{}
}

func describeDeadCode(expr string, ls, rs Source, op *promParser.BinaryExpr, match string) *DeadInfo {
	var lse, rse string
	if ls.ReturnInfo.LogicalExpr != "" {
		lse = ls.ReturnInfo.LogicalExpr
	} else {
		lse = GetQueryFragment(expr, ls.ReturnInfo.ValuePosition)
	}
	if rs.ReturnInfo.LogicalExpr != "" {
		rse = rs.ReturnInfo.LogicalExpr
	} else {
		rse = GetQueryFragment(expr, rs.ReturnInfo.ValuePosition)
	}

	cmpPrefix := fmt.Sprintf("`%s %s %s` always evaluates to", lse, op.Op, rse)

	var cmpSuffix string
	if op.ReturnBool {
		cmpSuffix = "and uses the `bool` modifier which means it will always return 0"
	} else {
		cmpSuffix = "which is not possible, so it will never return anything."
	}
	return &DeadInfo{
		Reason: fmt.Sprintf(
			"%s `%s %s %s` %s",
			cmpPrefix,
			strconv.FormatFloat(ls.ReturnInfo.ReturnedNumber, 'f', -1, 64),
			match,
			strconv.FormatFloat(rs.ReturnInfo.ReturnedNumber, 'f', -1, 64),
			cmpSuffix,
		),
		Fragment: ls.Position,
	}
}

func calculateStaticReturn(expr string, ls, rs Source, op *promParser.BinaryExpr) (ret ReturnInfo, deadinfo *DeadInfo) {
	ret = ls.ReturnInfo
	switch op.Op {
	case promParser.EQLC:
		if ls.ReturnInfo.ReturnedNumber != rs.ReturnInfo.ReturnedNumber {
			deadinfo = describeDeadCode(expr, ls, rs, op, "==")
		}
	case promParser.NEQ:
		if ls.ReturnInfo.ReturnedNumber == rs.ReturnInfo.ReturnedNumber {
			deadinfo = describeDeadCode(expr, ls, rs, op, "!=")
		}
	case promParser.LTE:
		if ls.ReturnInfo.ReturnedNumber > rs.ReturnInfo.ReturnedNumber {
			deadinfo = describeDeadCode(expr, ls, rs, op, "<=")
		}
	case promParser.LSS:
		if ls.ReturnInfo.ReturnedNumber >= rs.ReturnInfo.ReturnedNumber {
			deadinfo = describeDeadCode(expr, ls, rs, op, "<")
		}
	case promParser.GTE:
		if ls.ReturnInfo.ReturnedNumber < rs.ReturnInfo.ReturnedNumber {
			deadinfo = describeDeadCode(expr, ls, rs, op, ">=")
		}
	case promParser.GTR:
		if ls.ReturnInfo.ReturnedNumber <= rs.ReturnInfo.ReturnedNumber {
			deadinfo = describeDeadCode(expr, ls, rs, op, ">")
		}
	case promParser.ADD:
		ret.ReturnedNumber = ls.ReturnInfo.ReturnedNumber + rs.ReturnInfo.ReturnedNumber
		ret.LogicalExpr = formatDesc(expr, ls, rs, "+")
	case promParser.SUB:
		ret.ReturnedNumber = ls.ReturnInfo.ReturnedNumber - rs.ReturnInfo.ReturnedNumber
		ret.LogicalExpr = formatDesc(expr, ls, rs, "-")
	case promParser.MUL:
		ret.ReturnedNumber = ls.ReturnInfo.ReturnedNumber * rs.ReturnInfo.ReturnedNumber
		ret.LogicalExpr = formatDesc(expr, ls, rs, "*")
	case promParser.DIV:
		ret.ReturnedNumber = ls.ReturnInfo.ReturnedNumber / rs.ReturnInfo.ReturnedNumber
		ret.LogicalExpr = formatDesc(expr, ls, rs, "/")
	case promParser.MOD:
		ret.ReturnedNumber = math.Mod(ls.ReturnInfo.ReturnedNumber, rs.ReturnInfo.ReturnedNumber)
		ret.LogicalExpr = formatDesc(expr, ls, rs, "%")
	case promParser.POW:
		ret.ReturnedNumber = math.Pow(ls.ReturnInfo.ReturnedNumber, rs.ReturnInfo.ReturnedNumber)
		ret.LogicalExpr = formatDesc(expr, ls, rs, "^")
	}
	return ret, deadinfo
}

func formatDesc(expr string, ls, rs Source, op string) string {
	var lse, rse string
	if ls.ReturnInfo.LogicalExpr != "" {
		lse = ls.ReturnInfo.LogicalExpr
	} else {
		lse = GetQueryFragment(expr, ls.ReturnInfo.ValuePosition)
	}
	if rs.ReturnInfo.LogicalExpr != "" {
		rse = rs.ReturnInfo.LogicalExpr
	} else {
		rse = GetQueryFragment(expr, rs.ReturnInfo.ValuePosition)
	}
	return lse + " " + op + " " + rse
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
