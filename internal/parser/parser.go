package parser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/comments"
	"github.com/cloudflare/pint/internal/diags"
)

const (
	recordKey        = "record"
	exprKey          = "expr"
	labelsKey        = "labels"
	alertKey         = "alert"
	forKey           = "for"
	keepFiringForKey = "keep_firing_for"
	annotationsKey   = "annotations"
)

var ErrRuleCommentOnFile = errors.New("this comment is only valid when attached to a rule")

type Schema int

const (
	PrometheusSchema Schema = iota
	ThanosSchema
)

func NewParser(isStrict bool, schema Schema, names model.ValidationScheme) Parser {
	model.NameValidationScheme = names
	return Parser{
		isStrict: isStrict,
		schema:   schema,
	}
}

type Parser struct {
	schema   Schema
	isStrict bool
}

func (p Parser) Parse(content []byte) (rules []Rule, err error) {
	if len(content) == 0 {
		return nil, nil
	}

	contentLines := strings.Split(string(content), "\n")

	dec := yaml.NewDecoder(bytes.NewReader(content))
	var index int
	for {
		var doc yaml.Node
		decodeErr := dec.Decode(&doc)
		if errors.Is(decodeErr, io.EOF) {
			break
		}
		if decodeErr != nil {
			return nil, tryDecodingYamlError(decodeErr)
		}
		index++
		if p.isStrict {
			r, err := parseGroups(contentLines, &doc, p.schema)
			if err.Err != nil {
				return rules, err
			}
			rules = append(rules, r...)
		} else {
			rules = append(rules, parseNode(content, contentLines, &doc, 0, 0, p.schema)...)
		}
		if index > 1 && p.isStrict {
			rules = append(rules, Rule{
				Lines: diags.LineRange{First: doc.Line, Last: doc.Line},
				Error: ParseError{
					Err: errors.New("multi-document YAML files are not allowed"),
					Details: `This is a multi-document YAML file. Prometheus will only parse the first document and silently ignore the rest.
To allow for multi-document YAML files set parser->relaxed option in pint config file.`,
					Line: doc.Line,
				},
			})
		}
	}

	return rules, err
}

func parseNode(content []byte, contentLines []string, node *yaml.Node, offsetLine, offsetColumn int, schema Schema) (rules []Rule) {
	ret, isEmpty := parseRule(contentLines, node, offsetLine, offsetColumn)
	if !isEmpty {
		rules = append(rules, ret)
		return rules
	}

	var rule Rule
	for _, root := range node.Content {
		// nolint: exhaustive
		switch root.Kind {
		case yaml.SequenceNode:
			for _, n := range root.Content {
				rules = append(rules, parseNode(content, contentLines, n, offsetLine, offsetColumn, schema)...)
			}
		case yaml.MappingNode:
			rule, isEmpty = parseRule(contentLines, root, offsetLine, offsetColumn)
			if !isEmpty {
				rules = append(rules, rule)
			} else {
				for _, n := range root.Content {
					rules = append(rules, parseNode(content, contentLines, n, offsetLine, offsetColumn, schema)...)
				}
			}
		case yaml.ScalarNode:
			if root.Value != string(content) {
				c := []byte(root.Value)
				var n yaml.Node
				// FIXME there must be a better way.
				// If we have YAML inside YAML:
				// alerts: |
				//   groups:
				//     rules: ...
				// Then we need to get the offset of `groups` inside the FILE, not inside the YAML node value.
				// Right now we read the line where it's in the file and count leading spaces.
				if err := yaml.Unmarshal(c, &n); err == nil {
					rules = append(rules,
						parseNode(
							c,
							strings.Split(root.Value, "\n"),
							&n,
							offsetLine+root.Line,
							offsetColumn+countLeadingSpace(contentLines[root.Line]),
							schema)...,
					)
				}
			}
		}
	}
	return rules
}

func parseRule(contentLines []string, node *yaml.Node, offsetLine, offsetColumn int) (rule Rule, _ bool) {
	if node.Kind != yaml.MappingNode {
		return rule, true
	}

	var recordPart *YamlNode
	var exprPart *PromQLExpr
	var labelsPart *YamlMap

	var alertPart *YamlNode
	var forPart *YamlNode
	var keepFiringForPart *YamlNode
	var annotationsPart *YamlMap

	var recordNode *yaml.Node
	var alertNode *yaml.Node
	var exprNode *yaml.Node
	var forNode *yaml.Node
	var keepFiringForNode *yaml.Node
	var labelsNode *yaml.Node
	var annotationsNode *yaml.Node

	labelsNodes := []yamlMap{}
	annotationsNodes := []yamlMap{}

	var key *yaml.Node
	unknownKeys := []*yaml.Node{}

	var lines diags.LineRange

	var ruleComments []comments.Comment

	for i, part := range unpackNodes(node) {
		if lines.First == 0 || part.Line+offsetLine < lines.First {
			lines.First = part.Line + offsetLine
		}
		lines.Last = max(lines.Last, part.Line+offsetLine)

		if i == 0 && node.HeadComment != "" && part.HeadComment == "" {
			part.HeadComment = node.HeadComment
		}
		if i == 0 && node.LineComment != "" && part.LineComment == "" {
			part.LineComment = node.LineComment
		}
		if i == len(node.Content)-1 && node.FootComment != "" && part.HeadComment == "" {
			part.FootComment = node.FootComment
		}
		for _, s := range mergeComments(part) {
			for _, c := range comments.Parse(part.Line, s) {
				if comments.IsRuleComment(c.Type) {
					ruleComments = append(ruleComments, c)
				}
			}
		}

		if i%2 == 0 {
			key = part
		} else {
			switch key.Value {
			case recordKey:
				if recordPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, recordKey)
				}
				recordNode = part
				recordPart = newYamlNode(part, offsetLine, offsetColumn, contentLines, key.Column+2)
				lines.Last = max(lines.Last, recordPart.Lines.Last)
			case alertKey:
				if alertPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, alertKey)
				}
				alertNode = part
				alertPart = newYamlNode(part, offsetLine, offsetColumn, contentLines, key.Column+2)
				lines.Last = max(lines.Last, alertPart.Lines.Last)
			case exprKey:
				if exprPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, exprKey)
				}
				exprNode = part
				exprPart = newPromQLExpr(part, offsetLine, offsetColumn, contentLines, key.Column+2)
				exprPart.Value.Lines = exprPart.Value.Pos.Lines()
				lines.Last = max(lines.Last, exprPart.Value.Lines.Last)
			case forKey:
				if forPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, forKey)
				}
				forNode = part
				forPart = newYamlNode(part, offsetLine, offsetColumn, contentLines, key.Column+2)
				lines.Last = max(lines.Last, forPart.Lines.Last)
			case keepFiringForKey:
				if keepFiringForPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, keepFiringForKey)
				}
				keepFiringForNode = part
				keepFiringForPart = newYamlNode(part, offsetLine, offsetColumn, contentLines, key.Column+2)
				lines.Last = max(lines.Last, keepFiringForPart.Lines.Last)
			case labelsKey:
				if labelsPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, labelsKey)
				}
				labelsNode = part
				labelsNodes = mappingNodes(part)
				labelsPart = newYamlMap(key, part, offsetLine, offsetColumn, contentLines)
				lines.Last = max(lines.Last, labelsPart.Lines.Last)
			case annotationsKey:
				if annotationsPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, annotationsKey)
				}
				annotationsNode = part
				annotationsNodes = mappingNodes(part)
				annotationsPart = newYamlMap(key, part, offsetLine, offsetColumn, contentLines)
				lines.Last = max(lines.Last, annotationsPart.Lines.Last)
			default:
				unknownKeys = append(unknownKeys, key)
			}
		}
	}

	if recordPart != nil && alertPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: node.Line + offsetLine,
				Err:  fmt.Errorf("got both %s and %s keys in a single rule", recordKey, alertKey),
			},
		}
		return rule, false
	}
	if exprPart != nil && alertPart == nil && recordPart == nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: exprPart.Value.Lines.Last,
				Err:  fmt.Errorf("incomplete rule, no %s or %s key", alertKey, recordKey),
			},
		}
		return rule, false
	}
	if recordPart != nil && forPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: forPart.Lines.First,
				Err:  fmt.Errorf("invalid field '%s' in recording rule", forKey),
			},
		}
		return rule, false
	}
	if recordPart != nil && keepFiringForPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: keepFiringForPart.Lines.First,
				Err:  fmt.Errorf("invalid field '%s' in recording rule", keepFiringForKey),
			},
		}
		return rule, false
	}
	if recordPart != nil && annotationsPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: annotationsPart.Lines.First,
				Err:  fmt.Errorf("invalid field '%s' in recording rule", annotationsKey),
			},
		}
		return rule, false
	}
	for _, entry := range []struct {
		part *yaml.Node
		key  string
	}{
		{key: recordKey, part: recordNode},
		{key: alertKey, part: alertNode},
		{key: exprKey, part: exprNode},
		{key: forKey, part: forNode},
		{key: keepFiringForKey, part: keepFiringForNode},
	} {
		if entry.part != nil && !isTag(entry.part.ShortTag(), strTag) {
			return invalidValueError(lines, entry.part.Line+offsetLine, entry.key, describeTag(strTag), describeTag(entry.part.ShortTag()))
		}
	}

	for _, entry := range []struct {
		part *yaml.Node
		key  string
	}{
		{key: labelsKey, part: labelsNode},
		{key: annotationsKey, part: annotationsNode},
	} {
		if entry.part != nil && !isTag(entry.part.ShortTag(), mapTag) {
			return invalidValueError(lines, entry.part.Line+offsetLine, entry.key, describeTag(mapTag), describeTag(entry.part.ShortTag()))
		}
	}

	for _, elem := range []struct {
		key   string
		parts []yamlMap
	}{
		{key: labelsKey, parts: labelsNodes},
		{key: annotationsKey, parts: annotationsNodes},
	} {
		names := map[string]struct{}{}
		for _, entry := range elem.parts {
			if !isTag(entry.val.ShortTag(), strTag) {
				return invalidValueError(lines, entry.val.Line+offsetLine, fmt.Sprintf("%s %s", elem.key, nodeValue(entry.key)), describeTag(strTag), describeTag(entry.val.ShortTag()))
			}
			if _, ok := names[entry.key.Value]; ok {
				return Rule{
					Lines: rangeFromYamlMaps(elem.parts),
					Error: ParseError{
						Line: entry.key.Line,
						Err:  fmt.Errorf("duplicated %s key %s", elem.key, entry.key.Value),
					},
				}, false
			}
			names[entry.key.Value] = struct{}{}
		}
	}

	if r, ok := ensureRequiredKeys(lines, recordKey, recordPart, exprPart); !ok {
		return r, false
	}
	if r, ok := ensureRequiredKeys(lines, alertKey, alertPart, exprPart); !ok {
		return r, false
	}
	if (recordPart != nil || alertPart != nil) && len(unknownKeys) > 0 {
		var keys []string
		for _, n := range unknownKeys {
			keys = append(keys, n.Value)
		}
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: unknownKeys[0].Line + offsetLine,
				Err:  fmt.Errorf("invalid key(s) found: %s", strings.Join(keys, ", ")),
			},
		}
		return rule, false
	}

	if recordPart != nil && !model.IsValidMetricName(model.LabelValue(recordPart.Value)) {
		return Rule{
			Lines: lines,
			Error: ParseError{
				Line: recordPart.Lines.First,
				Err:  fmt.Errorf("invalid recording rule name: %s", recordPart.Value),
			},
		}, false
	}

	if (recordPart != nil || alertPart != nil) && labelsPart != nil {
		for _, lab := range labelsPart.Items {
			if !model.LabelName(lab.Key.Value).IsValid() || lab.Key.Value == model.MetricNameLabel {
				return Rule{
					Lines: lines,
					Error: ParseError{
						Line: lab.Key.Lines.First,
						Err:  fmt.Errorf("invalid label name: %s", lab.Key.Value),
					},
				}, false
			}
			if !model.LabelValue(lab.Value.Value).IsValid() {
				return Rule{
					Lines: lines,
					Error: ParseError{
						Line: lab.Key.Lines.First,
						Err:  fmt.Errorf("invalid label value: %s", lab.Value.Value),
					},
				}, false
			}
		}
	}

	if alertPart != nil && annotationsPart != nil {
		for _, ann := range annotationsPart.Items {
			if !model.LabelName(ann.Key.Value).IsValid() {
				return Rule{
					Lines: lines,
					Error: ParseError{
						Line: ann.Key.Lines.First,
						Err:  fmt.Errorf("invalid annotation name: %s", ann.Key.Value),
					},
				}, false
			}
		}
	}

	if recordPart != nil && exprPart != nil {
		rule = Rule{
			Lines: lines,
			RecordingRule: &RecordingRule{
				Record: *recordPart,
				Expr:   *exprPart,
				Labels: labelsPart,
			},
			Comments: ruleComments,
		}
		return rule, false
	}

	if alertPart != nil && exprPart != nil {
		rule = Rule{
			Lines: lines,
			AlertingRule: &AlertingRule{
				Alert:         *alertPart,
				Expr:          *exprPart,
				For:           forPart,
				KeepFiringFor: keepFiringForPart,
				Labels:        labelsPart,
				Annotations:   annotationsPart,
			},
			Comments: ruleComments,
		}
		return rule, false
	}

	return rule, true
}

func unpackNodes(node *yaml.Node) []*yaml.Node {
	nodes := make([]*yaml.Node, 0, len(node.Content))
	var isMerge bool
	for _, part := range node.Content {
		if part.ShortTag() == mergeTag && part.Value == "<<" {
			isMerge = true
		}

		if part.Alias != nil {
			if isMerge {
				nodes = append(nodes, resolveMapAlias(part, node).Content...)
			} else {
				nodes = append(nodes, resolveMapAlias(part, part))
			}
			isMerge = false
			continue
		}
		if isMerge {
			continue
		}
		nodes = append(nodes, part)
	}
	return nodes
}

func nodeKeys(node *yaml.Node) (keys []string) {
	if node.Kind != yaml.MappingNode {
		return keys
	}
	for i, n := range node.Content {
		if i%2 == 0 && n.Value != "" {
			keys = append(keys, n.Value)
		}
	}
	return keys
}

func hasKey(node *yaml.Node, key string) bool {
	for _, k := range nodeKeys(node) {
		if k == key {
			return true
		}
	}
	return false
}

func hasValue(node *YamlNode) bool {
	if node == nil {
		return false
	}
	return node.Value != ""
}

func ensureRequiredKeys(lines diags.LineRange, key string, keyVal *YamlNode, expr *PromQLExpr) (Rule, bool) {
	if keyVal == nil {
		return Rule{Lines: lines}, true
	}
	if !hasValue(keyVal) {
		return Rule{
			Lines: lines,
			Error: ParseError{
				Line: keyVal.Lines.Last,
				Err:  fmt.Errorf("%s value cannot be empty", key),
			},
		}, false
	}
	if expr == nil {
		return Rule{
			Lines: lines,
			Error: ParseError{
				Line: keyVal.Lines.Last,
				Err:  fmt.Errorf("missing %s key", exprKey),
			},
		}, false
	}
	if !hasValue(expr.Value) {
		return Rule{
			Lines: lines,
			Error: ParseError{
				Line: expr.Value.Lines.Last,
				Err:  fmt.Errorf("%s value cannot be empty", exprKey),
			},
		}, false
	}
	return Rule{Lines: lines}, true
}

func resolveMapAlias(part, parent *yaml.Node) *yaml.Node {
	node := *part
	node.Content = nil
	var ok bool
	for i, alias := range part.Alias.Content {
		if i%2 == 0 {
			ok = !hasKey(parent, alias.Value)
		}
		if ok {
			node.Content = append(node.Content, alias)
		}
		if i%2 == 1 {
			ok = false
		}
	}
	return &node
}

func duplicatedKeyError(lines diags.LineRange, line int, key string) (Rule, bool) {
	rule := Rule{
		Lines: lines,
		Error: ParseError{
			Line: line,
			Err:  fmt.Errorf("duplicated %s key", key),
		},
	}
	return rule, false
}

func invalidValueError(lines diags.LineRange, line int, key, expectedTag, gotTag string) (Rule, bool) {
	rule := Rule{
		Lines: lines,
		Error: ParseError{
			Line: line,
			Err:  fmt.Errorf("%s value must be a %s, got %s instead", key, expectedTag, gotTag),
		},
	}
	return rule, false
}

func isTag(tag, expected string) bool {
	if tag == nullTag {
		return true
	}
	return tag == expected
}

type yamlMap struct {
	key *yaml.Node
	val *yaml.Node
}

func mappingNodes(node *yaml.Node) []yamlMap {
	m := make([]yamlMap, 0, len(node.Content)/2)
	var key *yaml.Node
	for _, child := range node.Content {
		if key != nil {
			m = append(m, yamlMap{key: key, val: child})
			key = nil
		} else {
			key = child
		}
	}
	return m
}

func rangeFromYamlMaps(m []yamlMap) (lr diags.LineRange) {
	for _, entry := range m {
		if lr.First == 0 {
			lr.First = entry.key.Line
			lr.Last = entry.val.Line
		}
		lr.First = min(lr.First, entry.key.Line, entry.val.Line)
		lr.Last = max(lr.Last, entry.key.Line, entry.val.Line)
	}
	return lr
}

var (
	yamlErrRe          = regexp.MustCompile("^yaml: line (.+): (.+)")
	yamlUnmarshalErrRe = regexp.MustCompile("^yaml: unmarshal errors:\n  line (.+): (.+)")
)

func tryDecodingYamlError(err error) ParseError {
	for _, re := range []*regexp.Regexp{yamlErrRe, yamlUnmarshalErrRe} {
		parts := re.FindStringSubmatch(err.Error())
		if len(parts) > 2 {
			if line, err2 := strconv.Atoi(parts[1]); line > 0 && err2 == nil {
				return ParseError{
					Line: line,
					Err:  errors.New(parts[2]),
				}
			}
		}
	}
	return ParseError{Line: 1, Err: err}
}

func countLeadingSpace(line string) (i int) {
	for _, r := range line {
		if r != ' ' {
			return i
		}
		i++
	}
	return i
}
