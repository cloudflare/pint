package utils_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"

	promParser "github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/promql/parser/posrange"
)

func TestLabelsSource(t *testing.T) {
	type testCaseT struct {
		expr   string
		output []utils.Source
	}

	mustParseVector := func(s string, offset int) *promParser.VectorSelector {
		m, err := promParser.ParseExpr(s)
		require.NoErrorf(t, err, "failed to parse vector selector: %s", s)
		v := m.(*promParser.VectorSelector)
		v.PosRange.Start += posrange.Pos(offset)
		v.PosRange.End += posrange.Pos(offset)
		return v
	}

	mustParseMatrix := func(s string, offset int) *promParser.MatrixSelector {
		e, err := promParser.ParseExpr(s)
		require.NoErrorf(t, err, "failed to parse matrix selector: %s", s)
		m := e.(*promParser.MatrixSelector)
		m.VectorSelector = mustParseVector(m.VectorSelector.String(), offset)
		m.EndPos += posrange.Pos(offset)
		return m
	}

	testCases := []testCaseT{
		{
			expr: "1",
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{1},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "1",
						},
					},
				},
			},
		},
		{
			expr: "1 / 5",
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{0.2},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "1",
						},
					},
				},
			},
		},
		{
			expr: "(2 ^ 5) == bool 5",
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					IsDead:          true,
					ReturnedNumbers: []float64{32},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "2",
						},
					},
				},
			},
		},
		{
			expr: "(2 ^ 5 + 11) % 5 <= bool 2",
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					IsDead:          true,
					ReturnedNumbers: []float64{3},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "2",
						},
					},
				},
			},
		},
		{
			expr: "(2 ^ 5 + 11) % 5 >= bool 20",
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					IsDead:          true,
					ReturnedNumbers: []float64{3},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "2",
						},
					},
				},
			},
		},
		{
			expr: "(2 ^ 5 + 11) % 5 <= bool 3",
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{3},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "2",
						},
					},
				},
			},
		},
		{
			expr: "(2 ^ 5 + 11) % 5 < bool 1",
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					IsDead:          true,
					ReturnedNumbers: []float64{3},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "2",
						},
					},
				},
			},
		},
		{
			expr: "20 - 15 < bool 1",
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					IsDead:          true,
					ReturnedNumbers: []float64{5},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "20",
						},
					},
				},
			},
		},
		{
			expr: "2 * 5",
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{10},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "2",
						},
					},
				},
			},
		},
		{
			expr: "(foo or bar) * 5",
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector("foo", 1),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector("bar", 8),
					},
				},
			},
		},
		{
			expr: "(foo or vector(2)) * 5",
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector("foo", 1),
					},
				},
				{
					Type:            utils.FuncSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "vector",
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{10},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `vector()` will return a vector value with no labels.",
							Fragment: "vector(2)",
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "vector",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeScalar,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							&promParser.NumberLiteral{
								Val: 2,
								PosRange: posrange.PositionRange{
									Start: 15,
									End:   16,
								},
							},
						},
						PosRange: posrange.PositionRange{
							Start: 8,
							End:   17,
						},
					},
				},
			},
		},
		{
			expr: "(foo or vector(5)) * (vector(2) or bar)",
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector("foo", 1),
					},
				},
				{
					Type:            utils.FuncSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "vector",
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{5}, // FIXME should be 10 really but it's one-to-one binops
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `vector()` will return a vector value with no labels.",
							Fragment: "vector(5)",
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "vector",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeScalar,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							&promParser.NumberLiteral{
								Val: 5,
								PosRange: posrange.PositionRange{
									Start: 15,
									End:   16,
								},
							},
						},
						PosRange: posrange.PositionRange{
							Start: 8,
							End:   17,
						},
					},
				},
			},
		},
		{
			expr: `1 > bool 0`,
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{1},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "1",
						},
					},
				},
			},
		},
		{
			expr: `20 > bool 10`,
			output: []utils.Source{
				{
					Type:            utils.NumberSource,
					Returns:         promParser.ValueTypeScalar,
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{20},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a number value with no labels.",
							Fragment: "20",
						},
					},
				},
			},
		},
		{
			expr: `"test"`,
			output: []utils.Source{
				{
					Type:          utils.StringSource,
					Returns:       promParser.ValueTypeString,
					FixedLabels:   true,
					AlwaysReturns: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "This returns a string value with no labels.",
							Fragment: `"test"`,
						},
					},
				},
			},
		},
		{
			expr: "foo",
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector("foo", 0),
					},
				},
			},
		},
		{
			expr: `foo{job="bar"}`,
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="bar"}`, 0),
					},
					GuaranteedLabels: []string{"job"},
				},
			},
		},
		{
			expr: `foo{job="bar"} or bar{job="foo"}`,
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="bar"}`, 0),
					},
					Operation:        promParser.CardManyToMany.String(),
					GuaranteedLabels: []string{"job"},
				},
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`bar{job="foo"}`, 18),
					},
					Operation:        promParser.CardManyToMany.String(),
					GuaranteedLabels: []string{"job"},
				},
			},
		},
		{
			expr: `foo{a="bar"} or bar{b="foo"}`,
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{a="bar"}`, 0),
					},
					Operation:        promParser.CardManyToMany.String(),
					GuaranteedLabels: []string{"a"},
				},
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`bar{b="foo"}`, 16),
					},
					Operation:        promParser.CardManyToMany.String(),
					GuaranteedLabels: []string{"b"},
				},
			},
		},
		{
			expr: "foo[5m]",
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector("foo", 0),
					},
				},
			},
		},
		{
			expr: "prometheus_build_info[2m:1m]",
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector("prometheus_build_info", 0),
					},
				},
			},
		},
		{
			expr: "deriv(rate(distance_covered_meters_total[1m])[5m:1m])",
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "deriv",
					Selectors: []*promParser.VectorSelector{
						mustParseVector("distance_covered_meters_total", 11),
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "deriv",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeMatrix,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							&promParser.SubqueryExpr{
								Expr: &promParser.Call{
									Func: &promParser.Function{
										Name: "rate",
										ArgTypes: []promParser.ValueType{
											promParser.ValueTypeMatrix,
										},
										Variadic:   0,
										ReturnType: promParser.ValueTypeVector,
									},
									Args: promParser.Expressions{
										mustParseMatrix(`distance_covered_meters_total[1m]`, 11),
									},
									PosRange: posrange.PositionRange{
										Start: 6,
										End:   45,
									},
								},
								Range:  time.Minute * 5,
								Step:   time.Minute,
								EndPos: posrange.Pos(52),
							},
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   53,
						},
					},
				},
			},
		},
		{
			expr: "foo - 1",
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector("foo", 0),
					},
				},
			},
		},
		{
			expr: "foo / 5",
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector("foo", 0),
					},
				},
			},
		},
		{
			expr: "-foo",
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector("foo", 1),
					},
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"})`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 4),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo{job="myjob"})`,
						},
					},
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"}) without(job)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 4),
					},
					ExcludedLabels: []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason:   "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `sum(foo{job="myjob"}) without(job)`,
						},
					},
				},
			},
		},
		{
			expr: `sum(foo) by(job)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 4),
					},
					IncludedLabels: []string{"job"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: `sum(foo) by(job)`,
						},
					},
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"}) by(job)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 4),
					},
					IncludedLabels:   []string{"job"},
					GuaranteedLabels: []string{"job"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: `sum(foo{job="myjob"}) by(job)`,
						},
					},
				},
			},
		},
		{
			expr: `abs(foo{job="myjob"} or bar{cluster="dev"})`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "abs",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 4),
						mustParseVector(`bar{cluster="dev"}`, 24),
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "abs",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							&promParser.BinaryExpr{
								Op:  promParser.LOR,
								LHS: mustParseVector(`foo{job="myjob"}`, 4),
								RHS: mustParseVector(`bar{cluster="dev"}`, 24),
								VectorMatching: &promParser.VectorMatching{
									Card: promParser.CardManyToMany,
								},
							},
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   43,
						},
					},
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"} or bar{cluster="dev"}) without(instance)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 4),
					},
					GuaranteedLabels: []string{"job"},
					ExcludedLabels:   []string{"instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"instance": {
							Reason:   "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `sum(foo{job="myjob"} or bar{cluster="dev"}) without(instance)`,
						},
					},
				},
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`bar{cluster="dev"}`, 24),
					},
					GuaranteedLabels: []string{"cluster"},
					ExcludedLabels:   []string{"instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"instance": {
							Reason:   "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `sum(foo{job="myjob"} or bar{cluster="dev"}) without(instance)`,
						},
					},
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"}) without(instance)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 4),
					},
					GuaranteedLabels: []string{"job"},
					ExcludedLabels:   []string{"instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"instance": {
							Reason:   "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `sum(foo{job="myjob"}) without(instance)`,
						},
					},
				},
			},
		},
		{
			expr: `min(foo{job="myjob"}) / max(foo{job="myjob"})`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "min",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 4),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `min(foo{job="myjob"})`,
						},
					},
				},
			},
		},
		{
			expr: `max(foo{job="myjob"}) / min(foo{job="myjob"})`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "max",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 4),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `max(foo{job="myjob"})`,
						},
					},
				},
			},
		},
		{
			expr: `avg(foo{job="myjob"}) by(job)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "avg",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 4),
					},
					GuaranteedLabels: []string{"job"},
					IncludedLabels:   []string{"job"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: `avg(foo{job="myjob"}) by(job)`,
						},
					},
				},
			},
		},
		{
			expr: `group(foo) by(job)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "group",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 6),
					},
					IncludedLabels: []string{"job"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: `group(foo) by(job)`,
						},
					},
				},
			},
		},
		{
			expr: `stddev(rate(foo[5m]))`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "stddev",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 12),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `stddev(rate(foo[5m]))`,
						},
					},
				},
			},
		},
		{
			expr: `stdvar(rate(foo[5m]))`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "stdvar",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 12),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `stdvar(rate(foo[5m]))`,
						},
					},
				},
			},
		},
		{
			expr: `stddev_over_time(foo[5m])`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "stddev_over_time",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 17),
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "stddev_over_time",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeMatrix,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseMatrix(`foo[5m]`, 17),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   25,
						},
					},
				},
			},
		},
		{
			expr: `stdvar_over_time(foo[5m])`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "stdvar_over_time",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 17),
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "stdvar_over_time",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeMatrix,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseMatrix(`foo[5m]`, 17),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   25,
						},
					},
				},
			},
		},
		{
			expr: `quantile(0.9, rate(foo[5m]))`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "quantile",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 19),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `quantile(0.9, rate(foo[5m]))`,
						},
					},
				},
			},
		},
		{
			expr: `count_values("version", build_version)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "count_values",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`build_version`, 24),
					},
					GuaranteedLabels: []string{"version"},
					IncludedLabels:   []string{"version"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `count_values("version", build_version)`,
						},
					},
				},
			},
		},
		{
			expr: `count_values("version", build_version) without(job)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "count_values",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`build_version`, 24),
					},
					IncludedLabels:   []string{"version"},
					GuaranteedLabels: []string{"version"},
					ExcludedLabels:   []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason:   "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `count_values("version", build_version) without(job)`,
						},
					},
				},
			},
		},
		{
			expr: `count_values("version", build_version{job="foo"}) without(job)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "count_values",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`build_version{job="foo"}`, 24),
					},
					IncludedLabels:   []string{"version"},
					GuaranteedLabels: []string{"version"},
					ExcludedLabels:   []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason:   "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `count_values("version", build_version{job="foo"}) without(job)`,
						},
					},
				},
			},
		},
		{
			expr: `count_values("version", build_version) by(job)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "count_values",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`build_version`, 24),
					},
					GuaranteedLabels: []string{"version"},
					IncludedLabels:   []string{"job", "version"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: `count_values("version", build_version) by(job)`,
						},
					},
				},
			},
		},
		{
			expr: `topk(10, foo{job="myjob"}) > 10`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "topk",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 9),
					},
					GuaranteedLabels: []string{"job"},
				},
			},
		},
		{
			expr: `topk(10, foo or bar)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "topk",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 9),
					},
				},
				{
					Type:      utils.AggregateSource,
					Operation: "topk",
					Returns:   promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`bar`, 16),
					},
				},
			},
		},
		{
			expr: `rate(foo[10m])`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "rate",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 5),
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "rate",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeMatrix,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseMatrix(`foo[10m]`, 5),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   14,
						},
					},
				},
			},
		},
		{
			expr: `sum(rate(foo[10m])) without(instance)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 9),
					},
					ExcludedLabels: []string{"instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"instance": {
							Reason:   "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `sum(rate(foo[10m])) without(instance)`,
						},
					},
				},
			},
		},
		{
			expr: `foo{job="foo"} / bar`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardOneToOne.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="foo"}`, 0),
					},
					GuaranteedLabels: []string{"job"},
				},
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) bar`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardOneToOne.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="foo"}`, 0),
					},
					GuaranteedLabels: []string{"job"},
					IncludedLabels:   []string{"instance"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using one-to-one vector matching with `on(instance)`, only labels included inside `on(...)` will be present on the results.",
							Fragment: `foo{job="foo"} * on(instance) bar`,
						},
					},
				},
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) group_left(bar) bar`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToOne.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="foo"}`, 0),
					},
					IncludedLabels:   []string{"bar", "instance"},
					GuaranteedLabels: []string{"job"},
				},
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) group_left(cluster) bar{cluster="bar", ignored="true"}`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToOne.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="foo"}`, 0),
					},
					IncludedLabels:   []string{"cluster", "instance"},
					GuaranteedLabels: []string{"job"},
				},
			},
		},
		{
			expr: `foo{job="foo", ignored="true"} * on(instance) group_right(job) bar{cluster="bar"}`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardOneToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`bar{cluster="bar"}`, 63),
					},
					IncludedLabels:   []string{"job", "instance"},
					GuaranteedLabels: []string{"cluster"},
				},
			},
		},
		{
			expr: `count(foo / bar)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "count",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 6),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `count(foo / bar)`,
						},
					},
				},
			},
		},
		{
			expr: `count(up{job="a"} / on () up{job="b"})`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "count",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`up{job="a"}`, 6),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `count(up{job="a"} / on () up{job="b"})`,
						},
					},
				},
			},
		},
		{
			expr: `count(up{job="a"} / on (env) up{job="b"})`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "count",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`up{job="a"}`, 6),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `count(up{job="a"} / on (env) up{job="b"})`,
						},
					},
				},
			},
		},
		{
			expr: `foo{job="foo", instance="1"} and bar`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="foo", instance="1"}`, 0),
					},
					GuaranteedLabels: []string{"job", "instance"},
				},
			},
		},
		{
			expr: `foo{job="foo", instance="1"} and on(cluster) bar`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="foo", instance="1"}`, 0),
					},
					IncludedLabels:   []string{"cluster"},
					GuaranteedLabels: []string{"job", "instance"},
				},
			},
		},
		{
			expr: `topk(10, foo)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "topk",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 9),
					},
				},
			},
		},
		{
			expr: `topk(10, foo) without(cluster)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "topk",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 9),
					},
				},
			},
		},
		{
			expr: `topk(10, foo) by(cluster)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "topk",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 9),
					},
				},
			},
		},
		{
			expr: `bottomk(10, sum(rate(foo[5m])) without(job))`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "bottomk",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 21),
					},
					ExcludedLabels: []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason:   "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `sum(rate(foo[5m])) without(job)`,
						},
					},
				},
			},
		},
		{
			expr: `foo or bar`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 0),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`bar`, 7),
					},
				},
			},
		},
		{
			expr: `foo or bar or baz`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 0),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`bar`, 7),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`baz`, 14),
					},
				},
			},
		},
		{
			expr: `(foo or bar) or baz`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 1),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`bar`, 8),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`baz`, 16),
					},
				},
			},
		},
		{
			expr: `foo unless bar`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 0),
					},
				},
			},
		},
		{
			expr: `count(sum(up{job="foo", cluster="dev"}) by(job, cluster) == 0) without(job, cluster)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "count",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`up{job="foo", cluster="dev"}`, 10),
					},
					ExcludedLabels: []string{"job", "cluster"}, // FIXME empty
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(job, cluster)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: `sum(up{job="foo", cluster="dev"}) by(job, cluster)`,
						},
						"job": {
							Reason:   "Query is using aggregation with `without(job, cluster)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `count(sum(up{job="foo", cluster="dev"}) by(job, cluster) == 0) without(job, cluster)`,
						},
						"cluster": {
							Reason:   "Query is using aggregation with `without(job, cluster)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `count(sum(up{job="foo", cluster="dev"}) by(job, cluster) == 0) without(job, cluster)`,
						},
					},
				},
			},
		},
		{
			expr: "year()",
			output: []utils.Source{
				{
					Type:          utils.FuncSource,
					Returns:       promParser.ValueTypeVector,
					Operation:     "year",
					FixedLabels:   true,
					AlwaysReturns: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `year()` with no arguments will return an empty time series with no labels.",
							Fragment: `year()`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "year",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   1,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   6,
						},
					},
				},
			},
		},
		{
			expr: "year(foo)",
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "year",
					Selectors: []*promParser.VectorSelector{
						mustParseVector("foo", 5),
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "year",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   1,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector("foo", 5),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   9,
						},
					},
				},
			},
		},
		{
			expr: `label_join(up{job="api-server",src1="a",src2="b",src3="c"}, "foo", ",", "src1", "src2", "src3")`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "label_join",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`up{job="api-server",src1="a",src2="b",src3="c"}`, 11),
					},
					GuaranteedLabels: []string{"job", "src1", "src2", "src3", "foo"},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "label_join",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
								promParser.ValueTypeString,
								promParser.ValueTypeString,
								promParser.ValueTypeString,
							},
							Variadic:   -1,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`up{job="api-server",src1="a",src2="b",src3="c"}`, 11),
							&promParser.StringLiteral{
								Val:      "foo",
								PosRange: posrange.PositionRange{Start: 60, End: 65},
							},
							&promParser.StringLiteral{
								Val:      ",",
								PosRange: posrange.PositionRange{Start: 67, End: 70},
							},
							&promParser.StringLiteral{
								Val:      "src1",
								PosRange: posrange.PositionRange{Start: 72, End: 78},
							},
							&promParser.StringLiteral{
								Val:      "src2",
								PosRange: posrange.PositionRange{Start: 80, End: 86},
							},
							&promParser.StringLiteral{
								Val:      "src3",
								PosRange: posrange.PositionRange{Start: 88, End: 94},
							},
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   95,
						},
					},
				},
			},
		},
		{
			expr: `
(
	sum(foo:sum > 0) without(notify)
	* on(job) group_left(notify)
	job:notify
)
and on(job)
sum(foo:count) by(job) > 20`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo:sum`, 8),
					},
					IncludedLabels: []string{"notify", "job"},
					ExcludeReason:  map[string]utils.ExcludedLabel{},
				},
			},
		},
		{
			expr: `container_file_descriptors / on (instance, app_name) container_ulimits_soft{ulimit="max_open_files"}`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardOneToOne.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`container_file_descriptors`, 0),
					},
					IncludedLabels: []string{"instance", "app_name"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using one-to-one vector matching with `on(instance, app_name)`, only labels included inside `on(...)` will be present on the results.",
							Fragment: `container_file_descriptors / on (instance, app_name) container_ulimits_soft{ulimit="max_open_files"}`,
						},
					},
				},
			},
		},
		{
			expr: `container_file_descriptors / on (instance, app_name) group_left() container_ulimits_soft{ulimit="max_open_files"}`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToOne.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`container_file_descriptors`, 0),
					},
					IncludedLabels: []string{"instance", "app_name"},
				},
			},
		},
		{
			expr: `absent(foo{job="bar"})`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="bar"}`, 7),
					},
					IncludedLabels:   []string{"job"},
					GuaranteedLabels: []string{"job"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(foo{job="bar"})`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "absent",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`foo{job="bar"}`, 7),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   22,
						},
					},
				},
			},
		},
		{
			expr: `absent(foo{job="bar", cluster!="dev", instance=~".+", env="prod"})`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="bar", cluster!="dev", instance=~".+", env="prod"}`, 7),
					},
					IncludedLabels:   []string{"job", "env"},
					GuaranteedLabels: []string{"job", "env"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(foo{job="bar", cluster!="dev", instance=~".+", env="prod"})`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "absent",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`foo{job="bar", cluster!="dev", instance=~".+", env="prod"}`, 7),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   66,
						},
					},
				},
			},
		},
		{
			expr: `absent(sum(foo) by(job, instance))`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 11),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(sum(foo) by(job, instance))`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "absent",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							&promParser.AggregateExpr{
								Op:       promParser.SUM,
								Expr:     mustParseVector("foo", 11),
								Grouping: []string{"job", "instance"},
								PosRange: posrange.PositionRange{
									Start: 7,
									End:   33,
								},
							},
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   34,
						},
					},
				},
			},
		},
		{
			expr: `absent(foo{job="prometheus", xxx="1"}) AND on(job) prometheus_build_info`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="prometheus", xxx="1"}`, 7),
					},
					IncludedLabels:   []string{"job", "xxx"},
					GuaranteedLabels: []string{"job", "xxx"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(foo{job="prometheus", xxx="1"})`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "absent",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`foo{job="prometheus", xxx="1"}`, 7),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   38,
						},
					},
				},
			},
		},
		{
			expr: `1 + sum(foo) by(notjob)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 8),
					},
					IncludedLabels: []string{"notjob"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(notjob)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: `sum(foo) by(notjob)`,
						},
					},
				},
			},
		},
		{
			expr: `count(node_exporter_build_info) by (instance, version) != ignoring(package,version) group_left(foo) count(deb_package_version) by (instance, version, package)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "count",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`node_exporter_build_info`, 6),
					},
					IncludedLabels: []string{"instance", "version", "foo"}, // FIXME foo shouldn't be there because count() doesn't produce it
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(instance, version)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: `count(node_exporter_build_info) by (instance, version)`,
						},
					},
				},
			},
		},
		{
			expr: `absent(foo) or absent(bar)`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 7),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(foo)`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "absent",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`foo`, 7),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   11,
						},
					},
				},
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`bar`, 22),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(bar)`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "absent",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`bar`, 22),
						},
						PosRange: posrange.PositionRange{
							Start: 15,
							End:   26,
						},
					},
				},
			},
		},
		{
			expr: `absent_over_time(foo[5m]) or absent(bar)`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent_over_time",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 17),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent_over_time()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent_over_time) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent_over_time(foo[5m])`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "absent_over_time",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeMatrix,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseMatrix(`foo[5m]`, 17),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   25,
						},
					},
				},
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`bar`, 36),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(bar)`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "absent",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`bar`, 36),
						},
						PosRange: posrange.PositionRange{
							Start: 29,
							End:   40,
						},
					},
				},
			},
		},
		{
			expr: `bar * on() group_right(cluster, env) absent(foo{job="xxx"})`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="xxx"}`, 44),
					},
					IncludedLabels:   []string{"job", "cluster", "env"},
					GuaranteedLabels: []string{"job"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(foo{job="xxx"})`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "absent",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`foo{job="xxx"}`, 44),
						},
						PosRange: posrange.PositionRange{
							Start: 37,
							End:   59,
						},
					},
				},
			},
		},
		{
			expr: `bar * on() group_right() absent(foo{job="xxx"})`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="xxx"}`, 32),
					},
					IncludedLabels:   []string{"job"},
					GuaranteedLabels: []string{"job"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(foo{job="xxx"})`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "absent",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`foo{job="xxx"}`, 32),
						},
						PosRange: posrange.PositionRange{
							Start: 25,
							End:   47,
						},
					},
				},
			},
		},
		{
			expr: "vector(1)",
			output: []utils.Source{
				{
					Type:            utils.FuncSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "vector",
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{1},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `vector()` will return a vector value with no labels.",
							Fragment: `vector(1)`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "vector",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeScalar,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							&promParser.NumberLiteral{
								Val: 1,
								PosRange: posrange.PositionRange{
									Start: 7,
									End:   8,
								},
							},
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   9,
						},
					},
				},
			},
		},
		{
			expr: `sum_over_time(foo{job="myjob"}[5m])`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum_over_time",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="myjob"}`, 14),
					},
					GuaranteedLabels: []string{"job"},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "sum_over_time",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeMatrix,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseMatrix(`foo{job="myjob"}[5m]`, 14),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   35,
						},
					},
				},
			},
		},
		{
			expr: `days_in_month()`,
			output: []utils.Source{
				{
					Type:          utils.FuncSource,
					Returns:       promParser.ValueTypeVector,
					Operation:     "days_in_month",
					FixedLabels:   true,
					AlwaysReturns: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `days_in_month()` with no arguments will return an empty time series with no labels.",
							Fragment: `days_in_month()`,
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "days_in_month",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   1,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   15,
						},
					},
				},
			},
		},
		{
			expr: `days_in_month(foo{job="foo"})`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "days_in_month",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo{job="foo"}`, 14),
					},
					GuaranteedLabels: []string{"job"},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "days_in_month",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
							},
							Variadic:   1,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`foo{job="foo"}`, 14),
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   29,
						},
					},
				},
			},
		},
		{
			expr: `label_replace(up{job="api-server",service="a:c"}, "foo", "$1", "service", "(.*):.*")`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "label_replace",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`up{job="api-server",service="a:c"}`, 14),
					},
					GuaranteedLabels: []string{"job", "service", "foo"},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "label_replace",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
								promParser.ValueTypeString,
								promParser.ValueTypeString,
								promParser.ValueTypeString,
								promParser.ValueTypeString,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							mustParseVector(`up{job="api-server",service="a:c"}`, 14),
							&promParser.StringLiteral{
								Val: "foo",
								PosRange: posrange.PositionRange{
									Start: 50,
									End:   55,
								},
							},
							&promParser.StringLiteral{
								Val: "$1",
								PosRange: posrange.PositionRange{
									Start: 57,
									End:   61,
								},
							},
							&promParser.StringLiteral{
								Val: "service",
								PosRange: posrange.PositionRange{
									Start: 63,
									End:   72,
								},
							},
							&promParser.StringLiteral{
								Val: "(.*):.*",
								PosRange: posrange.PositionRange{
									Start: 74,
									End:   83,
								},
							},
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   84,
						},
					},
				},
			},
		},
		{
			expr: `(time() - my_metric) > 5*3600`,
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector("my_metric", 10),
					},
				},
			},
		},
		{
			expr: `up{instance="a", job="prometheus"} * ignoring(job) up{instance="a", job="pint"}`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardOneToOne.String(),
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`up{instance="a", job="prometheus"}`, 0),
					},
					GuaranteedLabels: []string{"instance"},
					ExcludedLabels:   []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason:   "Query is using one-to-one vector matching with `ignoring(job)`, all labels included inside `ignoring(...)` will be removed on the results.",
							Fragment: `up{instance="a", job="prometheus"} * ignoring(job) up{instance="a", job="pint"}`,
						},
					},
				},
			},
		},
		{
			expr: `
avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"})
< 0.5 > 0
or sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"})
< on() count(colo_router_tier:disabled_pops:max{tier="1",router=~"edge.*"}) * 0.4 > 0
or avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"})
< 0.1 > 0
`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "avg",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"}`, 41),
					},
					ExcludedLabels: []string{"router", "colo_id", "instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"router": {
							Reason:   "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"})`,
						},
						"colo_id": {
							Reason:   "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"})`,
						},
						"instance": {
							Reason:   "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"})`,
						},
					},
				},
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"}`, 155),
					},
					GuaranteedLabels: []string{"cidr_use_case"},
					ExcludedLabels:   []string{"router", "colo_id", "instance"},
					FixedLabels:      true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"router": {
							Reason:   "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"})`,
						},
						"colo_id": {
							Reason:   "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"})`,
						},
						"instance": {
							Reason:   "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"})`,
						},
						"": {
							Reason:   "Query is using one-to-one vector matching with `on()`, only labels included inside `on(...)` will be present on the results.",
							Fragment: "sum without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~\".*tier1.*\"})\n< on() count(colo_router_tier:disabled_pops:max{tier=\"1\",router=~\"edge.*\"}) * 0.4",
						},
					},
				},
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "avg",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"}`, 343),
					},
					GuaranteedLabels: []string{"cidr_use_case"},
					ExcludedLabels:   []string{"router", "colo_id", "instance"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"router": {
							Reason:   "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"})`,
						},
						"colo_id": {
							Reason:   "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"})`,
						},
						"instance": {
							Reason:   "Query is using aggregation with `without(router, colo_id, instance)`, all labels included inside `without(...)` will be removed from the results.",
							Fragment: `avg without(router, colo_id, instance) (router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"})`,
						},
					},
				},
			},
		},
		{
			expr: `label_replace(sum(foo) without(instance), "instance", "none", "", "")`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "label_replace",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 18),
					},
					GuaranteedLabels: []string{"instance"},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "label_replace",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeVector,
								promParser.ValueTypeString,
								promParser.ValueTypeString,
								promParser.ValueTypeString,
								promParser.ValueTypeString,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							&promParser.AggregateExpr{
								Op:       promParser.SUM,
								Expr:     mustParseVector("foo", 18),
								Grouping: []string{"instance"},
								Without:  true,
								PosRange: posrange.PositionRange{
									Start: 14,
									End:   40,
								},
							},
							&promParser.StringLiteral{
								Val: "instance",
								PosRange: posrange.PositionRange{
									Start: 42,
									End:   52,
								},
							},
							&promParser.StringLiteral{
								Val: "none",
								PosRange: posrange.PositionRange{
									Start: 54,
									End:   60,
								},
							},
							&promParser.StringLiteral{
								Val: "",
								PosRange: posrange.PositionRange{
									Start: 62,
									End:   64,
								},
							},
							&promParser.StringLiteral{
								Val: "",
								PosRange: posrange.PositionRange{
									Start: 66,
									End:   68,
								},
							},
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   69,
						},
					},
				},
			},
		},
		{
			expr: `
sum by (region, target, colo_name) (
    sum_over_time(probe_success{job="abc"}[5m])
	or
	vector(1)
) == 0`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`probe_success{job="abc"}`, 56),
					},
					IncludedLabels: []string{"region", "target", "colo_name"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(region, target, colo_name)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: "sum by (region, target, colo_name) (\n    sum_over_time(probe_success{job=\"abc\"}[5m])\n\tor\n\tvector(1)\n)",
						},
					},
				},
				{
					Type:            utils.AggregateSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "sum",
					FixedLabels:     true,
					AlwaysReturns:   true,
					IsDead:          true,
					ReturnedNumbers: []float64{1},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `vector()` will return a vector value with no labels.",
							Fragment: "vector(1)",
						},
					},
				},
			},
		},
		{
			expr: `vector(1) or foo`,
			output: []utils.Source{
				{
					Type:            utils.FuncSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "vector",
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{1},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `vector()` will return a vector value with no labels.",
							Fragment: "vector(1)",
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "vector",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeScalar,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							&promParser.NumberLiteral{
								Val: 1,
								PosRange: posrange.PositionRange{
									Start: 7,
									End:   8,
								},
							},
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   9,
						},
					},
				},
				{
					Type:      utils.SelectorSource,
					Operation: promParser.CardManyToMany.String(),
					Returns:   promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParseVector("foo", 13),
					},
					IsDead: true,
				},
			},
		},
		{
			expr: `vector(0) > 0`,
			output: []utils.Source{
				{
					Type:            utils.FuncSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "vector",
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{0},
					IsDead:          true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `vector()` will return a vector value with no labels.",
							Fragment: "vector(0)",
						},
					},
					Call: &promParser.Call{
						Func: &promParser.Function{
							Name: "vector",
							ArgTypes: []promParser.ValueType{
								promParser.ValueTypeScalar,
							},
							Variadic:   0,
							ReturnType: promParser.ValueTypeVector,
						},
						Args: promParser.Expressions{
							&promParser.NumberLiteral{
								Val: 0,
								PosRange: posrange.PositionRange{
									Start: 7,
									End:   8,
								},
							},
						},
						PosRange: posrange.PositionRange{
							Start: 0,
							End:   9,
						},
					},
				},
			},
		},
		{
			expr: `sum(foo or vector(0)) > 0`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 4),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo or vector(0))`,
						},
					},
				},
				{
					Type:            utils.AggregateSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "sum",
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{0},
					IsDead:          true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: "sum(foo or vector(0))",
						},
					},
				},
			},
		},
		{
			expr: `(sum(foo or vector(1)) > 0) == 2`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 5),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo or vector(1))`,
						},
					},
				},
				{
					Type:            utils.AggregateSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "sum",
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{1},
					IsDead:          true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: "sum(foo or vector(1))",
						},
					},
				},
			},
		},
		{
			expr: `(sum(foo or vector(1)) > 0) != 2`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 5),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo or vector(1))`,
						},
					},
				},
				{
					Type:            utils.AggregateSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "sum",
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{1},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: "sum(foo or vector(1))",
						},
					},
				},
			},
		},
		{
			expr: `(sum(foo or vector(2)) > 0) != 2`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParseVector(`foo`, 5),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo or vector(2))`,
						},
					},
				},
				{
					Type:            utils.AggregateSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "sum",
					FixedLabels:     true,
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{2},
					IsDead:          true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: "sum(foo or vector(2))",
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expr, func(t *testing.T) {
			n, err := parser.DecodeExpr(tc.expr)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			output := utils.LabelsSource(tc.expr, n.Expr)
			require.Len(t, output, len(tc.output))
			for i := range len(tc.output) {
				require.EqualExportedValues(t, tc.output[i], output[i], "Mismatch at index %d", i)
			}
		})
	}
}

func TestLabelsSourceCallCoverage(t *testing.T) {
	for name, def := range promParser.Functions {
		t.Run(name, func(t *testing.T) {
			if def.Experimental {
				t.SkipNow()
			}

			var b strings.Builder
			b.WriteString(name)
			b.WriteRune('(')
			for i, at := range def.ArgTypes {
				if i > 0 {
					b.WriteString(", ")
				}
				switch at {
				case promParser.ValueTypeNone:
				case promParser.ValueTypeScalar:
					b.WriteRune('1')
				case promParser.ValueTypeVector:
					b.WriteString("http_requests_total")
				case promParser.ValueTypeMatrix:
					b.WriteString("http_requests_total[2m]")
				case promParser.ValueTypeString:
					b.WriteString(`"foo"`)
				}
			}
			b.WriteRune(')')

			n, err := parser.DecodeExpr(b.String())
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			output := utils.LabelsSource(b.String(), n.Expr)
			require.Len(t, output, 1)
			require.NotNil(t, output[0].Call, "no call detected in: %q ~> %+v", b.String(), output)
			require.Equal(t, name, output[0].Operation)
			require.Equal(t, def.ReturnType, output[0].Returns, "incorrect return type on Source{}")
		})
	}
}

func TestLabelsSourceCallCoverageFail(t *testing.T) {
	n := &parser.PromQLNode{
		Expr: &promParser.Call{
			Func: &promParser.Function{
				Name: "fake_call",
			},
		},
	}
	output := utils.LabelsSource("fake_call()", n.Expr)
	require.Len(t, output, 1)
	require.Nil(t, output[0].Call, "no call should have been detected in fake function")
}
