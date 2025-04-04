package parser

import (
	"fmt"
	"slices"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/diags"
)

func nodeValue(node *yaml.Node) string {
	if node.Alias != nil {
		return node.Alias.Value
	}
	return node.Value
}

func mergeComments(node *yaml.Node) (comments []string) {
	if node.HeadComment != "" {
		comments = append(comments, node.HeadComment)
	}
	if node.LineComment != "" {
		comments = append(comments, node.LineComment)
	}
	if node.FootComment != "" {
		comments = append(comments, node.FootComment)
	}
	for _, child := range node.Content {
		comments = append(comments, mergeComments(child)...)
	}
	return comments
}

type YamlNode struct {
	Value string
	Pos   diags.PositionRanges
}

func (yn *YamlNode) IsIdentical(b *YamlNode) bool {
	if (yn == nil) != (b == nil) {
		return false
	}
	if yn == nil {
		return true
	}
	if yn.Value != b.Value {
		return false
	}
	return true
}

func newYamlNode(node *yaml.Node, offsetLine, offsetColumn int, contentLines []string, minColumn int) *YamlNode {
	pos := diags.NewPositionRange(contentLines, node, minColumn)
	pos.AddOffset(offsetLine, offsetColumn)
	return &YamlNode{
		Pos:   pos,
		Value: nodeValue(node),
	}
}

type YamlKeyValue struct {
	Key   *YamlNode
	Value *YamlNode
}

type YamlMap struct {
	Key   *YamlNode
	Items []*YamlKeyValue
}

func (ym *YamlMap) IsIdentical(b *YamlMap) bool {
	var al, bl []string

	if ym != nil && ym.Items != nil {
		al = make([]string, 0, len(ym.Items))
		for _, kv := range ym.Items {
			al = append(al, fmt.Sprintf("%s: %s", kv.Key.Value, kv.Value.Value))
		}
		slices.Sort(al)
	}

	if b != nil && b.Items != nil {
		bl = make([]string, 0, len(b.Items))
		for _, kv := range b.Items {
			bl = append(bl, fmt.Sprintf("%s: %s", kv.Key.Value, kv.Value.Value))
		}
		slices.Sort(bl)
	}

	return slices.Equal(al, bl)
}

func (ym YamlMap) GetValue(key string) *YamlNode {
	for _, child := range ym.Items {
		if child.Key.Value == key {
			return child.Value
		}
	}
	return nil
}

func (ym YamlMap) Lines() (lr diags.LineRange) {
	lr = ym.Key.Pos.Lines()
	for _, item := range ym.Items {
		lr.First = min(lr.First, item.Key.Pos.Lines().First)
		lr.Last = max(lr.Last, item.Value.Pos.Lines().Last)
	}
	return lr
}

func newYamlMap(key, value *yaml.Node, offsetLine, offsetColumn int, contentLines []string) *YamlMap {
	ym := YamlMap{
		Key:   newYamlNode(key, offsetLine, offsetColumn, contentLines, 1),
		Items: nil,
	}

	var ckey *yaml.Node
	for _, child := range value.Content {
		if ckey != nil {
			kv := YamlKeyValue{
				Key:   newYamlNode(ckey, offsetLine, offsetColumn, contentLines, key.Column+2),
				Value: newYamlNode(child, offsetLine, offsetColumn, contentLines, ckey.Column+2),
			}
			ym.Items = append(ym.Items, &kv)
			ckey = nil
		} else {
			ckey = child
		}
	}

	return &ym
}

func (pqle PromQLExpr) IsIdentical(b PromQLExpr) bool {
	return pqle.Value.Value == b.Value.Value
}

func newPromQLExpr(node *yaml.Node, offsetLine, offsetColumn int, contentLines []string, minColumn int) *PromQLExpr {
	expr := PromQLExpr{
		Value:       newYamlNode(node, offsetLine, offsetColumn, contentLines, minColumn),
		SyntaxError: nil,
		Query:       nil,
	}

	qlNode, err := DecodeExpr(expr.Value.Value)
	if err != nil {
		expr.SyntaxError = err
		return &expr

	}
	expr.Query = qlNode
	return &expr
}

type AlertingRule struct {
	Expr          PromQLExpr
	For           *YamlNode
	KeepFiringFor *YamlNode
	Labels        *YamlMap
	Annotations   *YamlMap
	Alert         YamlNode
}

func (ar *AlertingRule) IsIdentical(b *AlertingRule) bool {
	if (ar == nil) != (b == nil) {
		return false
	}
	if ar == nil {
		return true
	}
	if !ar.Alert.IsIdentical(&b.Alert) {
		return false
	}
	if !ar.Expr.IsIdentical(b.Expr) {
		return false
	}
	if !ar.For.IsIdentical(b.For) {
		return false
	}
	if !ar.KeepFiringFor.IsIdentical(b.KeepFiringFor) {
		return false
	}
	if !ar.Labels.IsIdentical(b.Labels) {
		return false
	}
	if !ar.Annotations.IsIdentical(b.Annotations) {
		return false
	}
	return true
}

type RecordingRule struct {
	Expr   PromQLExpr
	Labels *YamlMap
	Record YamlNode
}

func (rr *RecordingRule) IsIdentical(b *RecordingRule) bool {
	if (rr == nil) != (b == nil) {
		return false
	}
	if rr == nil {
		return true
	}
	if !rr.Record.IsIdentical(&b.Record) {
		return false
	}
	if !rr.Expr.IsIdentical(b.Expr) {
		return false
	}
	if !rr.Labels.IsIdentical(b.Labels) {
		return false
	}
	return true
}

// Use insread of StrictError.
type ParseError struct {
	Err     error
	Details string
	Line    int
}

func (pe ParseError) Error() string {
	return fmt.Sprintf("error at line %d: %s", pe.Line, pe.Err)
}

type Rule struct {
	AlertingRule  *AlertingRule
	RecordingRule *RecordingRule
	Comments      []comments.Comment
	Error         ParseError
	Lines         diags.LineRange
}

func (r Rule) IsIdentical(b Rule) bool {
	if r.Type() != b.Type() {
		return false
	}
	if !r.AlertingRule.IsIdentical(b.AlertingRule) {
		return false
	}
	if !r.RecordingRule.IsIdentical(b.RecordingRule) {
		return false
	}

	ac := make([]string, 0, len(r.Comments))
	for _, c := range r.Comments {
		ac = append(ac, c.Value.String())
	}
	slices.Sort(ac)

	bc := make([]string, 0, len(r.Comments))
	for _, c := range b.Comments {
		bc = append(bc, c.Value.String())
	}
	slices.Sort(bc)

	return slices.Equal(ac, bc)
}

func (r Rule) IsSame(nr Rule) bool {
	if (r.AlertingRule != nil) != (nr.AlertingRule != nil) {
		return false
	}
	if (r.RecordingRule != nil) != (nr.RecordingRule != nil) {
		return false
	}
	if r.Error != nr.Error {
		return false
	}
	if r.Lines.First != nr.Lines.First {
		return false
	}
	if r.Lines.Last != nr.Lines.Last {
		return false
	}
	return true
}

func (r Rule) Name() string {
	if r.RecordingRule != nil {
		return r.RecordingRule.Record.Value
	}
	if r.AlertingRule != nil {
		return r.AlertingRule.Alert.Value
	}
	return ""
}

func (r Rule) NameNode() YamlNode {
	if r.RecordingRule != nil {
		return r.RecordingRule.Record
	}
	return r.AlertingRule.Alert
}

func (r Rule) Expr() PromQLExpr {
	if r.RecordingRule != nil {
		return r.RecordingRule.Expr
	}
	return r.AlertingRule.Expr
}

func (r Rule) LastKey() (node *YamlNode) {
	if r.RecordingRule != nil {
		node = &r.RecordingRule.Record
		if r.RecordingRule.Expr.Value.Pos.Lines().Last > node.Pos.Lines().Last {
			node = r.RecordingRule.Expr.Value
		}
		if r.RecordingRule.Labels != nil {
			for _, lab := range r.RecordingRule.Labels.Items {
				if lab.Key.Pos.Lines().Last > node.Pos.Lines().Last {
					node = lab.Key
				}
			}
		}
	}
	if r.AlertingRule != nil {
		node = &r.AlertingRule.Alert
		if r.AlertingRule.Expr.Value.Pos.Lines().Last > node.Pos.Lines().Last {
			node = r.AlertingRule.Expr.Value
		}
		if r.AlertingRule.For != nil && r.AlertingRule.For.Pos.Lines().Last > node.Pos.Lines().Last {
			node = r.AlertingRule.For
		}
		if r.AlertingRule.KeepFiringFor != nil && r.AlertingRule.KeepFiringFor.Pos.Lines().Last > node.Pos.Lines().Last {
			node = r.AlertingRule.KeepFiringFor
		}
		if r.AlertingRule.Labels != nil {
			for _, lab := range r.AlertingRule.Labels.Items {
				if lab.Key.Pos.Lines().Last > node.Pos.Lines().Last {
					node = lab.Key
				}
			}
		}
		if r.AlertingRule.Annotations != nil {
			for _, ann := range r.AlertingRule.Annotations.Items {
				if ann.Key.Pos.Lines().Last > node.Pos.Lines().Last {
					node = ann.Key
				}
			}
		}
	}
	return node
}

type RuleType string

const (
	AlertingRuleType  RuleType = "alerting"
	RecordingRuleType RuleType = "recording"
	InvalidRuleType   RuleType = "invalid"
)

func (r Rule) Type() RuleType {
	if r.AlertingRule != nil {
		return AlertingRuleType
	}
	if r.RecordingRule != nil {
		return RecordingRuleType
	}
	return InvalidRuleType
}

type File struct {
	Comments  []comments.Comment
	Groups    []Group
	Error     ParseError
	IsRelaxed bool
}

type Group struct {
	Labels      map[string]string
	Name        string
	Error       ParseError
	Rules       []Rule
	Interval    time.Duration
	QueryOffset time.Duration
	Limit       int
}
