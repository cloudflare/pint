package checks

import (
	"context"
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
		IsOnline: true,
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
		for _, problem := range c.checkNode(ctx, expr.Query, c.limit, fmt.Sprintf("%s is the maximum allowed range query.", model.Duration(c.limit))) {
			problems = append(problems, Problem{
				Lines:    expr.Value.Lines,
				Reporter: c.Reporter(),
				Text:     problem.text,
				Details:  maybeComment(c.comment),
				Severity: c.severity,
			})
		}
	}

	if c.prom == nil || len(problems) > 0 {
		return problems
	}

	flags, err := c.prom.Flags(ctx)
	if err != nil {
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Warning)
		problems = append(problems, Problem{
			Lines:    expr.Value.Lines,
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return problems
	}

	var retention time.Duration
	if v, ok := flags.Flags["storage.tsdb.retention.time"]; ok {
		r, err := model.ParseDuration(v)
		if err != nil {
			problems = append(problems, Problem{
				Lines:    expr.Value.Lines,
				Reporter: c.Reporter(),
				Text:     fmt.Sprintf("Cannot parse --storage.tsdb.retention.time=%q flag value: %s", v, err),
				Severity: Warning,
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

	for _, problem := range c.checkNode(ctx, expr.Query, retention, fmt.Sprintf("%s is configured to only keep %s of metrics history.", promText(c.prom.Name(), flags.URI), model.Duration(retention))) {
		problems = append(problems, Problem{
			Lines:    expr.Value.Lines,
			Reporter: c.Reporter(),
			Text:     problem.text,
			Severity: problem.severity,
		})
	}

	return problems
}

func (c RangeQueryCheck) checkNode(ctx context.Context, node *parser.PromQLNode, retention time.Duration, reason string) (problems []exprProblem) {
	if n, ok := node.Expr.(*promParser.MatrixSelector); ok {
		if n.Range > retention {
			problems = append(problems, exprProblem{
				expr: node.Expr.String(),
				text: fmt.Sprintf("`%s` selector is trying to query Prometheus for %s worth of metrics, but %s",
					node.Expr, model.Duration(n.Range), reason),
				severity: Warning,
			})
		}
	}

	for _, child := range node.Children {
		problems = append(problems, c.checkNode(ctx, child, retention, reason)...)
	}

	return problems
}
