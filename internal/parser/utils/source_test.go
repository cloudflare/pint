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
		output utils.Source
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
			output: utils.Source{
				Type:        utils.NumberSource,
				Returns:     promParser.ValueTypeScalar,
				FixedLabels: true,
			},
		},
		{
			expr: "1 / 5",
			output: utils.Source{
				Type:        utils.NumberSource,
				Returns:     promParser.ValueTypeScalar,
				FixedLabels: true,
			},
		},
		{
			expr: `"test"`,
			output: utils.Source{
				Type:        utils.StringSource,
				Returns:     promParser.ValueTypeString,
				FixedLabels: true,
			},
		},
		{
			expr: "foo",
			output: utils.Source{
				Type:     utils.SelectorSource,
				Returns:  promParser.ValueTypeVector,
				Selector: mustParseVector("foo", 0),
			},
		},
		{
			expr: "foo[5m]",
			output: utils.Source{
				Type:     utils.SelectorSource,
				Returns:  promParser.ValueTypeVector,
				Selector: mustParseVector("foo", 0),
			},
		},
		{
			expr: "prometheus_build_info[2m:1m]",
			output: utils.Source{
				Type:     utils.SelectorSource,
				Returns:  promParser.ValueTypeVector,
				Selector: mustParseVector("prometheus_build_info", 0),
			},
		},
		{
			expr: "deriv(rate(distance_covered_meters_total[1m])[5m:1m])",
			output: utils.Source{
				Type:      utils.FuncSource,
				Returns:   promParser.ValueTypeVector,
				Operation: "deriv",
				Selector:  mustParseVector("distance_covered_meters_total", 11),
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
		{
			expr: "foo - 1",
			output: utils.Source{
				Type:     utils.SelectorSource,
				Returns:  promParser.ValueTypeVector,
				Selector: mustParseVector("foo", 0),
			},
		},
		{
			expr: "foo / 5",
			output: utils.Source{
				Type:     utils.SelectorSource,
				Returns:  promParser.ValueTypeVector,
				Selector: mustParseVector("foo", 0),
			},
		},
		{
			expr: "-foo",
			output: utils.Source{
				Type:     utils.SelectorSource,
				Returns:  promParser.ValueTypeVector,
				Selector: mustParseVector("foo", 1),
			},
		},
		{
			expr: `sum(foo{job="myjob"})`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "sum",
				Selector:    mustParseVector(`foo{job="myjob"}`, 4),
				FixedLabels: true,
			},
		},
		{
			expr: `sum(foo{job="myjob"}) without(job)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      "sum",
				Selector:       mustParseVector(`foo{job="myjob"}`, 4),
				ExcludedLabels: []string{"job"},
				ExcludeReason: map[string]string{
					"job": "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
				},
			},
		},
		{
			expr: `sum(foo) by(job)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      "sum",
				Selector:       mustParseVector(`foo`, 4),
				IncludedLabels: []string{"job"},
				FixedLabels:    true,
				ExcludeReason: map[string]string{
					"": "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"}) by(job)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "sum",
				Selector:         mustParseVector(`foo{job="myjob"}`, 4),
				IncludedLabels:   []string{"job"},
				GuaranteedLabels: []string{"job"},
				FixedLabels:      true,
				ExcludeReason: map[string]string{
					"": "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `sum(foo{job="myjob"}) without(instance)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "sum",
				Selector:         mustParseVector(`foo{job="myjob"}`, 4),
				GuaranteedLabels: []string{"job"},
				ExcludedLabels:   []string{"instance"},
				ExcludeReason: map[string]string{
					"instance": "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
				},
			},
		},
		{
			expr: `min(foo{job="myjob"}) / max(foo{job="myjob"})`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "min",
				Selector:    mustParseVector(`foo{job="myjob"}`, 4),
				FixedLabels: true,
			},
		},
		{
			expr: `max(foo{job="myjob"}) / min(foo{job="myjob"})`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "max",
				Selector:    mustParseVector(`foo{job="myjob"}`, 4),
				FixedLabels: true,
			},
		},
		{
			expr: `avg(foo{job="myjob"}) by(job)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "avg",
				Selector:         mustParseVector(`foo{job="myjob"}`, 4),
				GuaranteedLabels: []string{"job"},
				IncludedLabels:   []string{"job"},
				FixedLabels:      true,
				ExcludeReason: map[string]string{
					"": "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `group(foo) by(job)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      "group",
				Selector:       mustParseVector(`foo`, 6),
				IncludedLabels: []string{"job"},
				FixedLabels:    true,
				ExcludeReason: map[string]string{
					"": "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `stddev(rate(foo[5m]))`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "stddev",
				Selector:    mustParseVector(`foo`, 12),
				FixedLabels: true,
			},
		},
		{
			expr: `stdvar(rate(foo[5m]))`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "stdvar",
				Selector:    mustParseVector(`foo`, 12),
				FixedLabels: true,
			},
		},
		{
			expr: `stddev_over_time(foo[5m])`,
			output: utils.Source{
				Type:      utils.FuncSource,
				Returns:   promParser.ValueTypeVector,
				Operation: "stddev_over_time",
				Selector:  mustParseVector(`foo`, 17),
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
		{
			expr: `stdvar_over_time(foo[5m])`,
			output: utils.Source{
				Type:      utils.FuncSource,
				Returns:   promParser.ValueTypeVector,
				Operation: "stdvar_over_time",
				Selector:  mustParseVector(`foo`, 17),
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
		{
			expr: `quantile(0.9, rate(foo[5m]))`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "quantile",
				Selector:    mustParseVector(`foo`, 19),
				FixedLabels: true,
			},
		},
		{
			expr: `count_values("version", build_version)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "count_values",
				Selector:         mustParseVector(`build_version`, 24),
				GuaranteedLabels: []string{"version"},
				IncludedLabels:   []string{"version"},
				FixedLabels:      true,
			},
		},
		{
			expr: `count_values("version", build_version) without(job)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "count_values",
				Selector:         mustParseVector(`build_version`, 24),
				IncludedLabels:   []string{"version"},
				GuaranteedLabels: []string{"version"},
				ExcludedLabels:   []string{"job"},
				ExcludeReason: map[string]string{
					"job": "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
				},
			},
		},
		{
			expr: `count_values("version", build_version{job="foo"}) without(job)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "count_values",
				Selector:         mustParseVector(`build_version{job="foo"}`, 24),
				IncludedLabels:   []string{"version"},
				GuaranteedLabels: []string{"version"},
				ExcludedLabels:   []string{"job"},
				ExcludeReason: map[string]string{
					"job": "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
				},
			},
		},
		{
			expr: `count_values("version", build_version) by(job)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "count_values",
				Selector:         mustParseVector(`build_version`, 24),
				GuaranteedLabels: []string{"version"},
				IncludedLabels:   []string{"job", "version"},
				FixedLabels:      true,
				ExcludeReason: map[string]string{
					"": "Query is using aggregation with `by(job)`, only labels included inside `by(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `topk(10, foo{job="myjob"}) > 10`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "topk",
				Selector:         mustParseVector(`foo{job="myjob"}`, 9),
				GuaranteedLabels: []string{"job"},
			},
		},
		{
			expr: `rate(foo[10m])`,
			output: utils.Source{
				Type:      utils.FuncSource,
				Returns:   promParser.ValueTypeVector,
				Operation: "rate",
				Selector:  mustParseVector(`foo`, 5),
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
		{
			expr: `sum(rate(foo[10m])) without(instance)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      "sum",
				Selector:       mustParseVector(`foo`, 9),
				ExcludedLabels: []string{"instance"},
				ExcludeReason: map[string]string{
					"instance": "Query is using aggregation with `without(instance)`, all labels included inside `without(...)` will be removed from the results.",
				},
			},
		},
		{
			expr: `foo{job="foo"} / bar`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        promParser.CardOneToOne.String(),
				Selector:         mustParseVector(`foo{job="foo"}`, 0),
				GuaranteedLabels: []string{"job"},
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) bar`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        promParser.CardOneToOne.String(),
				Selector:         mustParseVector(`foo{job="foo"}`, 0),
				GuaranteedLabels: []string{"job"},
				IncludedLabels:   []string{"instance"},
				FixedLabels:      true,
				ExcludeReason: map[string]string{
					"": "Query is using one-to-one vector matching with `on(instance)`, only labels included inside `on(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) group_left(bar) bar`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        promParser.CardManyToOne.String(),
				Selector:         mustParseVector(`foo{job="foo"}`, 0),
				IncludedLabels:   []string{"bar", "instance"},
				GuaranteedLabels: []string{"job"},
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) group_left(cluster) bar{cluster="bar", ignored="true"}`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        promParser.CardManyToOne.String(),
				Selector:         mustParseVector(`foo{job="foo"}`, 0),
				IncludedLabels:   []string{"cluster", "instance"},
				GuaranteedLabels: []string{"job"},
			},
		},
		{
			expr: `foo{job="foo", ignored="true"} * on(instance) group_right(job) bar{cluster="bar"}`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        promParser.CardOneToMany.String(),
				Selector:         mustParseVector(`bar{cluster="bar"}`, 63),
				IncludedLabels:   []string{"job", "instance"},
				GuaranteedLabels: []string{"cluster"},
			},
		},
		{
			expr: `count(foo / bar)`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "count",
				Selector:    mustParseVector(`foo`, 6),
				FixedLabels: true,
			},
		},
		{
			expr: `count(up{job="a"} / on () up{job="b"})`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "count",
				Selector:    mustParseVector(`up{job="a"}`, 6),
				FixedLabels: true,
				ExcludeReason: map[string]string{
					"": "Query is using one-to-one vector matching with `on()`, only labels included inside `on(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `count(up{job="a"} / on (env) up{job="b"})`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "count",
				Selector:    mustParseVector(`up{job="a"}`, 6),
				FixedLabels: true,
				ExcludeReason: map[string]string{
					"": "Query is using one-to-one vector matching with `on(env)`, only labels included inside `on(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `foo{job="foo", instance="1"} and bar`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        promParser.CardManyToMany.String(),
				Selector:         mustParseVector(`foo{job="foo", instance="1"}`, 0),
				GuaranteedLabels: []string{"job", "instance"},
			},
		},
		{
			expr: `foo{job="foo", instance="1"} and on(cluster) bar`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        promParser.CardManyToMany.String(),
				Selector:         mustParseVector(`foo{job="foo", instance="1"}`, 0),
				IncludedLabels:   []string{"cluster"},
				GuaranteedLabels: []string{"job", "instance"},
			},
		},
		{
			expr: `topk(10, foo)`,
			output: utils.Source{
				Type:      utils.AggregateSource,
				Returns:   promParser.ValueTypeVector,
				Operation: "topk",
				Selector:  mustParseVector(`foo`, 9),
			},
		},
		{
			expr: `topk(10, foo) without(cluster)`,
			output: utils.Source{
				Type:      utils.AggregateSource,
				Returns:   promParser.ValueTypeVector,
				Operation: "topk",
				Selector:  mustParseVector(`foo`, 9),
			},
		},
		{
			expr: `topk(10, foo) by(cluster)`,
			output: utils.Source{
				Type:      utils.AggregateSource,
				Returns:   promParser.ValueTypeVector,
				Operation: "topk",
				Selector:  mustParseVector(`foo`, 9),
			},
		},
		{
			expr: `bottomk(10, sum(rate(foo[5m])) without(job))`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      "bottomk",
				Selector:       mustParseVector(`foo`, 21),
				ExcludedLabels: []string{"job"},
				ExcludeReason: map[string]string{
					"job": "Query is using aggregation with `without(job)`, all labels included inside `without(...)` will be removed from the results.",
				},
			},
		},
		{
			expr: `foo or bar`,
			output: utils.Source{
				Type:      utils.SelectorSource,
				Returns:   promParser.ValueTypeVector,
				Operation: promParser.CardManyToMany.String(),
				Selector:  mustParseVector(`foo`, 0),
				Alternatives: []utils.Source{
					{
						Type:     utils.SelectorSource,
						Returns:  promParser.ValueTypeVector,
						Selector: mustParseVector(`bar`, 7),
					},
				},
			},
		},
		{
			expr: `foo or bar or baz`,
			output: utils.Source{
				Type:      utils.SelectorSource,
				Returns:   promParser.ValueTypeVector,
				Operation: promParser.CardManyToMany.String(),
				Selector:  mustParseVector(`foo`, 0),
				Alternatives: []utils.Source{
					{
						Type:     utils.SelectorSource,
						Returns:  promParser.ValueTypeVector,
						Selector: mustParseVector(`bar`, 7),
					},
					{
						Type:     utils.SelectorSource,
						Returns:  promParser.ValueTypeVector,
						Selector: mustParseVector(`baz`, 14),
					},
				},
			},
		},
		{
			expr: `(foo or bar) or baz`,
			output: utils.Source{
				Type:      utils.SelectorSource,
				Returns:   promParser.ValueTypeVector,
				Operation: promParser.CardManyToMany.String(),
				Selector:  mustParseVector(`foo`, 1),
				Alternatives: []utils.Source{
					{
						Type:     utils.SelectorSource,
						Returns:  promParser.ValueTypeVector,
						Selector: mustParseVector(`bar`, 8),
					},
					{
						Type:     utils.SelectorSource,
						Returns:  promParser.ValueTypeVector,
						Selector: mustParseVector(`baz`, 16),
					},
				},
			},
		},
		{
			expr: `foo unless bar`,
			output: utils.Source{
				Type:      utils.SelectorSource,
				Returns:   promParser.ValueTypeVector,
				Operation: promParser.CardManyToMany.String(),
				Selector:  mustParseVector(`foo`, 0),
			},
		},
		{
			expr: `count(sum(up{job="foo", cluster="dev"}) by(job, cluster) == 0) without(job, cluster)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      "count",
				Selector:       mustParseVector(`up{job="foo", cluster="dev"}`, 10),
				ExcludedLabels: []string{"job", "cluster"}, // FIXME empty
				FixedLabels:    true,
				ExcludeReason: map[string]string{
					"":        "Query is using aggregation with `by(job, cluster)`, only labels included inside `by(...)` will be present on the results.",
					"job":     "Query is using aggregation with `without(job, cluster)`, all labels included inside `without(...)` will be removed from the results.",
					"cluster": "Query is using aggregation with `without(job, cluster)`, all labels included inside `without(...)` will be removed from the results.",
				},
			},
		},
		{
			expr: "year()",
			output: utils.Source{
				Type:        utils.FuncSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "year",
				FixedLabels: true,
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
		{
			expr: "year(foo)",
			output: utils.Source{
				Type:      utils.FuncSource,
				Returns:   promParser.ValueTypeVector,
				Operation: "year",
				Selector:  mustParseVector("foo", 5),
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
		{
			expr: `label_join(up{job="api-server",src1="a",src2="b",src3="c"}, "foo", ",", "src1", "src2", "src3")`,
			output: utils.Source{
				Type:             utils.FuncSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "label_join",
				Selector:         mustParseVector(`up{job="api-server",src1="a",src2="b",src3="c"}`, 11),
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
		{
			expr: `
(
	sum(foo:sum > 0) without(notify)
	* on(job) group_left(notify)
	job:notify
)
and on(job)
sum(foo:count) by(job) > 20`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      "sum",
				Selector:       mustParseVector(`foo:sum`, 8),
				IncludedLabels: []string{"notify", "job"},
				ExcludeReason:  map[string]string{},
			},
		},
		{
			expr: `container_file_descriptors / on (instance, app_name) container_ulimits_soft{ulimit="max_open_files"}`,
			output: utils.Source{
				Type:           utils.SelectorSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      promParser.CardOneToOne.String(),
				Selector:       mustParseVector(`container_file_descriptors`, 0),
				IncludedLabels: []string{"instance", "app_name"},
				FixedLabels:    true,
				ExcludeReason: map[string]string{
					"": "Query is using one-to-one vector matching with `on(instance, app_name)`, only labels included inside `on(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `container_file_descriptors / on (instance, app_name) group_left() container_ulimits_soft{ulimit="max_open_files"}`,
			output: utils.Source{
				Type:           utils.SelectorSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      promParser.CardManyToOne.String(),
				Selector:       mustParseVector(`container_file_descriptors`, 0),
				IncludedLabels: []string{"instance", "app_name"},
			},
		},
		{
			expr: `absent(foo{job="bar"})`,
			output: utils.Source{
				Type:             utils.FuncSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "absent",
				Selector:         mustParseVector(`foo{job="bar"}`, 7),
				IncludedLabels:   []string{"job"},
				GuaranteedLabels: []string{"job"},
				FixedLabels:      true,
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
		{
			expr: `absent(foo{job="bar", cluster!="dev", instance=~".+", env="prod"})`,
			output: utils.Source{
				Type:             utils.FuncSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "absent",
				Selector:         mustParseVector(`foo{job="bar", cluster!="dev", instance=~".+", env="prod"}`, 7),
				IncludedLabels:   []string{"job", "env"},
				GuaranteedLabels: []string{"job", "env"},
				FixedLabels:      true,
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
		{
			expr: `absent(sum(foo) by(job, instance))`,
			output: utils.Source{
				Type:        utils.FuncSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "absent",
				Selector:    mustParseVector(`foo`, 11),
				FixedLabels: true,
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
		{
			expr: `absent(foo{job="prometheus", xxx="1"}) AND on(job) prometheus_build_info`,
			output: utils.Source{
				Type:             utils.FuncSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "absent",
				Selector:         mustParseVector(`foo{job="prometheus", xxx="1"}`, 7),
				IncludedLabels:   []string{"job", "xxx"},
				GuaranteedLabels: []string{"job", "xxx"},
				FixedLabels:      true,
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
		{
			expr: `1 + sum(foo) by(notjob)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      "sum",
				Selector:       mustParseVector(`foo`, 8),
				IncludedLabels: []string{"notjob"},
				FixedLabels:    true,
				ExcludeReason: map[string]string{
					"": "Query is using aggregation with `by(notjob)`, only labels included inside `by(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `count(node_exporter_build_info) by (instance, version) != ignoring(package,version) group_left(foo) count(deb_package_version) by (instance, version, package)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Returns:        promParser.ValueTypeVector,
				Operation:      "count",
				Selector:       mustParseVector(`node_exporter_build_info`, 6),
				IncludedLabels: []string{"instance", "version", "foo"}, // FIXME foo shouldn't be there because count() doesn't produce it
				FixedLabels:    true,
				ExcludeReason: map[string]string{
					"": "Query is using aggregation with `by(instance, version)`, only labels included inside `by(...)` will be present on the results.",
				},
			},
		},
		{
			expr: `absent(foo) or absent(bar)`,
			output: utils.Source{
				Type:        utils.FuncSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "absent",
				Selector:    mustParseVector(`foo`, 7),
				FixedLabels: true,
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
				Alternatives: []utils.Source{
					{
						Type:        utils.FuncSource,
						Returns:     promParser.ValueTypeVector,
						Operation:   "absent",
						Selector:    mustParseVector(`bar`, 22),
						FixedLabels: true,
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
		},
		{
			expr: `absent_over_time(foo[5m]) or absent(bar)`,
			output: utils.Source{
				Type:        utils.FuncSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "absent_over_time",
				Selector:    mustParseVector(`foo`, 17),
				FixedLabels: true,
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
				Alternatives: []utils.Source{
					{
						Type:        utils.FuncSource,
						Returns:     promParser.ValueTypeVector,
						Operation:   "absent",
						Selector:    mustParseVector(`bar`, 36),
						FixedLabels: true,
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
		},
		{
			expr: `bar * on() group_right(cluster, env) absent(foo{job="xxx"})`,
			output: utils.Source{
				Type:             utils.FuncSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "absent",
				Selector:         mustParseVector(`foo{job="xxx"}`, 44),
				IncludedLabels:   []string{"job", "cluster", "env"},
				GuaranteedLabels: []string{"job"},
				FixedLabels:      true,
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
		{
			expr: `bar * on() group_right() absent(foo{job="xxx"})`,
			output: utils.Source{
				Type:             utils.FuncSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "absent",
				Selector:         mustParseVector(`foo{job="xxx"}`, 32),
				IncludedLabels:   []string{"job"},
				GuaranteedLabels: []string{"job"},
				FixedLabels:      true,
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
		{
			expr: "vector(1)",
			output: utils.Source{
				Type:        utils.FuncSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "vector",
				FixedLabels: true,
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
		{
			expr: `sum_over_time(foo{job="myjob"}[5m])`,
			output: utils.Source{
				Type:             utils.FuncSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "sum_over_time",
				Selector:         mustParseVector(`foo{job="myjob"}`, 14),
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
		{
			expr: `days_in_month()`,
			output: utils.Source{
				Type:        utils.FuncSource,
				Returns:     promParser.ValueTypeVector,
				Operation:   "days_in_month",
				FixedLabels: true,
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
		{
			expr: `days_in_month(foo{job="foo"})`,
			output: utils.Source{
				Type:             utils.FuncSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "days_in_month",
				Selector:         mustParseVector(`foo{job="foo"}`, 14),
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
		{
			expr: `label_replace(up{job="api-server",service="a:c"}, "foo", "$1", "service", "(.*):.*")`,
			output: utils.Source{
				Type:             utils.FuncSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        "label_replace",
				Selector:         mustParseVector(`up{job="api-server",service="a:c"}`, 14),
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
		{
			expr: `(time() - my_metric) > 5*3600`,
			output: utils.Source{
				Type:     utils.SelectorSource,
				Returns:  promParser.ValueTypeVector,
				Selector: mustParseVector("my_metric", 10),
			},
		},
		{
			expr: `up{instance="a", job="prometheus"} * ignoring(job) up{instance="a", job="pint"}`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Returns:          promParser.ValueTypeVector,
				Operation:        promParser.CardOneToOne.String(),
				Selector:         mustParseVector(`up{instance="a", job="prometheus"}`, 0),
				GuaranteedLabels: []string{"instance"},
				ExcludedLabels:   []string{"job"},
				ExcludeReason: map[string]string{
					"job": "Query is using one-to-one vector matching with `ignoring(job)`, all labels included inside `ignoring(...)` will be removed on the results.",
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
			output := utils.LabelsSource(n)
			require.EqualExportedValues(t, tc.output, output)
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
			output := utils.LabelsSource(n)
			require.NotNil(t, output.Call, "no call detected in: %q ~> %+v", b.String(), output)
			require.Equal(t, name, output.Operation)
			require.Equal(t, def.ReturnType, output.Returns, "incorrect return type on Source{}")
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
	output := utils.LabelsSource(n)
	require.Nil(t, output.Call, "no call should have been detected in fake function")
}
