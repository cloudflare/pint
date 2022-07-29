package parser

import (
	"bufio"
	"strings"

	"golang.org/x/exp/slices"
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
	return
}

type YamlNode struct {
	Position FilePosition
	Value    string
	Comments []string
}

func newYamlNode(node *yaml.Node, offset int) *YamlNode {
	return &YamlNode{
		Position: NewFilePosition(nodeLines(node, offset)),
		Value:    node.Value,
		Comments: mergeComments(node),
	}
}

func newYamlNodeWithParent(parent, node *yaml.Node, offset int) *YamlNode {
	return &YamlNode{
		Position: NewFilePosition(nodeLines(node, offset)),
		Value:    node.Value,
		Comments: mergeComments(node),
	}
}

func newYamlKeyValue(key, val *yaml.Node, offset int) *YamlKeyValue {
	return &YamlKeyValue{
		Key:   newYamlNode(key, offset),
		Value: newYamlNodeWithParent(key, val, offset),
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
	return
}

func newPromQLExpr(key, val *yaml.Node, offset int) *PromQLExpr {
	expr := PromQLExpr{
		Key:   newYamlNode(key, offset),
		Value: newYamlNodeWithParent(key, val, offset),
	}

	qlNode, err := DecodeExpr(val.Value)
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
	slices.Sort(lines)
	return
}

func (ar AlertingRule) Comments() (comments []string) {
	comments = append(comments, ar.Alert.Key.Comments...)
	comments = append(comments, ar.Alert.Value.Comments...)
	comments = append(comments, ar.Expr.Key.Comments...)
	comments = append(comments, ar.Expr.Value.Comments...)
	if ar.For != nil {
		comments = append(comments, ar.For.Key.Comments...)
		comments = append(comments, ar.For.Value.Comments...)
	}
	if ar.Labels != nil {
		comments = append(comments, ar.Labels.Key.Comments...)
		for _, label := range ar.Labels.Items {
			comments = append(comments, label.Key.Comments...)
			comments = append(comments, label.Value.Comments...)
		}
	}
	if ar.Annotations != nil {
		comments = append(comments, ar.Annotations.Key.Comments...)
		for _, annotation := range ar.Annotations.Items {
			comments = append(comments, annotation.Key.Comments...)
			comments = append(comments, annotation.Value.Comments...)
		}
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
	slices.Sort(lines)
	return
}

func (rr RecordingRule) Comments() (comments []string) {
	comments = append(comments, rr.Record.Key.Comments...)
	comments = append(comments, rr.Record.Value.Comments...)
	comments = append(comments, rr.Expr.Key.Comments...)
	comments = append(comments, rr.Expr.Value.Comments...)
	if rr.Labels != nil {
		comments = append(comments, rr.Labels.Key.Comments...)
		for _, label := range rr.Labels.Items {
			comments = append(comments, label.Key.Comments...)
			comments = append(comments, label.Value.Comments...)
		}
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

func (r Rule) HasComment(comment string) bool {
	var comments []string
	if r.RecordingRule != nil {
		comments = r.RecordingRule.Comments()
	} else if r.AlertingRule != nil {
		comments = r.AlertingRule.Comments()
	}
	for _, c := range comments {
		if hasComment(c, comment) {
			return true
		}
	}
	return false
}

func (r Rule) GetComment(comment ...string) (s Comment, ok bool) {
	var comments []string
	if r.RecordingRule != nil {
		comments = r.RecordingRule.Comments()
	} else if r.AlertingRule != nil {
		comments = r.AlertingRule.Comments()
	}
	for _, c := range comments {
		if val, ok := GetComment(c, comment...); ok {
			return val, ok
		}
	}
	return
}

func (r Rule) GetComments(key string) (cs []Comment) {
	var comments []string
	if r.RecordingRule != nil {
		comments = r.RecordingRule.Comments()
	} else if r.AlertingRule != nil {
		comments = r.AlertingRule.Comments()
	}
	for _, c := range comments {
		sc := bufio.NewScanner(strings.NewReader(c))
		for sc.Scan() {
			if val, ok := GetComment(sc.Text(), key); ok {
				cs = append(cs, val)
			}
		}
	}
	return cs
}

type Result struct {
	Path    string
	Error   error
	Content []byte
	Rules   []Rule
}
