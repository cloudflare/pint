package parser

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"

	promparser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/comments"
)

func appendLine(lines []int, newLines ...int) []int {
	for _, nl := range newLines {
		var present bool
		for _, l := range lines {
			if l == nl {
				present = true
				break
			}
		}
		if !present {
			lines = append(lines, nl)
		}
	}

	return lines
}

func nodeLines(node *yaml.Node, offset int) (lines []int) {
	lineCount := len(strings.Split(strings.TrimSuffix(node.Value, "\n"), "\n"))

	var firstLine int
	// nolint: exhaustive
	switch node.Style {
	case yaml.LiteralStyle, yaml.FoldedStyle:
		firstLine = node.Line + 1 + offset
	default:
		firstLine = node.Line + offset
	}

	for i := 0; i < lineCount; i++ {
		lines = appendLine(lines, firstLine+i)
	}

	return lines
}

func NewFilePosition(l []int) FilePosition {
	return FilePosition{Lines: l}
}

type FilePosition struct {
	Lines []int
}

func (fp FilePosition) FirstLine() (line int) {
	for _, l := range fp.Lines {
		if line == 0 || l < line {
			line = l
		}
	}
	return line
}

func (fp FilePosition) LastLine() (line int) {
	for _, l := range fp.Lines {
		if l > line {
			line = l
		}
	}
	return line
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
	Position FilePosition
	Value    string
}

func newYamlNode(node *yaml.Node, offset int) *YamlNode {
	n := YamlNode{
		Position: NewFilePosition(nodeLines(node, offset)),
		Value:    node.Value,
	}
	if node.Alias != nil {
		n.Value = node.Alias.Value
	}
	return &n
}

func newYamlKeyValue(key, val *yaml.Node, offset int) *YamlKeyValue {
	return &YamlKeyValue{
		Key:   newYamlNode(key, offset),
		Value: newYamlNode(val, offset),
	}
}

type YamlKeyValue struct {
	Key   *YamlNode
	Value *YamlNode
}

func (ykv YamlKeyValue) Lines() (lines []int) {
	lines = appendLine(lines, ykv.Key.Position.Lines...)
	lines = appendLine(lines, ykv.Value.Position.Lines...)
	return lines
}

func (ykv *YamlKeyValue) IsIdentical(b *YamlKeyValue) bool {
	if (ykv == nil) != (b == nil) {
		return false
	}
	if ykv == nil {
		return true
	}
	if ykv.Value.Value != b.Value.Value {
		return false
	}
	return true
}

type YamlMap struct {
	Key   *YamlNode
	Items []*YamlKeyValue
}

func (ym YamlMap) Lines() (lines []int) {
	lines = appendLine(lines, ym.Key.Position.Lines...)
	for _, item := range ym.Items {
		lines = appendLine(lines, item.Lines()...)
	}
	return lines
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

func newYamlMap(key, value *yaml.Node, offset int) *YamlMap {
	ym := YamlMap{
		Key: newYamlNode(key, offset),
	}

	var ckey *yaml.Node
	for _, child := range value.Content {
		if ckey != nil {
			kv := YamlKeyValue{
				Key:   newYamlNode(ckey, offset),
				Value: newYamlNode(child, offset),
			}
			ym.Items = append(ym.Items, &kv)
			ckey = nil
		} else {
			ckey = child
		}
	}

	return &ym
}

type PromQLNode struct {
	Expr     string
	Node     promparser.Expr
	Children []*PromQLNode
}

type PromQLError struct {
	node *PromQLNode
	Err  error
}

func (pqle PromQLError) Error() string {
	return pqle.Err.Error()
}

func (pqle *PromQLError) Unwrap() error {
	return pqle.Err
}

func (pqle PromQLError) Node() *PromQLNode {
	return pqle.node
}

type PromQLExpr struct {
	Key         *YamlNode
	Value       *YamlNode
	SyntaxError error
	Query       *PromQLNode
}

func (pqle PromQLExpr) Lines() (lines []int) {
	lines = appendLine(lines, pqle.Key.Position.Lines...)
	lines = appendLine(lines, pqle.Value.Position.Lines...)
	return lines
}

func (pqle PromQLExpr) IsIdentical(b PromQLExpr) bool {
	return pqle.Value.Value == b.Value.Value
}

func newPromQLExpr(key, val *yaml.Node, offset int) *PromQLExpr {
	expr := PromQLExpr{
		Key:   newYamlNode(key, offset),
		Value: newYamlNode(val, offset),
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
	Alert         YamlKeyValue
	Expr          PromQLExpr
	For           *YamlKeyValue
	KeepFiringFor *YamlKeyValue
	Labels        *YamlMap
	Annotations   *YamlMap
}

func (ar AlertingRule) Lines() (lines []int) {
	lines = appendLine(lines, ar.Alert.Lines()...)
	lines = appendLine(lines, ar.Expr.Lines()...)
	if ar.For != nil {
		lines = appendLine(lines, ar.For.Lines()...)
	}
	if ar.KeepFiringFor != nil {
		lines = appendLine(lines, ar.KeepFiringFor.Lines()...)
	}
	if ar.Labels != nil {
		lines = appendLine(lines, ar.Labels.Lines()...)
	}
	if ar.Annotations != nil {
		lines = appendLine(lines, ar.Annotations.Lines()...)
	}
	slices.Sort(lines)
	return lines
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
	Record YamlKeyValue
	Expr   PromQLExpr
	Labels *YamlMap
}

func (rr RecordingRule) Lines() (lines []int) {
	lines = appendLine(lines, rr.Record.Lines()...)
	lines = appendLine(lines, rr.Expr.Lines()...)
	if rr.Labels != nil {
		lines = appendLine(lines, rr.Labels.Lines()...)
	}
	slices.Sort(lines)
	return lines
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

type ParseError struct {
	Fragment string
	Err      error
	Line     int
}

type Rule struct {
	AlertingRule  *AlertingRule
	RecordingRule *RecordingRule
	Comments      []comments.Comment
	Error         ParseError
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
	if !slices.Equal(r.Lines(), nr.Lines()) {
		return false
	}
	return true
}

func (r Rule) Name() string {
	if r.RecordingRule != nil {
		return r.RecordingRule.Record.Value.Value
	}
	if r.AlertingRule != nil {
		return r.AlertingRule.Alert.Value.Value
	}
	return ""
}

func (r Rule) Expr() PromQLExpr {
	if r.RecordingRule != nil {
		return r.RecordingRule.Expr
	}
	return r.AlertingRule.Expr
}

func (r Rule) Lines() []int {
	if r.RecordingRule != nil {
		return r.RecordingRule.Lines()
	}
	if r.AlertingRule != nil {
		return r.AlertingRule.Lines()
	}
	if r.Error.Err != nil {
		return []int{r.Error.Line}
	}
	return nil
}

func (r Rule) LineRange() []int {
	var lmin, lmax int
	for i, line := range r.Lines() {
		if i == 0 {
			lmin = line
			lmax = line
			continue
		}
		if line < lmin {
			lmin = line
		}
		if line > lmax {
			lmax = line
		}
	}

	lines := []int{}
	for i := lmin; i <= lmax; i++ {
		lines = append(lines, i)
	}
	return lines
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
