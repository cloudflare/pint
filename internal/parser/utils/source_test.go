package utils_test

import (
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
				Type: utils.NumberSource,
			},
		},
		{
			expr: `"test"`,
			output: utils.Source{
				Type: utils.StringSource,
			},
		},
		{
			expr: "foo",
			output: utils.Source{
				Type:     utils.SelectorSource,
				Selector: mustParseVector("foo", 0),
			},
		},
		{
			expr: "foo[5m]",
			output: utils.Source{
				Type:     utils.SelectorSource,
				Selector: mustParseVector("foo", 0),
			},
		},
		{
			expr: "prometheus_build_info[2m:1m]",
			output: utils.Source{
				Type:     utils.SelectorSource,
				Selector: mustParseVector("prometheus_build_info", 0),
			},
		},
		{
			expr: "deriv(rate(distance_covered_meters_total[1m])[5m:1m])",
			output: utils.Source{
				Type:      utils.FuncSource,
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
				Selector: mustParseVector("foo", 0),
			},
		},
		{
			expr: "-foo",
			output: utils.Source{
				Type:     utils.SelectorSource,
				Selector: mustParseVector("foo", 1),
			},
		},
		{
			expr: `sum(foo{job="myjob"})`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Operation:   "sum",
				Selector:    mustParseVector(`foo{job="myjob"}`, 4),
				FixedLabels: true,
			},
		},
		{
			expr: `sum(foo{job="myjob"}) without(job)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Operation:      "sum",
				Selector:       mustParseVector(`foo{job="myjob"}`, 4),
				ExcludedLabels: []string{"job"},
			},
		},
		{
			expr: `sum(foo) by(job)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Operation:      "sum",
				Selector:       mustParseVector(`foo`, 4),
				IncludedLabels: []string{"job"},
				FixedLabels:    true,
			},
		},
		{
			expr: `sum(foo{job="myjob"}) by(job)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Operation:        "sum",
				Selector:         mustParseVector(`foo{job="myjob"}`, 4),
				IncludedLabels:   []string{"job"},
				GuaranteedLabels: []string{"job"},
				FixedLabels:      true,
			},
		},
		{
			expr: `sum(foo{job="myjob"}) without(instance)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Operation:        "sum",
				Selector:         mustParseVector(`foo{job="myjob"}`, 4),
				GuaranteedLabels: []string{"job"},
				ExcludedLabels:   []string{"instance"},
			},
		},
		{
			expr: `min(foo{job="myjob"}) / max(foo{job="myjob"})`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Operation:   "min",
				Selector:    mustParseVector(`foo{job="myjob"}`, 4),
				FixedLabels: true,
			},
		},
		{
			expr: `max(foo{job="myjob"}) / min(foo{job="myjob"})`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Operation:   "max",
				Selector:    mustParseVector(`foo{job="myjob"}`, 4),
				FixedLabels: true,
			},
		},
		{
			expr: `avg(foo{job="myjob"}) by(job)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Operation:        "avg",
				Selector:         mustParseVector(`foo{job="myjob"}`, 4),
				GuaranteedLabels: []string{"job"},
				IncludedLabels:   []string{"job"},
				FixedLabels:      true,
			},
		},
		{
			expr: `group(foo) by(job)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Operation:      "group",
				Selector:       mustParseVector(`foo`, 6),
				IncludedLabels: []string{"job"},
				FixedLabels:    true,
			},
		},
		{
			expr: `stddev(rate(foo[5m]))`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Operation:   "stddev",
				Selector:    mustParseVector(`foo`, 12),
				FixedLabels: true,
			},
		},
		{
			expr: `stdvar(rate(foo[5m]))`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Operation:   "stdvar",
				Selector:    mustParseVector(`foo`, 12),
				FixedLabels: true,
			},
		},
		{
			expr: `stddev_over_time(foo[5m])`,
			output: utils.Source{
				Type:      utils.FuncSource,
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
				Operation:   "quantile",
				Selector:    mustParseVector(`foo`, 19),
				FixedLabels: true,
			},
		},
		{
			expr: `count_values("version", build_version)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
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
				Operation:        "count_values",
				Selector:         mustParseVector(`build_version`, 24),
				IncludedLabels:   []string{"version"},
				GuaranteedLabels: []string{"version"},
				ExcludedLabels:   []string{"job"},
			},
		},
		{
			expr: `count_values("version", build_version{job="foo"}) without(job)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Operation:        "count_values",
				Selector:         mustParseVector(`build_version{job="foo"}`, 24),
				IncludedLabels:   []string{"version"},
				GuaranteedLabels: []string{"version"},
				ExcludedLabels:   []string{"job"},
			},
		},
		{
			expr: `count_values("version", build_version) by(job)`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Operation:        "count_values",
				Selector:         mustParseVector(`build_version`, 24),
				GuaranteedLabels: []string{"version"},
				IncludedLabels:   []string{"job", "version"},
				FixedLabels:      true,
			},
		},
		{
			expr: `topk(10, foo{job="myjob"}) > 10`,
			output: utils.Source{
				Type:             utils.AggregateSource,
				Operation:        "topk",
				Selector:         mustParseVector(`foo{job="myjob"}`, 9),
				GuaranteedLabels: []string{"job"},
			},
		},
		{
			expr: `rate(foo[10m])`,
			output: utils.Source{
				Type:      utils.FuncSource,
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
				Operation:      "sum",
				Selector:       mustParseVector(`foo`, 9),
				ExcludedLabels: []string{"instance"},
			},
		},
		{
			expr: `foo{job="foo"} / bar`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Operation:        promParser.CardOneToOne.String(),
				Selector:         mustParseVector(`foo{job="foo"}`, 0),
				GuaranteedLabels: []string{"job"},
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) bar`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Operation:        promParser.CardOneToOne.String(),
				Selector:         mustParseVector(`foo{job="foo"}`, 0),
				GuaranteedLabels: []string{"job"},
				IncludedLabels:   []string{"instance"},
				FixedLabels:      true,
			},
		},
		{
			expr: `foo{job="foo"} * on(instance) group_left(bar) bar`,
			output: utils.Source{
				Type:             utils.SelectorSource,
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
				Operation:   "count",
				Selector:    mustParseVector(`foo`, 6),
				FixedLabels: true,
			},
		},
		{
			expr: `count(up{job="a"} / on () up{job="b"})`,
			output: utils.Source{
				Type:        utils.AggregateSource,
				Operation:   "count",
				Selector:    mustParseVector(`up{job="a"}`, 6),
				FixedLabels: true,
			},
		},
		{
			expr: `foo{job="foo", instance="1"} and bar`,
			output: utils.Source{
				Type:             utils.SelectorSource,
				Operation:        promParser.CardManyToMany.String(),
				Selector:         mustParseVector(`foo{job="foo", instance="1"}`, 0),
				GuaranteedLabels: []string{"job", "instance"},
			},
		},
		{
			expr: `foo{job="foo", instance="1"} and on(cluster) bar`,
			output: utils.Source{
				Type:             utils.SelectorSource,
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
				Operation: "topk",
				Selector:  mustParseVector(`foo`, 9),
			},
		},
		{
			expr: `bottomk(10, sum(rate(foo[5m])) without(job))`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Operation:      "bottomk",
				Selector:       mustParseVector(`foo`, 21),
				ExcludedLabels: []string{"job"},
			},
		},
		{
			expr: `foo or bar`,
			output: utils.Source{
				Type:      utils.SelectorSource,
				Operation: promParser.CardManyToMany.String(),
				Selector:  mustParseVector(`foo`, 0),
				Alternatives: []utils.Source{
					{
						Type:     utils.SelectorSource,
						Selector: mustParseVector(`bar`, 7),
					},
				},
			},
		},
		{
			expr: `foo or bar or baz`,
			output: utils.Source{
				Type:      utils.SelectorSource,
				Operation: promParser.CardManyToMany.String(),
				Selector:  mustParseVector(`foo`, 0),
				Alternatives: []utils.Source{
					{
						Type:     utils.SelectorSource,
						Selector: mustParseVector(`bar`, 7),
					},
					{
						Type:     utils.SelectorSource,
						Selector: mustParseVector(`baz`, 14),
					},
				},
			},
		},
		{
			expr: `(foo or bar) or baz`,
			output: utils.Source{
				Type:      utils.SelectorSource,
				Operation: promParser.CardManyToMany.String(),
				Selector:  mustParseVector(`foo`, 1),
				Alternatives: []utils.Source{
					{
						Type:     utils.SelectorSource,
						Selector: mustParseVector(`bar`, 8),
					},
					{
						Type:     utils.SelectorSource,
						Selector: mustParseVector(`baz`, 16),
					},
				},
			},
		},
		{
			expr: `count(sum(up{job="foo", cluster="dev"}) by(job, cluster) == 0) without(job, cluster)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Operation:      "count",
				Selector:       mustParseVector(`up{job="foo", cluster="dev"}`, 10),
				ExcludedLabels: []string{"job", "cluster"},
				FixedLabels:    true,
			},
		},
		{
			expr: "year()",
			output: utils.Source{
				Type:      utils.FuncSource,
				Operation: "year",
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
				Type:      utils.FuncSource,
				Operation: "label_join",
				Selector:  mustParseVector(`up{job="api-server",src1="a",src2="b",src3="c"}`, 11),
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
				Operation:      "sum",
				Selector:       mustParseVector(`foo:sum`, 8),
				IncludedLabels: []string{"notify", "job"},
			},
		},
		{
			expr: `container_file_descriptors / on (instance, app_name) container_ulimits_soft{ulimit="max_open_files"}`,
			output: utils.Source{
				Type:           utils.SelectorSource,
				Operation:      promParser.CardOneToOne.String(),
				Selector:       mustParseVector(`container_file_descriptors`, 0),
				IncludedLabels: []string{"instance", "app_name"},
				FixedLabels:    true,
			},
		},
		{
			expr: `container_file_descriptors / on (instance, app_name) group_left() container_ulimits_soft{ulimit="max_open_files"}`,
			output: utils.Source{
				Type:           utils.SelectorSource,
				Operation:      promParser.CardManyToOne.String(),
				Selector:       mustParseVector(`container_file_descriptors`, 0),
				IncludedLabels: []string{"instance", "app_name"},
			},
		},
		{
			expr: `absent(foo{job="bar"})`,
			output: utils.Source{
				Type:             utils.FuncSource,
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
				Operation:      "sum",
				Selector:       mustParseVector(`foo`, 8),
				IncludedLabels: []string{"notjob"},
				FixedLabels:    true,
			},
		},
		{
			expr: `count(node_exporter_build_info) by (instance, version) != ignoring(package,version) group_left(foo) count(deb_package_version) by (instance, version, package)`,
			output: utils.Source{
				Type:           utils.AggregateSource,
				Operation:      "count",
				Selector:       mustParseVector(`node_exporter_build_info`, 6),
				IncludedLabels: []string{"instance", "version", "foo"}, // FIXME foo shouldn't be there because count() doesn't produce it
				FixedLabels:    true,
			},
		},
		{
			expr: `absent(foo) or absent(bar)`,
			output: utils.Source{
				Type:        utils.FuncSource,
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
			expr: `bar * on() group_right(cluster, env) absent(foo{job="xxx"})`,
			output: utils.Source{
				Type:             utils.FuncSource,
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
