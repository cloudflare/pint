package parser

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cloudflare/pint/internal/comments"
)

func nodeLines(node *yaml.Node, offset int) (lr LineRange) {
	switch {
	case node.Value == "":
		lr.First = node.Line + offset
		lr.Last = node.Line + offset
	case node.Style == yaml.LiteralStyle || node.Style == yaml.FoldedStyle:
		lr.First = node.Line + 1 + offset
		lr.Last = lr.First + len(strings.Split(strings.TrimSuffix(node.Value, "\n"), "\n")) - 1
	default:
		lr.First = node.Line + offset
		lr.Last = node.Line + offset
	}
	return lr
}

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
	Value  string
	Lines  LineRange
	Column int
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

func newYamlNode(node *yaml.Node, offsetLine, offsetColumn int) *YamlNode {
	n := YamlNode{
		Lines:  nodeLines(node, offsetLine),
		Value:  nodeValue(node),
		Column: node.Column + offsetColumn,
	}
	return &n
}

func newYamlNodeWithKey(key, node *yaml.Node, offsetLine, offsetColumn int) *YamlNode {
	keyLines := nodeLines(key, offsetLine)
	valLines := nodeLines(node, offsetLine)
	n := YamlNode{
		Lines: LineRange{
			First: min(keyLines.First, valLines.First),
			Last:  max(keyLines.Last, valLines.Last),
		},
		Value:  nodeValue(node),
		Column: node.Column + offsetColumn,
	}
	return &n
}

type YamlKeyValue struct {
	Key   *YamlNode
	Value *YamlNode
}

type YamlMap struct {
	Key   *YamlNode
	Items []*YamlKeyValue
	Lines LineRange
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

func newYamlMap(key, value *yaml.Node, offsetLine, offsetColumn int) *YamlMap {
	ym := YamlMap{
		Key:   newYamlNode(key, offsetLine, offsetColumn),
		Items: nil,
		Lines: LineRange{
			First: key.Line + offsetLine,
			Last:  key.Line + offsetLine,
		},
	}

	var ckey *yaml.Node
	for _, child := range value.Content {
		if ckey != nil {
			kv := YamlKeyValue{
				Key:   newYamlNode(ckey, offsetLine, offsetColumn),
				Value: newYamlNode(child, offsetLine, offsetColumn),
			}
			if kv.Value.Lines.Last > ym.Lines.Last {
				ym.Lines.Last = kv.Value.Lines.Last
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

func newPromQLExpr(key, val *yaml.Node, offsetLine, offsetColumn int) *PromQLExpr {
	expr := PromQLExpr{
		Value:       newYamlNodeWithKey(key, val, offsetLine, offsetColumn),
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

type LineRange struct {
	First int
	Last  int
}

func (lr LineRange) String() string {
	if lr.First == lr.Last {
		return strconv.Itoa(lr.First)
	}
	return fmt.Sprintf("%d-%d", lr.First, lr.Last)
}

func (lr LineRange) Expand() []int {
	lines := make([]int, 0, lr.Last-lr.First+1)
	for i := lr.First; i <= lr.Last; i++ {
		lines = append(lines, i)
	}
	return lines
}

type Rule struct {
	AlertingRule  *AlertingRule
	RecordingRule *RecordingRule
	Error         ParseError
	Comments      []comments.Comment
	Lines         LineRange
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

func (r Rule) Expr() PromQLExpr {
	if r.RecordingRule != nil {
		return r.RecordingRule.Expr
	}
	return r.AlertingRule.Expr
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

type Result struct {
	Path    string
	Error   error
	Content []byte
	Rules   []Rule
}
