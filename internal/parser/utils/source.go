package utils

import (
	"slices"

	"github.com/cloudflare/pint/internal/parser"

	"github.com/prometheus/prometheus/model/labels"
	promParser "github.com/prometheus/prometheus/promql/parser"
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

type Source struct {
	Selector         *promParser.VectorSelector
	Call             *promParser.Call
	Operation        string
	IncludedLabels   []string // Labels that are included by filters, they will be present if exist on source series (by).
	ExcludedLabels   []string // Labels guaranteed to be excluded from the results (without).
	GuaranteedLabels []string // Labels guaranteed to be present on the results (matchers).
	Alternatives     []Source // Alternative lable sources
	Type             SourceType
	FixedLabels      bool // Labels are fixed and only allowed labels can be present.
}

func LabelsSource(node *parser.PromQLNode) Source {
	return walkNode(node.Expr)
}

func walkNode(node promParser.Node) (s Source) {
	switch n := node.(type) {
	case *promParser.AggregateExpr:
		switch n.Op {
		case promParser.SUM:
			s = parseAggregation(n)
			s.Operation = "sum"
		case promParser.MIN:
			s = parseAggregation(n)
			s.Operation = "min"
		case promParser.MAX:
			s = parseAggregation(n)
			s.Operation = "max"
		case promParser.AVG:
			s = parseAggregation(n)
			s.Operation = "avg"
		case promParser.GROUP:
			s = parseAggregation(n)
			s.Operation = "group"
		case promParser.STDDEV:
			s = parseAggregation(n)
			s.Operation = "stddev"
		case promParser.STDVAR:
			s = parseAggregation(n)
			s.Operation = "stdvar"
		case promParser.COUNT:
			s = parseAggregation(n)
			s.Operation = "count"
		case promParser.COUNT_VALUES:
			s = parseAggregation(n)
			s.Operation = "count_values"
			// Param is the label to store the count value in.
			s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, n.Param.(*promParser.StringLiteral).Val)
			s.IncludedLabels = appendToSlice(s.IncludedLabels, n.Param.(*promParser.StringLiteral).Val)
		case promParser.QUANTILE:
			s = parseAggregation(n)
			s.Operation = "quantile"
		case promParser.TOPK:
			s = walkNode(n.Expr)
			s.Type = AggregateSource
			s.Operation = "topk"
		case promParser.BOTTOMK:
			s = walkNode(n.Expr)
			s.Type = AggregateSource
			s.Operation = "bottomk"
			/*
				TODO these are experimental and promParser.EnableExperimentalFunctions must be set to true to enable parsing of these.
					case promParser.LIMITK:
						s = walkNode(n.Expr)
						s.Type = AggregateSource
						s.Operation = "limitk"
					case promParser.LIMIT_RATIO:
						s = walkNode(n.Expr)
						s.Type = AggregateSource
						s.Operation = "limit_ratio"
			*/
		}

	case *promParser.BinaryExpr:
		switch {
		case n.VectorMatching == nil:
			s = walkNode(n.LHS)
			if s.Type == NumberSource {
				s = walkNode(n.RHS)
			}
		case n.VectorMatching.Card == promParser.CardOneToOne:
			s = walkNode(n.LHS)
			if n.VectorMatching.On {
				s.FixedLabels = true
			}
		case n.VectorMatching.Card == promParser.CardOneToMany:
			s = walkNode(n.RHS)
		case n.VectorMatching.Card == promParser.CardManyToMany:
			s = walkNode(n.LHS)
			if n.Op == promParser.LOR {
				s.Alternatives = append(s.Alternatives, walkNode(n.RHS))
			}
		case n.VectorMatching.Card == promParser.CardManyToOne:
			s = walkNode(n.LHS)
		}
		if n.VectorMatching != nil {
			if s.Operation == "" {
				s.Operation = n.VectorMatching.Card.String()
			}
			s.IncludedLabels = appendToSlice(s.IncludedLabels, n.VectorMatching.Include...)
			// on=true when using on(), on=false when using ignore()
			if n.VectorMatching.On {
				s.IncludedLabels = appendToSlice(s.IncludedLabels, n.VectorMatching.MatchingLabels...)
			}
			s.ExcludedLabels = removeFromSlice(s.ExcludedLabels, n.VectorMatching.Include...)
		}

	case *promParser.Call:
		s = parseCall(n)

	case *promParser.MatrixSelector:
		s = walkNode(n.VectorSelector)

	case *promParser.SubqueryExpr:
		s = walkNode(n.Expr)

	case *promParser.NumberLiteral:
		s.Type = NumberSource

	case *promParser.ParenExpr:
		s = walkNode(n.Expr)

	case *promParser.StringLiteral:
		s.Type = StringSource

	case *promParser.UnaryExpr:
		s = walkNode(n.Expr)

	case *promParser.StepInvariantExpr:
		// Not possible to get this from the parser.

	case *promParser.VectorSelector:
		s.Type = SelectorSource
		s.Selector = n
		// Any label used in positive filters is gurnateed to be present.
		for _, lm := range s.Selector.LabelMatchers {
			if lm.Name == labels.MetricName {
				continue
			}
			if lm.Type == labels.MatchEqual || lm.Type == labels.MatchRegexp {
				s.GuaranteedLabels = appendToSlice(s.GuaranteedLabels, lm.Name)
			}
		}

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

func parseAggregation(n *promParser.AggregateExpr) (s Source) {
	s = walkNode(n.Expr)
	if n.Without {
		s.ExcludedLabels = appendToSlice(s.ExcludedLabels, n.Grouping...)
		s.IncludedLabels = removeFromSlice(s.IncludedLabels, n.Grouping...)
		s.GuaranteedLabels = removeFromSlice(s.GuaranteedLabels, n.Grouping...)
	} else {
		s.FixedLabels = true
		if len(n.Grouping) == 0 {
			s.IncludedLabels = nil
			s.GuaranteedLabels = nil
		} else {
			s.IncludedLabels = appendToSlice(s.IncludedLabels, n.Grouping...)
			for _, name := range n.Grouping {
				s.ExcludedLabels = removeFromSlice(s.ExcludedLabels, name)
			}
		}
	}
	s.Type = AggregateSource
	s.Call = nil
	return s
}

func parseCall(n *promParser.Call) (s Source) {
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
			s.Selector = walkNode(e).Selector
		}
	}

	if n.Func.Name == "absent" {
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
	}

	return s
}
