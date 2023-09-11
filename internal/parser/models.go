package parser

import (
	"bufio"
	"fmt"
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
	return comments
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

func newPromQLExpr(key, val *yaml.Node, offset int) *PromQLExpr {
	expr := PromQLExpr{
		Key:   newYamlNode(key, offset),
		Value: newYamlNode(val, offset),
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

func (ar AlertingRule) Comments() (comments []string) {
	comments = append(comments, ar.Alert.Key.Comments...)
	comments = append(comments, ar.Alert.Value.Comments...)
	comments = append(comments, ar.Expr.Key.Comments...)
	comments = append(comments, ar.Expr.Value.Comments...)
	if ar.For != nil {
		comments = append(comments, ar.For.Key.Comments...)
		comments = append(comments, ar.For.Value.Comments...)
	}
	if ar.KeepFiringFor != nil {
		comments = append(comments, ar.KeepFiringFor.Key.Comments...)
		comments = append(comments, ar.KeepFiringFor.Value.Comments...)
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
	return comments
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
	return comments
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

func (r Rule) ToYAML() string {
	if r.Error.Err != nil {
		return fmt.Sprintf("line=%d fragment=%s err=%s", r.Error.Line, r.Error.Fragment, r.Error.Err)
	}

	if r.AlertingRule == nil && r.RecordingRule == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("- ")
	if r.AlertingRule != nil {
		b.WriteString("  ")
		b.WriteString(r.AlertingRule.Alert.Key.Value)
		b.WriteRune(':')
		b.WriteString(r.AlertingRule.Alert.Value.Value)
		b.WriteRune('\n')

		b.WriteString("  ")
		b.WriteString(r.AlertingRule.Expr.Key.Value)
		b.WriteRune(':')
		b.WriteString(r.AlertingRule.Expr.Value.Value)
		b.WriteRune('\n')

		if r.AlertingRule.For != nil {
			b.WriteString("  ")
			b.WriteString(r.AlertingRule.For.Key.Value)
			b.WriteRune(':')
			b.WriteString(r.AlertingRule.For.Value.Value)
			b.WriteRune('\n')
		}
		if r.AlertingRule.KeepFiringFor != nil {
			b.WriteString("  ")
			b.WriteString(r.AlertingRule.KeepFiringFor.Key.Value)
			b.WriteRune(':')
			b.WriteString(r.AlertingRule.KeepFiringFor.Value.Value)
			b.WriteRune('\n')
		}

		if r.AlertingRule.Annotations != nil {
			b.WriteString("  annotations:\n")
			for _, a := range r.AlertingRule.Annotations.Items {
				b.WriteString("    ")
				b.WriteString(a.Key.Value)
				b.WriteRune(':')
				b.WriteString(a.Value.Value)
				b.WriteRune('\n')
			}
		}

		if r.AlertingRule.Labels != nil {
			b.WriteString("  labels:\n")
			for _, l := range r.AlertingRule.Labels.Items {
				b.WriteString("    ")
				b.WriteString(l.Key.Value)
				b.WriteRune(':')
				b.WriteString(l.Value.Value)
				b.WriteRune('\n')
			}
		}

		return b.String()
	}

	b.WriteString(r.RecordingRule.Record.Key.Value)
	b.WriteRune(':')
	b.WriteString(r.RecordingRule.Record.Value.Value)

	b.WriteString("  ")
	b.WriteString(r.RecordingRule.Expr.Key.Value)
	b.WriteRune(':')
	b.WriteString(r.RecordingRule.Expr.Value.Value)
	b.WriteRune('\n')

	if r.RecordingRule.Labels != nil {
		b.WriteString("  labels:\n")
		for _, l := range r.RecordingRule.Labels.Items {
			b.WriteString("    ")
			b.WriteString(l.Key.Value)
			b.WriteRune(':')
			b.WriteString(l.Value.Value)
			b.WriteRune('\n')
		}
	}

	return b.String()
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
		var val Comment
		if val, ok = GetLastComment(c, comment...); ok {
			return val, ok
		}
	}
	return s, ok
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
			if val, ok := GetLastComment(sc.Text(), key); ok {
				cs = append(cs, val)
			}
		}
	}
	return cs
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
