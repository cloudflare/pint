package utils_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/utils"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

func mustParse[T any](t *testing.T, s string, offset int) T {
	m, err := promParser.ParseExpr(strings.Repeat(" ", offset) + s)
	require.NoErrorf(t, err, "failed to parse vector selector: %s", s)
	n, ok := m.(T)
	require.True(t, ok, "failed to convert %q to %t\n", s, n)
	return n
}

func TestLabelsSource(t *testing.T) {
	type testCaseT struct {
		expr   string
		output []utils.Source
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
					IsConditional: true,
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
					IsConditional: true,
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
					IsConditional: true,
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
					IsConditional: true,
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
					IsConditional: true,
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
					IsConditional: true,
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
						mustParse[*promParser.VectorSelector](t, "foo", 1),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, "bar", 8),
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
						mustParse[*promParser.VectorSelector](t, "foo", 1),
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
					Call: mustParse[*promParser.Call](t, "vector(2)", 8),
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
						mustParse[*promParser.VectorSelector](t, "foo", 1),
					},
					Joins: []utils.Source{
						{
							Type:            utils.FuncSource,
							Returns:         promParser.ValueTypeVector,
							Operation:       "vector",
							FixedLabels:     true,
							AlwaysReturns:   true,
							ReturnedNumbers: []float64{2},
							Call:            mustParse[*promParser.Call](t, "vector(2)", 22),
							ExcludeReason: map[string]utils.ExcludedLabel{
								"": {
									Reason:   "Calling `vector()` will return a vector value with no labels.",
									Fragment: "vector(2)",
								},
							},
						},
						{
							Type:      utils.SelectorSource,
							Returns:   promParser.ValueTypeVector,
							Operation: promParser.CardManyToMany.String(),
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, "bar", 35),
							},
							IsDead: true,
						},
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
					Call: mustParse[*promParser.Call](t, "vector(5)", 8),
					Joins: []utils.Source{
						{
							Type:            utils.FuncSource,
							Returns:         promParser.ValueTypeVector,
							Operation:       "vector",
							FixedLabels:     true,
							AlwaysReturns:   true,
							ReturnedNumbers: []float64{2},
							Call:            mustParse[*promParser.Call](t, "vector(2)", 22),
							ExcludeReason: map[string]utils.ExcludedLabel{
								"": {
									Reason:   "Calling `vector()` will return a vector value with no labels.",
									Fragment: "vector(2)",
								},
							},
						},
						{
							Type:      utils.SelectorSource,
							Returns:   promParser.ValueTypeVector,
							Operation: promParser.CardManyToMany.String(),
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, "bar", 35),
							},
							IsDead: true,
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
					IsConditional: true,
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
					IsConditional: true,
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
						mustParse[*promParser.VectorSelector](t, "foo", 0),
					},
				},
			},
		},
		{
			expr: "foo offset 5m",
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, "foo offset 5m", 0),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="bar"}`, 0),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="bar"}`, 0),
					},
					Operation:        promParser.CardManyToMany.String(),
					GuaranteedLabels: []string{"job"},
				},
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `bar{job="foo"}`, 18),
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
						mustParse[*promParser.VectorSelector](t, `foo{a="bar"}`, 0),
					},
					Operation:        promParser.CardManyToMany.String(),
					GuaranteedLabels: []string{"a"},
				},
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `bar{b="foo"}`, 16),
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
						mustParse[*promParser.VectorSelector](t, "foo", 0),
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
						mustParse[*promParser.VectorSelector](t, "prometheus_build_info", 0),
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
						mustParse[*promParser.VectorSelector](t, "distance_covered_meters_total", 11),
					},
					Call: mustParse[*promParser.Call](t, "deriv(rate(distance_covered_meters_total[1m])[5m:1m])", 0),
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
						mustParse[*promParser.VectorSelector](t, "foo", 0),
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
						mustParse[*promParser.VectorSelector](t, "foo", 0),
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
						mustParse[*promParser.VectorSelector](t, "foo", 1),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
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
			expr: `sum(foo{job="myjob"}) > 20`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo{job="myjob"})`,
						},
					},
					IsConditional: true,
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 4),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
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
			expr: `abs(foo{job="myjob"} offset 5m)`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "abs",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"} offset 5m`, 4),
					},
					Call:             mustParse[*promParser.Call](t, `abs(foo{job="myjob"} offset 5m)`, 0),
					GuaranteedLabels: []string{"job"},
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					},
					GuaranteedLabels: []string{"job"},
					Call:             mustParse[*promParser.Call](t, `abs(foo{job="myjob"} or bar{cluster="dev"})`, 0),
				},
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "abs",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `bar{cluster="dev"}`, 24),
					},
					Call:             mustParse[*promParser.Call](t, `abs(foo{job="myjob"} or bar{cluster="dev"})`, 0),
					GuaranteedLabels: []string{"cluster"},
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
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
						mustParse[*promParser.VectorSelector](t, `bar{cluster="dev"}`, 24),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `min(foo{job="myjob"})`,
						},
					},
					Joins: []utils.Source{
						{
							Type:      utils.AggregateSource,
							Operation: "max",
							Returns:   promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 28),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `max(foo{job="myjob"})`,
						},
					},
					Joins: []utils.Source{
						{
							Type:      utils.AggregateSource,
							Operation: "min",
							Returns:   promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 28),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 4),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 6),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 12),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 12),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 17),
					},
					Call: mustParse[*promParser.Call](t, `stddev_over_time(foo[5m])`, 0),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 17),
					},
					Call: mustParse[*promParser.Call](t, `stdvar_over_time(foo[5m])`, 0),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 19),
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
						mustParse[*promParser.VectorSelector](t, `build_version`, 24),
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
						mustParse[*promParser.VectorSelector](t, `build_version`, 24),
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
						mustParse[*promParser.VectorSelector](t, `build_version{job="foo"}`, 24),
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
						mustParse[*promParser.VectorSelector](t, `build_version`, 24),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 9),
					},
					GuaranteedLabels: []string{"job"},
					IsConditional:    true,
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
						mustParse[*promParser.VectorSelector](t, `foo`, 9),
					},
				},
				{
					Type:      utils.AggregateSource,
					Operation: "topk",
					Returns:   promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `bar`, 16),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 5),
					},
					Call: mustParse[*promParser.Call](t, `rate(foo[10m])`, 0),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 9),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="foo"}`, 0),
					},
					GuaranteedLabels: []string{"job"},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 17),
							},
						},
					},
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
						mustParse[*promParser.VectorSelector](t, `foo{job="foo"}`, 0),
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
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 30),
							},
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
						mustParse[*promParser.VectorSelector](t, `foo{job="foo"}`, 0),
					},
					IncludedLabels:   []string{"bar", "instance"},
					GuaranteedLabels: []string{"job"},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 46),
							},
						},
					},
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
						mustParse[*promParser.VectorSelector](t, `foo{job="foo"}`, 0),
					},
					IncludedLabels:   []string{"cluster", "instance"},
					GuaranteedLabels: []string{"job"},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar{cluster="bar", ignored="true"}`, 50),
							},
							GuaranteedLabels: []string{"cluster", "ignored"},
						},
					},
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
						mustParse[*promParser.VectorSelector](t, `bar{cluster="bar"}`, 63),
					},
					IncludedLabels:   []string{"job", "instance"},
					GuaranteedLabels: []string{"cluster"},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `foo{job="foo", ignored="true"}`, 0),
							},
							GuaranteedLabels: []string{"job", "ignored"},
						},
					},
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
						mustParse[*promParser.VectorSelector](t, `foo`, 6),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `count(foo / bar)`,
						},
					},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 12),
							},
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
						mustParse[*promParser.VectorSelector](t, `up{job="a"}`, 6),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `count(up{job="a"} / on () up{job="b"})`,
						},
					},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `up{job="b"}`, 26),
							},
							GuaranteedLabels: []string{"job"},
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
						mustParse[*promParser.VectorSelector](t, `up{job="a"}`, 6),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `count(up{job="a"} / on (env) up{job="b"})`,
						},
					},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `up{job="b"}`, 29),
							},
							GuaranteedLabels: []string{"job"},
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
						mustParse[*promParser.VectorSelector](t, `foo{job="foo", instance="1"}`, 0),
					},
					GuaranteedLabels: []string{"job", "instance"},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 33),
							},
						},
					},
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
						mustParse[*promParser.VectorSelector](t, `foo{job="foo", instance="1"}`, 0),
					},
					IncludedLabels:   []string{"cluster"},
					GuaranteedLabels: []string{"job", "instance"},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 45),
							},
						},
					},
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
						mustParse[*promParser.VectorSelector](t, `foo`, 9),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 9),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 9),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 21),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 0),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `bar`, 7),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 0),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `bar`, 7),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `baz`, 14),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 1),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `bar`, 8),
					},
				},
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `baz`, 16),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 0),
					},
					Unless: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 11),
							},
						},
					},
				},
			},
		},
		{
			expr: `foo unless bar > 5`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `foo`, 0),
					},
					Unless: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 11),
							},
							IsConditional: true,
						},
					},
				},
			},
		},
		{
			expr: `foo unless bar unless baz`,
			output: []utils.Source{
				{
					Type:      utils.SelectorSource,
					Returns:   promParser.ValueTypeVector,
					Operation: promParser.CardManyToMany.String(),
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `foo`, 0),
					},
					Unless: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 11),
							},
						},
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `baz`, 22),
							},
						},
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
						mustParse[*promParser.VectorSelector](t, `up{job="foo", cluster="dev"}`, 10),
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
					IsConditional: true,
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
					Call: mustParse[*promParser.Call](t, `year()`, 0),
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
						mustParse[*promParser.VectorSelector](t, "foo", 5),
					},
					Call: mustParse[*promParser.Call](t, `year(foo)`, 0),
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
						mustParse[*promParser.VectorSelector](t, `up{job="api-server",src1="a",src2="b",src3="c"}`, 11),
					},
					GuaranteedLabels: []string{"job", "src1", "src2", "src3", "foo"},
					Call:             mustParse[*promParser.Call](t, `label_join(up{job="api-server",src1="a",src2="b",src3="c"}, "foo", ",", "src1", "src2", "src3")`, 0),
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
						mustParse[*promParser.VectorSelector](t, `foo:sum`, 8),
					},
					IncludedLabels: []string{"notify", "job"},
					ExcludeReason:  map[string]utils.ExcludedLabel{},
					IsConditional:  false, // FIXME should be true
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `job:notify`, 68),
							},
						},
						{
							Type:      utils.AggregateSource,
							Returns:   promParser.ValueTypeVector,
							Operation: "sum",
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `foo:count`, 97),
							},
							IncludedLabels: []string{"job"},
							FixedLabels:    true,
							ExcludeReason: map[string]utils.ExcludedLabel{
								"": {
									Reason:   "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
									Fragment: `sum(foo:count) by(job)`,
								},
							},
							IsConditional: true,
						},
					},
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
						mustParse[*promParser.VectorSelector](t, `container_file_descriptors`, 0),
					},
					IncludedLabels: []string{"instance", "app_name"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using one-to-one vector matching with `on(instance, app_name)`, only labels included inside `on(...)` will be present on the results.",
							Fragment: `container_file_descriptors / on (instance, app_name) container_ulimits_soft{ulimit="max_open_files"}`,
						},
					},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `container_ulimits_soft{ulimit="max_open_files"}`, 53),
							},
							GuaranteedLabels: []string{"ulimit"},
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
						mustParse[*promParser.VectorSelector](t, `container_file_descriptors`, 0),
					},
					IncludedLabels: []string{"instance", "app_name"},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `container_ulimits_soft{ulimit="max_open_files"}`, 66),
							},
							GuaranteedLabels: []string{"ulimit"},
						},
					},
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
						mustParse[*promParser.VectorSelector](t, `foo{job="bar"}`, 7),
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
					Call: mustParse[*promParser.Call](t, `absent(foo{job="bar"})`, 0),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="bar", cluster!="dev", instance=~".+", env="prod"}`, 7),
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
					Call: mustParse[*promParser.Call](t, `absent(foo{job="bar", cluster!="dev", instance=~".+", env="prod"})`, 0),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 11),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(sum(foo) by(job, instance))`,
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(sum(foo) by(job, instance))`, 0),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="prometheus", xxx="1"}`, 7),
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
					Call: mustParse[*promParser.Call](t, `absent(foo{job="prometheus", xxx="1"})`, 0),
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, "prometheus_build_info", 51),
							},
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
						mustParse[*promParser.VectorSelector](t, `foo`, 8),
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
						mustParse[*promParser.VectorSelector](t, `node_exporter_build_info`, 6),
					},
					IncludedLabels: []string{"instance", "version", "foo"}, // FIXME foo shouldn't be there because count() doesn't produce it
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(instance, version)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: `count(node_exporter_build_info) by (instance, version)`,
						},
					},
					IsConditional: true,
					Joins: []utils.Source{
						{
							Type:      utils.AggregateSource,
							Returns:   promParser.ValueTypeVector,
							Operation: "count",
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, "deb_package_version", 106),
							},
							IncludedLabels: []string{"instance", "version", "package"},
							FixedLabels:    true,
							ExcludeReason: map[string]utils.ExcludedLabel{
								"": {
									Reason:   "Query is using aggregation with `by(instance, version, package)`, only labels included inside `by(...)` will be present on the results.",
									Fragment: `count(deb_package_version) by (instance, version, package)`,
								},
							},
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
						mustParse[*promParser.VectorSelector](t, `foo`, 7),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(foo)`,
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(foo)`, 0),
				},
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `bar`, 22),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(bar)`,
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(bar)`, 15),
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
						mustParse[*promParser.VectorSelector](t, `foo`, 17),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent_over_time()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent_over_time) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent_over_time(foo[5m])`,
						},
					},
					Call: mustParse[*promParser.Call](t, `absent_over_time(foo[5m])`, 0),
				},
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "absent",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `bar`, 36),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "The [absent()](https://prometheus.io/docs/prometheus/latest/querying/functions/#absent) function is used to check if provided query doesn't match any time series.\nYou will only get any results back if the metric selector you pass doesn't match anything.\nSince there are no matching time series there are also no labels. If some time series is missing you cannot read its labels.\nThis means that the only labels you can get back from absent call are the ones you pass to it.\nIf you're hoping to get instance specific labels this way and alert when some target is down then that won't work, use the `up` metric instead.",
							Fragment: `absent(bar)`,
						},
					},
					Call: mustParse[*promParser.Call](t, `absent(bar)`, 29),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="xxx"}`, 44),
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
					Call: mustParse[*promParser.Call](t, `absent(foo{job="xxx"})`, 37),
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, "bar", 0),
							},
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
						mustParse[*promParser.VectorSelector](t, `foo{job="xxx"}`, 32),
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
					Call: mustParse[*promParser.Call](t, `absent(foo{job="xxx"})`, 25),
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, "bar", 0),
							},
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
					Call: mustParse[*promParser.Call](t, `vector(1)`, 0),
				},
			},
		},
		{
			expr: "vector(scalar(foo))",
			output: []utils.Source{
				{
					Type:          utils.FuncSource,
					Returns:       promParser.ValueTypeVector,
					Operation:     "vector",
					FixedLabels:   true,
					AlwaysReturns: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `vector()` will return a vector value with no labels.",
							Fragment: `vector(scalar(foo))`,
						},
					},
					Call: mustParse[*promParser.Call](t, `vector(scalar(foo))`, 0),
				},
			},
		},
		{
			expr: "vector(0.0  >= bool 0.5) == 1",
			output: []utils.Source{
				{
					Type:          utils.FuncSource,
					Returns:       promParser.ValueTypeVector,
					Operation:     "vector",
					FixedLabels:   true,
					AlwaysReturns: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `vector()` will return a vector value with no labels.",
							Fragment: `vector(0.0  >= bool 0.5)`,
						},
					},
					Call:          mustParse[*promParser.Call](t, `vector(0.0  >= bool 0.5)`, 0),
					IsConditional: true,
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
						mustParse[*promParser.VectorSelector](t, `foo{job="myjob"}`, 14),
					},
					GuaranteedLabels: []string{"job"},
					Call:             mustParse[*promParser.Call](t, `sum_over_time(foo{job="myjob"}[5m])`, 0),
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
					Call: mustParse[*promParser.Call](t, `days_in_month()`, 0),
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
						mustParse[*promParser.VectorSelector](t, `foo{job="foo"}`, 14),
					},
					GuaranteedLabels: []string{"job"},
					Call:             mustParse[*promParser.Call](t, `days_in_month(foo{job="foo"})`, 0),
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
						mustParse[*promParser.VectorSelector](t, `up{job="api-server",service="a:c"}`, 14),
					},
					GuaranteedLabels: []string{"job", "service", "foo"},
					Call:             mustParse[*promParser.Call](t, `label_replace(up{job="api-server",service="a:c"}, "foo", "$1", "service", "(.*):.*")`, 0),
				},
			},
		},
		{
			expr: `label_replace(sum by (pod) (pod_status) > 0, "cluster", "$1", "pod", "(.*)")`,
			output: []utils.Source{
				{
					Type:      utils.FuncSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "label_replace",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `pod_status`, 28),
					},
					FixedLabels:      true,
					IsConditional:    true,
					IncludedLabels:   []string{"pod"},
					GuaranteedLabels: []string{"cluster"},
					Call:             mustParse[*promParser.Call](t, `label_replace(sum by (pod) (pod_status) > 0, "cluster", "$1", "pod", "(.*)")`, 0),
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(pod)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: `sum by (pod) (pod_status)`,
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
						mustParse[*promParser.VectorSelector](t, "my_metric", 10),
					},
					IsConditional: true,
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
						mustParse[*promParser.VectorSelector](t, `up{instance="a", job="prometheus"}`, 0),
					},
					GuaranteedLabels: []string{"instance"},
					ExcludedLabels:   []string{"job"},
					ExcludeReason: map[string]utils.ExcludedLabel{
						"job": {
							Reason:   "Query is using one-to-one vector matching with `ignoring(job)`, all labels included inside `ignoring(...)` will be removed on the results.",
							Fragment: `up{instance="a", job="prometheus"} * ignoring(job) up{instance="a", job="pint"}`,
						},
					},
					Joins: []utils.Source{
						{
							Type:    utils.SelectorSource,
							Returns: promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `up{instance="a", job="pint"}`, 51),
							},
							GuaranteedLabels: []string{"instance", "job"},
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
						mustParse[*promParser.VectorSelector](t, `router_anycast_prefix_enabled{cidr_use_case!~".*offpeak.*"}`, 41),
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
					IsConditional: true,
				},
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `router_anycast_prefix_enabled{cidr_use_case=~".*tier1.*"}`, 155),
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
					IsConditional: true,
					Joins: []utils.Source{
						{
							Type:      utils.AggregateSource,
							Returns:   promParser.ValueTypeVector,
							Operation: "count",
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `colo_router_tier:disabled_pops:max{tier="1",router=~"edge.*"}`, 227),
							},
							FixedLabels: true,
							ExcludeReason: map[string]utils.ExcludedLabel{
								"": {
									Reason:   "Query is using aggregation that removes all labels.",
									Fragment: `count(colo_router_tier:disabled_pops:max{tier="1",router=~"edge.*"})`,
								},
							},
						},
					},
				},
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "avg",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `router_anycast_prefix_enabled{cidr_use_case=~".*regional.*"}`, 343),
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
					IsConditional: true,
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
						mustParse[*promParser.VectorSelector](t, `foo`, 18),
					},
					GuaranteedLabels: []string{"instance"},
					ExcludeReason:    map[string]utils.ExcludedLabel{},
					Call:             mustParse[*promParser.Call](t, `label_replace(sum(foo) without(instance), "instance", "none", "", "")`, 0),
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
						mustParse[*promParser.VectorSelector](t, `probe_success{job="abc"}`, 56),
					},
					IncludedLabels: []string{"region", "target", "colo_name"},
					FixedLabels:    true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation with `by(region, target, colo_name)`, only labels included inside `by(...)` will be present on the results.",
							Fragment: "sum by (region, target, colo_name) (\n    sum_over_time(probe_success{job=\"abc\"}[5m])\n\tor\n\tvector(1)\n)",
						},
					},
					IsConditional: true,
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
					IsConditional: true,
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
					Call: mustParse[*promParser.Call](t, "vector(1)", 0),
				},
				{
					Type:      utils.SelectorSource,
					Operation: promParser.CardManyToMany.String(),
					Returns:   promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, "foo", 13),
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
					Call:          mustParse[*promParser.Call](t, "vector(0)", 0),
					IsConditional: true,
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
						mustParse[*promParser.VectorSelector](t, `foo`, 4),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo or vector(0))`,
						},
					},
					IsConditional: true,
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
					IsConditional: true,
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
						mustParse[*promParser.VectorSelector](t, `foo`, 5),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo or vector(1))`,
						},
					},
					IsConditional: true,
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
					IsConditional: true,
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
						mustParse[*promParser.VectorSelector](t, `foo`, 5),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo or vector(1))`,
						},
					},
					IsConditional: true,
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
					IsConditional: true,
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
						mustParse[*promParser.VectorSelector](t, `foo`, 5),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo or vector(2))`,
						},
					},
					IsConditional: true,
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
					IsConditional: true,
				},
			},
		},
		{
			expr: `
(sum(sometimes{foo!="bar"} or vector(0)))
or
((bob > 10) or sum(foo) or vector(1))`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `sometimes{foo!="bar"}`, 6),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(sometimes{foo!="bar"} or vector(0)))`, // FIXME bogus )
						},
					},
				},
				{
					Type:            utils.AggregateSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "sum",
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{0},
					FixedLabels:     true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(sometimes{foo!="bar"} or vector(0)))`, // FIXME bogus )
						},
					},
				},
				{
					Type:      utils.SelectorSource,
					Operation: promParser.CardManyToMany.String(),
					Returns:   promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `bob`, 48),
					},
					IsConditional: true,
				},
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `foo`, 65),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(foo)`,
						},
					},
				},
				{
					Type:            utils.FuncSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "vector",
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{1},
					FixedLabels:     true,
					Call:            mustParse[*promParser.Call](t, "vector(1)", 73),
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
			expr: `
(
	sum(sometimes{foo!="bar"})
	or
	vector(1)
) and (
	((bob > 10) or sum(bar))
	or
	notfound > 0
)`,
			output: []utils.Source{
				{
					Type:      utils.AggregateSource,
					Returns:   promParser.ValueTypeVector,
					Operation: "sum",
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, `sometimes{foo!="bar"}`, 8),
					},
					FixedLabels: true,
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Query is using aggregation that removes all labels.",
							Fragment: `sum(sometimes{foo!="bar"})`,
						},
					},
					Joins: []utils.Source{
						{
							Type:      utils.SelectorSource,
							Operation: promParser.CardManyToMany.String(),
							Returns:   promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bob`, 57),
							},
							IsConditional: true,
						},
						{
							Type:      utils.AggregateSource,
							Returns:   promParser.ValueTypeVector,
							Operation: "sum",
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 74),
							},
							FixedLabels: true,
							ExcludeReason: map[string]utils.ExcludedLabel{
								"": {
									Reason:   "Query is using aggregation that removes all labels.",
									Fragment: `sum(bar))`, // FIXME bogus )
								},
							},
						},
						{
							Type:      utils.SelectorSource,
							Operation: promParser.CardManyToMany.String(),
							Returns:   promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `notfound`, 85),
							},
							IsConditional: true,
						},
					},
				},
				{
					Type:            utils.FuncSource,
					Returns:         promParser.ValueTypeVector,
					Operation:       "vector",
					AlwaysReturns:   true,
					ReturnedNumbers: []float64{1},
					FixedLabels:     true,
					Call:            mustParse[*promParser.Call](t, "vector(1)", 36),
					ExcludeReason: map[string]utils.ExcludedLabel{
						"": {
							Reason:   "Calling `vector()` will return a vector value with no labels.",
							Fragment: "vector(1)",
						},
					},
					Joins: []utils.Source{
						{
							Type:      utils.SelectorSource,
							Operation: promParser.CardManyToMany.String(),
							Returns:   promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bob`, 57),
							},
							IsConditional: true,
						},
						{
							Type:      utils.AggregateSource,
							Returns:   promParser.ValueTypeVector,
							Operation: "sum",
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `bar`, 74),
							},
							FixedLabels: true,
							ExcludeReason: map[string]utils.ExcludedLabel{
								"": {
									Reason:   "Query is using aggregation that removes all labels.",
									Fragment: `sum(bar))`, // FIXME bogus )
								},
							},
						},
						{
							Type:      utils.SelectorSource,
							Operation: promParser.CardManyToMany.String(),
							Returns:   promParser.ValueTypeVector,
							Selectors: []*promParser.VectorSelector{
								mustParse[*promParser.VectorSelector](t, `notfound`, 85),
							},
							IsConditional: true,
						},
					},
				},
			},
		},
		{
			expr: "foo offset 5m > 5",
			output: []utils.Source{
				{
					Type:    utils.SelectorSource,
					Returns: promParser.ValueTypeVector,
					Selectors: []*promParser.VectorSelector{
						mustParse[*promParser.VectorSelector](t, "foo offset 5m", 0),
					},
					IsConditional: true,
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
