package checks

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	RangeQueryCheckName = "promql/range_query"
)

func NewRangeQueryCheck(prom *promapi.FailoverGroup) RangeQueryCheck {
	return RangeQueryCheck{prom: prom}
}

type RangeQueryCheck struct {
	prom *promapi.FailoverGroup
}

func (c RangeQueryCheck) Meta() CheckMeta {
	return CheckMeta{IsOnline: true}
}

func (c RangeQueryCheck) String() string {
	return fmt.Sprintf("%s(%s)", RangeQueryCheckName, c.prom.Name())
}

func (c RangeQueryCheck) Reporter() string {
	return RangeQueryCheckName
}

func (c RangeQueryCheck) Check(ctx context.Context, path string, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return
	}

	flags, err := c.prom.Flags(ctx)
	if err != nil {
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Warning)
		problems = append(problems, Problem{
			Fragment: expr.Value.Value,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return
	}

	// Default Prometheus retention
	// https://prometheus.io/docs/prometheus/latest/storage/#operational-aspects
	retention := time.Hour * 24 * 15
	if v, ok := flags.Flags["storage.tsdb.retention.time"]; ok {
		r, err := model.ParseDuration(v)
		if err != nil {
			problems = append(problems, Problem{
				Fragment: expr.Value.Value,
				Lines:    expr.Lines(),
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("Cannot parse --storage.tsdb.retention.time=%q flag value: %s", v, err),
				Severity: Warning,
			})
		} else {
			retention = time.Duration(r)
		}
	}

	for _, problem := range c.checkNode(ctx, expr.Query, retention, flags.URI) {
		problems = append(problems, Problem{
			Fragment: problem.expr,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     problem.text,
			Severity: problem.severity,
		})
	}

	return problems
}

func (c RangeQueryCheck) checkNode(ctx context.Context, node *parser.PromQLNode, retention time.Duration, uri string) (problems []exprProblem) {
	if n, ok := node.Node.(*promParser.MatrixSelector); ok {
		if n.Range > retention {
			problems = append(problems, exprProblem{
				expr: node.Expr,
				text: fmt.Sprintf("%s selector is trying to query Prometheus for %s worth of metrics, but %s is configured to only keep %s of metrics history",
					node.Expr, model.Duration(n.Range), promText(c.prom.Name(), uri), model.Duration(retention)),
				severity: Warning,
			})
		}
	}

	for _, child := range node.Children {
		problems = append(problems, c.checkNode(ctx, child, retention, uri)...)
	}

	return problems
}
