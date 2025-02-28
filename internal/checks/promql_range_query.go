package checks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	RangeQueryCheckName = "promql/range_query"
)

func NewRangeQueryCheck(prom *promapi.FailoverGroup, limit time.Duration, comment string, severity Severity) RangeQueryCheck {
	return RangeQueryCheck{prom: prom, limit: limit, comment: comment, severity: severity}
}

type RangeQueryCheck struct {
	prom     *promapi.FailoverGroup
	comment  string
	limit    time.Duration
	severity Severity
}

func (c RangeQueryCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        true,
		AlwaysEnabled: false,
	}
}

func (c RangeQueryCheck) String() string {
	if c.limit > 0 {
		return fmt.Sprintf("%s(%s)", RangeQueryCheckName, output.HumanizeDuration(c.limit))
	}
	return fmt.Sprintf("%s(%s)", RangeQueryCheckName, c.prom.Name())
}

func (c RangeQueryCheck) Reporter() string {
	return RangeQueryCheckName
}

func (c RangeQueryCheck) Check(ctx context.Context, _ discovery.Path, rule parser.Rule, _ []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()
	if expr.SyntaxError != nil {
		return problems
	}

	if c.limit > 0 {
		for _, problem := range c.checkNode(ctx, expr, expr.Query, c.limit, fmt.Sprintf("%s is the maximum allowed range query.", model.Duration(c.limit))) {
			problems = append(problems, Problem{
				Anchor:      AnchorAfter,
				Lines:       expr.Value.Lines,
				Reporter:    c.Reporter(),
				Summary:     problem.summary,
				Details:     maybeComment(c.comment),
				Severity:    c.severity,
				Diagnostics: problem.diags,
			})
		}
	}

	if c.prom == nil || len(problems) > 0 {
		return problems
	}

	flags, err := c.prom.Flags(ctx)
	if err != nil {
		if errors.Is(err, promapi.ErrUnsupported) {
			c.prom.DisableCheck(promapi.APIPathFlags, c.Reporter())
			return problems
		}
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Warning)
		problems = append(problems, Problem{
			Anchor:   AnchorAfter,
			Lines:    expr.Value.Lines,
			Reporter: c.Reporter(),
			Summary:  "unable to run checks",
			Details:  "",
			Severity: severity,
			Diagnostics: []output.Diagnostic{
				{
					Message:     text,
					Pos:         expr.Value.Pos,
					FirstColumn: 1,
					LastColumn:  len(expr.Value.Value),
				},
			},
		})
		return problems
	}

	var retention time.Duration
	if v, ok := flags.Flags["storage.tsdb.retention.time"]; ok {
		r, err := model.ParseDuration(v)
		if err != nil {
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    expr.Value.Lines,
				Reporter: c.Reporter(),
				Summary:  "unable to run checks",
				Details:  "",
				Severity: Warning,
				Diagnostics: []output.Diagnostic{
					{
						Message:     fmt.Sprintf("Cannot parse --storage.tsdb.retention.time=%q flag value: %s", v, err),
						Pos:         expr.Value.Pos,
						FirstColumn: 1,
						LastColumn:  len(expr.Value.Value),
					},
				},
			})
		} else {
			retention = time.Duration(r)
		}
	}
	if retention <= 0 {
		// Default Prometheus retention
		// https://prometheus.io/docs/prometheus/latest/storage/#operational-aspects
		retention = time.Hour * 24 * 15
	}

	for _, problem := range c.checkNode(ctx, expr, expr.Query, retention, fmt.Sprintf("%s is configured to only keep %s of metrics history.", promText(c.prom.Name(), flags.URI), model.Duration(retention))) {
		problems = append(problems, Problem{
			Anchor:      AnchorAfter,
			Lines:       expr.Value.Lines,
			Reporter:    c.Reporter(),
			Summary:     "query beyond configured retention",
			Details:     "",
			Severity:    problem.severity,
			Diagnostics: problem.diags,
		})
	}

	return problems
}

func (c RangeQueryCheck) checkNode(ctx context.Context, expr parser.PromQLExpr, node *parser.PromQLNode, retention time.Duration, reason string) (problems []exprProblem) {
	if n, ok := node.Expr.(*promParser.MatrixSelector); ok {
		if n.Range > retention {
			problems = append(problems, exprProblem{
				summary:  "query beyond configured retention",
				details:  "",
				severity: Warning,
				diags: []output.Diagnostic{
					{
						Message: fmt.Sprintf("`%s` selector is trying to query Prometheus for %s worth of metrics, but %s",
							node.Expr, model.Duration(n.Range), reason),
						Pos:         expr.Value.Pos,
						FirstColumn: 1,
						LastColumn:  len(expr.Value.Value),
					},
				},
			})
		}
	}

	for _, child := range node.Children {
		problems = append(problems, c.checkNode(ctx, expr, child, retention, reason)...)
	}

	return problems
}
