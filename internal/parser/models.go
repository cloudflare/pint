package parser

import (
	"strings"

	"gopkg.in/yaml.v3"

	promparser "github.com/prometheus/prometheus/promql/parser"
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

func nodeLines(node *yaml.Node) (lines []int) {
	lineCount := len(strings.Split(strings.TrimSuffix(node.Value, "\n"), "\n"))

	var firstLine int
	switch node.Style {
	case yaml.LiteralStyle, yaml.FoldedStyle:
		firstLine = node.Line + 1
	default:
		firstLine = node.Line
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

func (fp FilePosition) FistLine() (line int) {
	for _, l := range fp.Lines {
		if line == 0 || l < line {
			line = l
		}
	}
	return
}

func (fp FilePosition) LastLine() (line int) {
	for _, l := range fp.Lines {
		if l > line {
			line = l
		}
	}
	return
}

type YamlNode struct {
	Position FilePosition
	Value    string
}

func newYamlNode(node *yaml.Node) *YamlNode {
	return &YamlNode{
		Position: NewFilePosition(nodeLines(node)),
		Value:    node.Value,
	}
}

func newYamlNodeWithParent(parent, node *yaml.Node) *YamlNode {
	return &YamlNode{
		Position: NewFilePosition(nodeLines(node)),
		Value:    node.Value,
	}
}

func newYamlKeyValue(key, val *yaml.Node) *YamlKeyValue {
	return &YamlKeyValue{
		Key:   newYamlNode(key),
		Value: newYamlNodeWithParent(key, val),
	}
}

type YamlKeyValue struct {
	Key   *YamlNode
	Value *YamlNode
}

func (ykv YamlKeyValue) Lines() (lines []int) {
	lines = appendLine(lines, ykv.Key.Position.Lines...)
	lines = appendLine(lines, ykv.Value.Position.Lines...)
	return
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
	return
}

func (ym YamlMap) GetValue(key string) *YamlNode {
	for _, child := range ym.Items {
		if child.Key.Value == key {
			return child.Value
		}
	}
	return nil
}

func newYamlMap(key, value *yaml.Node) *YamlMap {
	ym := YamlMap{
		Key: newYamlNode(key),
	}

	var ckey *yaml.Node
	for _, child := range value.Content {
		if ckey != nil {
			kv := YamlKeyValue{
				Key:   newYamlNode(ckey),
				Value: newYamlNode(child),
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
	return
}

func newPromQLExpr(key, val *yaml.Node) *PromQLExpr {
	expr := PromQLExpr{
		Key:   newYamlNode(key),
		Value: newYamlNodeWithParent(key, val),
	}

	qlNode, err := decodeExpr(val.Value)
	if err != nil {
		expr.SyntaxError = err
		return &expr

	}
	expr.Query = qlNode
	return &expr
}

type AlertingRule struct {
	Alert       YamlKeyValue
	Expr        PromQLExpr
	For         *YamlKeyValue
	Labels      *YamlMap
	Annotations *YamlMap
}

func (ar AlertingRule) Lines() (lines []int) {
	lines = appendLine(lines, ar.Alert.Lines()...)
	lines = appendLine(lines, ar.Expr.Lines()...)
	if ar.For != nil {
		lines = appendLine(lines, ar.For.Lines()...)
	}
	if ar.Labels != nil {
		lines = appendLine(lines, ar.Labels.Lines()...)
	}
	if ar.Annotations != nil {
		lines = appendLine(lines, ar.Annotations.Lines()...)
	}
	return
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
	return
}

type ParseError struct {
	Fragment string
	Err      error
	Line     int
}

type Rule struct {
	AlertingRule  *AlertingRule
	RecordingRule *RecordingRule
	Error         ParseError
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
	return []int{r.Error.Line}
}

type Result struct {
	Path    string
	Error   error
	Content []byte
	Rules   []Rule
}
