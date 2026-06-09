package checks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/source"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	OffsetCheckName = "promql/offset"
)

func NewOffsetCheck(prom *promapi.FailoverGroup) OffsetCheck {
	return OffsetCheck{
		prom: prom,
	}
}

type OffsetCheck struct {
	prom *promapi.FailoverGroup
}

func (c OffsetCheck) Meta() CheckMeta {
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

func (c OffsetCheck) String() string {
	return fmt.Sprintf("%s(%s)", OffsetCheckName, c.prom.Name())
}

func (c OffsetCheck) Reporter() string {
	return OffsetCheckName
}

func (c OffsetCheck) Check(ctx context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError() != nil {
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
	}

	reason := fmt.Sprintf(
		"%s is configured to only keep %s of metrics history.",
		promText(c.prom.Name(), flags.URI),
		model.Duration(retention),
	)
	problems = append(problems, c.checkSources(expr, retention, reason)...)

	return problems
}

func (c OffsetCheck) checkSources(expr *parser.PromQLExpr, retention time.Duration, reason string) (problems []Problem) {
	for _, src := range expr.Source() {
		src.WalkSources(func(s *source.Source, _ *source.Join, _ *source.Unless) {
			if vs, ok := source.MostOuterOperation[*promParser.VectorSelector](s); ok && vs.OriginalOffset > retention {
				problems = append(problems, c.offsetProblem(expr, s, vs, vs.OriginalOffset, reason))
			}
			// A source can have multiple SubqueryExprs (nested subqueries), check all of them.
			for _, op := range s.Operations {
				sq, ok := op.Node.(*promParser.SubqueryExpr)
				if !ok {
					continue
				}
				if sq.OriginalOffset > retention {
					problems = append(problems, c.offsetProblem(expr, s, sq, sq.OriginalOffset, reason))
				}
			}
		})
	}
	return problems
}

func (c OffsetCheck) offsetProblem(expr *parser.PromQLExpr, s *source.Source, node promParser.Node, offset time.Duration, reason string) Problem {
	firstColumn, lastColumn := findOffsetColumns(expr.Value.Value, s, node)
	return Problem{
		Anchor:   AnchorAfter,
		Lines:    expr.Value.Pos.Lines(),
		Reporter: c.Reporter(),
		Summary:  "query offset beyond configured retention",
		Details:  "",
		Severity: Warning,
		Diagnostics: []diags.Diagnostic{
			{
				Message: fmt.Sprintf(
					"`%s` selector is using a %s offset, but %s",
					node,
					model.Duration(offset),
					reason,
				),
				Pos:         expr.Value.Pos,
				Expr:        expr.Query().Expr,
				FirstColumn: firstColumn,
				LastColumn:  lastColumn,
				Kind:        diags.Issue,
			},
		},
	}
}

func findOffsetColumns(query string, s *source.Source, node promParser.Node) (firstColumn, lastColumn int) {
	nodeStart := int(node.PositionRange().Start)

	var endPos int
	switch node.(type) {
	case *promParser.VectorSelector:
		// If VectorSelector is inside a MatrixSelector, use the MatrixSelector's end position.
		if m, ok := source.MostOuterOperation[*promParser.MatrixSelector](s); ok {
			endPos = int(m.PositionRange().End)
		} else {
			endPos = int(node.PositionRange().End)
		}
	case *promParser.SubqueryExpr:
		endPos = int(node.PositionRange().End)
	}

	idx := strings.LastIndex(strings.ToLower(query[nodeStart:endPos]), "offset")
	firstColumn = nodeStart + idx + 1
	lastColumn = endPos

	return firstColumn, lastColumn
}
