package checks

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

const (
	RateCheckName = "promql/rate"
)

func NewRateCheck(prom *promapi.FailoverGroup) RateCheck {
	return RateCheck{prom: prom}
}

type RateCheck struct {
	prom *promapi.FailoverGroup
}

func (c RateCheck) String() string {
	return fmt.Sprintf("%s(%s)", RateCheckName, c.prom.Name())
}

func (c RateCheck) Reporter() string {
	return RateCheckName
}

func (c RateCheck) Check(ctx context.Context, rule parser.Rule, entries []discovery.Entry) (problems []Problem) {
	expr := rule.Expr()

	if expr.SyntaxError != nil {
		return
	}

	cfg, err := c.prom.Config(ctx)
	if err != nil {
		text, severity := textAndSeverityFromError(err, c.Reporter(), c.prom.Name(), Bug)
		problems = append(problems, Problem{
			Fragment: expr.Value.Value,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     text,
			Severity: severity,
		})
		return
	}

	for _, problem := range c.checkNode(expr.Query, cfg) {
		problems = append(problems, Problem{
			Fragment: problem.expr,
			Lines:    expr.Lines(),
			Reporter: c.Reporter(),
			Text:     problem.text,
			Severity: problem.severity,
		})
	}

	return
}

func (c RateCheck) checkNode(node *parser.PromQLNode, cfg *promapi.ConfigResult) (problems []exprProblem) {
	if n, ok := node.Node.(*promParser.Call); ok && (n.Func.Name == "rate" || n.Func.Name == "irate") {
		var minIntervals int
		var recIntervals int
		switch n.Func.Name {
		case "rate":
			minIntervals = 2
			recIntervals = 4
		case "irate":
			minIntervals = 2
			recIntervals = 3
		}
		for _, arg := range n.Args {
			if m, ok := arg.(*promParser.MatrixSelector); ok {
				if m.Range < cfg.Config.Global.ScrapeInterval*time.Duration(minIntervals) {
					p := exprProblem{
						expr: node.Expr,
						text: fmt.Sprintf("duration for %s() must be at least %d x scrape_interval, %s is using %s scrape_interval",
							n.Func.Name, minIntervals, promText(c.prom.Name(), cfg.URI), output.HumanizeDuration(cfg.Config.Global.ScrapeInterval)),
						severity: Bug,
					}
					problems = append(problems, p)
				} else if m.Range < cfg.Config.Global.ScrapeInterval*time.Duration(recIntervals) {
					p := exprProblem{
						expr: node.Expr,
						text: fmt.Sprintf("duration for %s() is recommended to be at least %d x scrape_interval, %s is using %s scrape_interval",
							n.Func.Name, recIntervals, promText(c.prom.Name(), cfg.URI), output.HumanizeDuration(cfg.Config.Global.ScrapeInterval)),
						severity: Warning,
					}
					problems = append(problems, p)
				}
			}
		}
	}

	for _, child := range node.Children {
		problems = append(problems, c.checkNode(child, cfg)...)
	}

	return
}
