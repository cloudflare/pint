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
	problems = append(problems, c.checkNode(ctx, expr, expr.Query(), retention, reason)...)

	return problems
}

func (c OffsetCheck) checkNode(ctx context.Context, expr *parser.PromQLExpr, node *parser.PromQLNode, retention time.Duration, reason string) (problems []Problem) {
	switch n := node.Expr.(type) {
	case *promParser.VectorSelector:
		if n.OriginalOffset > retention {
			problems = append(problems, c.offsetProblem(expr, node, n.OriginalOffset, reason))
		}
	case *promParser.SubqueryExpr:
		if n.OriginalOffset > retention {
			problems = append(problems, c.offsetProblem(expr, node, n.OriginalOffset, reason))
		}
	}

	for _, child := range node.Children {
		problems = append(problems, c.checkNode(ctx, expr, child, retention, reason)...)
	}

	return problems
}

func (c OffsetCheck) offsetProblem(expr *parser.PromQLExpr, node *parser.PromQLNode, offset time.Duration, reason string) Problem {
	firstColumn, lastColumn := findOffsetColumns(expr.Value.Value, node)
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
					"`%s` selector is using %s offset, but %s",
					node.Expr,
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

func findOffsetColumns(query string, node *parser.PromQLNode) (firstColumn, lastColumn int) {
	nodeStart := int(node.Expr.PositionRange().Start)

	var endPos int
	switch node.Expr.(type) {
	case *promParser.VectorSelector:
		if node.Parent != nil {
			if _, ok := node.Parent.Expr.(*promParser.MatrixSelector); ok {
				endPos = int(node.Parent.Expr.PositionRange().End)
				break
			}
		}
		endPos = int(node.Expr.PositionRange().End)
	case *promParser.SubqueryExpr:
		endPos = int(node.Expr.PositionRange().End)
	}

	idx := strings.LastIndex(strings.ToLower(query[nodeStart:endPos]), "offset")
	firstColumn = nodeStart + idx + 1
	lastColumn = endPos

	return firstColumn, lastColumn
}
