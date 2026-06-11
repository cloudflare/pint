package checks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/output"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/source"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	RangeQueryCheckName = "promql/range_query"
)

func NewRangeQueryCheck(prom *promapi.FailoverGroup, limit time.Duration, comment string, severity Severity) RangeQueryCheck {
	var instance string
	if limit > 0 {
		instance = fmt.Sprintf("%s(%s)", RangeQueryCheckName, output.HumanizeDuration(limit))
	} else {
		instance = fmt.Sprintf("%s(%s)", RangeQueryCheckName, prom.Name())
	}

	return RangeQueryCheck{
		prom:     prom,
		limit:    limit,
		comment:  comment,
		severity: severity,
		instance: instance,
	}
}

type RangeQueryCheck struct {
	prom     *promapi.FailoverGroup
	comment  string
	instance string
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
	return c.instance
}

func (c RangeQueryCheck) Reporter() string {
	return RangeQueryCheckName
}

func (c RangeQueryCheck) Check(ctx context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError() != nil {
		return problems
	}

	if c.limit > 0 {
		problems = append(
			problems, c.checkSources(
				expr,
				c.limit,
				fmt.Sprintf("%s is the maximum allowed range query.", model.Duration(c.limit)),
				c.severity,
			)...,
		)
	}

	if c.prom == nil || len(problems) > 0 {
		return problems
	}

	flags, err := c.prom.Flags(ctx).Wait()
	if err != nil {
		if errors.Is(err, promapi.ErrUnsupported) {
			c.prom.DisableCheck(promapi.APIPathFlags, c.Reporter())
			return problems
		}
		problems = append(problems, problemFromError(err, entry.Rule, c.Reporter(), c.prom.Name(), Warning))
		return problems
	}

	retention, p := retentionFromFlags(flags.Flags, c.Reporter(), expr)
	if p != nil {
		problems = append(problems, *p)
		return problems
	}

	problems = append(
		problems, c.checkSources(
			expr,
			retention,
			fmt.Sprintf("%s is configured to only keep %s of metrics history.", promText(c.prom.Name(), flags.URI),
				model.Duration(retention)),
			Warning,
		)...,
	)

	return problems
}

func (c RangeQueryCheck) checkSources(expr *parser.PromQLExpr, retention time.Duration, reason string, s Severity) (problems []Problem) {
	for _, src := range expr.Source() {
		src.WalkSources(func(src *source.Source, _ *source.Join, _ *source.Unless) {
			n, ok := source.MostOuterOperation[*promParser.MatrixSelector](src)
			if !ok {
				return
			}
			if n.Range <= retention {
				return
			}
			problems = append(problems, Problem{
				Anchor:   AnchorAfter,
				Lines:    expr.Value.Pos.Lines(),
				Reporter: c.Reporter(),
				Summary:  "query beyond configured retention",
				Details:  maybeComment(c.comment),
				Severity: s,
				Diagnostics: []diags.Diagnostic{
					{
						Message: fmt.Sprintf(
							"`%s` selector is trying to query Prometheus for %s worth of metrics, but %s",
							n, model.Duration(n.Range), reason,
						),
						Pos:         expr.Value.Pos,
						Expr:        expr.Query().Expr,
						FirstColumn: int(n.PositionRange().Start) + 1,
						LastColumn:  int(n.PositionRange().End),
						Kind:        diags.Issue,
					},
				},
			})
		})
	}
	return problems
}
