package parser

import (
	"errors"
	"slices"
	"sync"

	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/parser/source"
)

type PromQLExpr struct {
	syntaxError error
	Value       *YamlNode
	query       *PromQLNode
	mu          *sync.Mutex
	source      []source.Source
	hasSource   bool
}

func (pn *PromQLExpr) parse() {
	if pn.query == nil {
		pn.query, pn.syntaxError = DecodeExpr(pn.Value.Value)
	}
}

func (pn *PromQLExpr) Source() []source.Source {
	pn.mu.Lock()
	defer pn.mu.Unlock()
	pn.parse()
	if !pn.hasSource {
		pn.source = source.LabelsSource(pn.Value.Value, pn.query.Expr)
		pn.hasSource = true
	}
	return pn.source
}

func (pn *PromQLExpr) Query() *PromQLNode {
	pn.mu.Lock()
	defer pn.mu.Unlock()
	pn.parse()
	return pn.query
}

func (pn *PromQLExpr) SyntaxError() error {
	pn.mu.Lock()
	defer pn.mu.Unlock()
	pn.parse()
	return pn.syntaxError
}

// PromQLNode is used to turn the parsed PromQL query expression into a tree.
// This allows us to walk the tree up & down and look for either parents
// or children of specific type. Which is useful if you, for example,
// want to check if all vector selectors are wrapped inside function
// calls etc.
type PromQLNode struct {
	Parent   *PromQLNode
	Expr     promParser.Node
	Children []*PromQLNode
}

// Tree takes a parsed PromQL node and turns it into a Node
// instance with parent and children populated.
func tree(expr promParser.Node, parent *PromQLNode) *PromQLNode {
	n := PromQLNode{
		Parent:   parent,
		Expr:     expr,
		Children: nil,
	}
	for _, child := range promParser.Children(expr) {
		n.Children = append(n.Children, tree(child, &n))
	}

	return &n
}

// WalkUpExpr allows to iterate a promQLNode node looking for
// parents of specific type.
// Prometheus parser returns interfaces which makes it more difficult
// to figure out what kind of node we're dealing with, hence this
// helper takes a type parameter it tries to cast.
// It starts by checking the node passed to it and then walks
// up by visiting all parent nodes.
func WalkUpExpr[T promParser.Node](node *PromQLNode) (nodes []*PromQLNode) {
	if node == nil {
		return nodes
	}
	if _, ok := node.Expr.(T); ok {
		nodes = append(nodes, node)
	}
	if node.Parent != nil {
		nodes = append(nodes, WalkUpExpr[T](node.Parent)...)
	}
	return nodes
}

// WalkDownExpr works just like WalkUpExpr but it walks the tree
// down, visiting all children.
// It also starts by checking the node passed to it before walking
// down the tree.
func WalkDownExpr[T promParser.Node](node *PromQLNode) (nodes []*PromQLNode) {
	if _, ok := node.Expr.(T); ok {
		nodes = append(nodes, node)
	}
	for _, child := range node.Children {
		nodes = append(nodes, WalkDownExpr[T](child)...)
	}
	return nodes
}

// WalkUpParent works like WalkUpExpr but checks the parent
// (if present) instead of the node itself.
// It returns the nodes where the parent is of given type.
func WalkUpParent[T promParser.Node](node *PromQLNode) (nodes []*PromQLNode) {
	if node == nil || node.Parent == nil {
		return nodes
	}
	if _, ok := node.Parent.Expr.(T); ok {
		nodes = append(nodes, node)
	}
	if node.Parent != nil {
		nodes = append(nodes, WalkUpParent[T](node.Parent)...)
	}
	return nodes
}

func DecodeExpr(expr string) (*PromQLNode, error) {
	node, err := promParser.ParseExpr(expr)
	if err != nil {
		var errorList promParser.ParseErrors
		if errors.As(err, &errorList) {
			// Find the error pointing at the shortest query fragment.
			slices.SortFunc(errorList, func(a, b promParser.ParseErr) int {
				ar := a.PositionRange.End - a.PositionRange.Start
				br := b.PositionRange.End - b.PositionRange.Start
				switch {
				case ar < br:
					return -1
				case ar > br:
					return 1
				default:
					return 0
				}
			})
			for _, el := range errorList {
				if el.PositionRange.Start > 0 && el.PositionRange.End > 0 {
					return nil, promParser.ParseErrors{el}
				}
			}
		}
		return nil, err
	}
	return tree(node, nil), nil
}
