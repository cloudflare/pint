package parser

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/comments"
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

func NewParser() Parser {
	return Parser{}
}

type Parser struct{}

func (p Parser) Parse(content []byte) (rules []Rule, err error) {
	if len(content) == 0 {
		return nil, nil
	}

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("unable to parse YAML file: %s", r)
		}
	}()

	var node yaml.Node
	err = yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, err
	}

	rules, err = parseNode(content, &node, 0)
	return rules, err
}

func parseNode(content []byte, node *yaml.Node, offset int) (rules []Rule, err error) {
	ret, isEmpty, err := parseRule(content, node, offset)
	if err != nil {
		return nil, err
	}
	if !isEmpty {
		rules = append(rules, ret)
		return rules, nil
	}

	var rl []Rule
	var rule Rule
	for _, root := range node.Content {
		// nolint: exhaustive
		switch root.Kind {
		case yaml.SequenceNode:
			for _, n := range root.Content {
				rl, err = parseNode(content, n, offset)
				if err != nil {
					return nil, err
				}
				rules = append(rules, rl...)
			}
		case yaml.MappingNode:
			rule, isEmpty, err = parseRule(content, root, offset)
			if err != nil {
				return nil, err
			}
			if !isEmpty {
				rules = append(rules, rule)
			} else {
				for _, n := range root.Content {
					rl, err = parseNode(content, n, offset)
					if err != nil {
						return nil, err
					}
					rules = append(rules, rl...)
				}
			}
		case yaml.ScalarNode:
			if root.Value != string(content) {
				c := []byte(root.Value)
				var n yaml.Node
				err = yaml.Unmarshal(c, &n)
				if err == nil {
					ret, err := parseNode(c, &n, offset+root.Line)
					if err != nil {
						return nil, err
					}
					rules = append(rules, ret...)
				}
			}
		}
	}
	return rules, nil
}

func parseRule(content []byte, node *yaml.Node, offset int) (rule Rule, _ bool, err error) {
	if node.Kind != yaml.MappingNode {
		return rule, true, err
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
	labelsNodes := map[*yaml.Node]*yaml.Node{}
	annotationsNodes := map[*yaml.Node]*yaml.Node{}

	var key *yaml.Node
	unknownKeys := []*yaml.Node{}

	var lines LineRange

	var ruleComments []comments.Comment

	for i, part := range unpackNodes(node) {
		if lines.First == 0 || part.Line+offset < lines.First {
			lines.First = part.Line + offset
		}
		lines.Last = max(lines.Last, part.Line+offset)

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
					return duplicatedKeyError(lines, part.Line+offset, recordKey)
				}
				recordNode = part
				recordPart = newYamlNodeWithKey(key, part, offset)
				lines.Last = max(lines.Last, recordPart.Lines.Last)
			case alertKey:
				if alertPart != nil {
					return duplicatedKeyError(lines, part.Line+offset, alertKey)
				}
				alertNode = part
				alertPart = newYamlNodeWithKey(key, part, offset)
				lines.Last = max(lines.Last, alertPart.Lines.Last)
			case exprKey:
				if exprPart != nil {
					return duplicatedKeyError(lines, part.Line+offset, exprKey)
				}
				exprNode = part
				exprPart = newPromQLExpr(key, part, offset)
				lines.Last = max(lines.Last, exprPart.Value.Lines.Last)
			case forKey:
				if forPart != nil {
					return duplicatedKeyError(lines, part.Line+offset, forKey)
				}
				forNode = part
				forPart = newYamlNodeWithKey(key, part, offset)
				lines.Last = max(lines.Last, forPart.Lines.Last)
			case keepFiringForKey:
				if keepFiringForPart != nil {
					return duplicatedKeyError(lines, part.Line+offset, keepFiringForKey)
				}
				keepFiringForNode = part
				keepFiringForPart = newYamlNodeWithKey(key, part, offset)
				lines.Last = max(lines.Last, keepFiringForPart.Lines.Last)
			case labelsKey:
				if labelsPart != nil {
					return duplicatedKeyError(lines, part.Line+offset, labelsKey)
				}
				labelsNode = part
				labelsNodes = mappingNodes(part)
				labelsPart = newYamlMap(key, part, offset)
				lines.Last = max(lines.Last, labelsPart.Lines.Last)
			case annotationsKey:
				if annotationsPart != nil {
					return duplicatedKeyError(lines, part.Line+offset, annotationsKey)
				}
				annotationsNode = part
				annotationsNodes = mappingNodes(part)
				annotationsPart = newYamlMap(key, part, offset)
				lines.Last = max(lines.Last, annotationsPart.Lines.Last)
			default:
				unknownKeys = append(unknownKeys, key)
			}
		}
	}

	if exprPart != nil && exprPart.Value.Lines.First != exprPart.Value.Lines.Last {
		contentLines := strings.Split(string(content), "\n")
		for {
			start := exprPart.Value.Lines.First
			end := exprPart.Value.Lines.Last
			if end > len(contentLines) {
				end--
			}
			input := strings.Join(contentLines[start:end], "")
			input = strings.ReplaceAll(input, " ", "")
			output := strings.ReplaceAll(exprPart.Value.Value, "\n", "")
			output = strings.ReplaceAll(output, " ", "")
			if end >= len(contentLines) {
				break
			}
			if input == output {
				break
			}
			exprPart.Value.Lines.Last = end + 1
		}
	}

	if recordPart != nil && alertPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: node.Line + offset,
				Err:  fmt.Errorf("got both %s and %s keys in a single rule", recordKey, alertKey),
			},
		}
		return rule, false, err
	}
	if exprPart != nil && alertPart == nil && recordPart == nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: exprPart.Value.Lines.Last,
				Err:  fmt.Errorf("incomplete rule, no %s or %s key", alertKey, recordKey),
			},
		}
		return rule, false, err
	}
	if recordPart != nil && forPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: forPart.Lines.First,
				Err:  fmt.Errorf("invalid field '%s' in recording rule", forKey),
			},
		}
		return rule, false, err
	}
	if recordPart != nil && keepFiringForPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: keepFiringForPart.Lines.First,
				Err:  fmt.Errorf("invalid field '%s' in recording rule", keepFiringForKey),
			},
		}
		return rule, false, err
	}
	if recordPart != nil && annotationsPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: annotationsPart.Lines.First,
				Err:  fmt.Errorf("invalid field '%s' in recording rule", annotationsKey),
			},
		}
		return rule, false, err
	}
	if r, ok := ensureRequiredKeys(lines, recordKey, recordPart, exprPart); !ok {
		return r, false, err
	}
	if r, ok := ensureRequiredKeys(lines, alertKey, alertPart, exprPart); !ok {
		return r, false, err
	}
	if (recordPart != nil || alertPart != nil) && len(unknownKeys) > 0 {
		var keys []string
		for _, n := range unknownKeys {
			keys = append(keys, n.Value)
		}
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: unknownKeys[0].Line + offset,
				Err:  fmt.Errorf("invalid key(s) found: %s", strings.Join(keys, ", ")),
			},
		}
		return rule, false, err
	}

	if recordPart != nil && !model.IsValidMetricName(model.LabelValue(recordPart.Value)) {
		return Rule{
			Lines: lines,
			Error: ParseError{
				Line: recordPart.Lines.First,
				Err:  fmt.Errorf("invalid recording rule name: %s", recordPart.Value),
			},
		}, false, err
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
				}, false, err
			}
			if !model.LabelValue(lab.Value.Value).IsValid() {
				return Rule{
					Lines: lines,
					Error: ParseError{
						Line: lab.Key.Lines.First,
						Err:  fmt.Errorf("invalid label value: %s", lab.Value.Value),
					},
				}, false, err
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
				}, false, err
			}
		}
	}

	for key, part := range map[string]*yaml.Node{
		recordKey:        recordNode,
		alertKey:         alertNode,
		exprKey:          exprNode,
		forKey:           forNode,
		keepFiringForKey: keepFiringForNode,
	} {
		if part != nil && !isTag(part.ShortTag(), "!!str") {
			return invalidValueError(lines, part.Line+offset, key, "string", describeTag(part.ShortTag()))
		}
	}
	for key, part := range map[string]*yaml.Node{
		labelsKey:      labelsNode,
		annotationsKey: annotationsNode,
	} {
		if part != nil && !isTag(part.ShortTag(), "!!map") {
			return invalidValueError(lines, part.Line+offset, key, "mapping", describeTag(part.ShortTag()))
		}
	}

	for section, parts := range map[string]map[*yaml.Node]*yaml.Node{
		labelsKey:      labelsNodes,
		annotationsKey: annotationsNodes,
	} {
		for key, value := range parts {
			if !isTag(value.ShortTag(), "!!str") {
				return invalidValueError(lines, value.Line+offset, fmt.Sprintf("%s %s", section, nodeValue(key)), "string", describeTag(value.ShortTag()))
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
		return rule, false, err
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
		return rule, false, err
	}

	return rule, true, err
}

func unpackNodes(node *yaml.Node) []*yaml.Node {
	nodes := make([]*yaml.Node, 0, len(node.Content))
	var isMerge bool
	for _, part := range node.Content {
		if part.ShortTag() == "!!merge" && part.Value == "<<" {
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

func ensureRequiredKeys(lines LineRange, key string, keyVal *YamlNode, expr *PromQLExpr) (Rule, bool) {
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

func duplicatedKeyError(lines LineRange, line int, key string) (Rule, bool, error) {
	rule := Rule{
		Lines: lines,
		Error: ParseError{
			Line: line,
			Err:  fmt.Errorf("duplicated %s key", key),
		},
	}
	return rule, false, nil
}

func invalidValueError(lines LineRange, line int, key, expectedKind, gotKind string) (Rule, bool, error) {
	rule := Rule{
		Lines: lines,
		Error: ParseError{
			Line: line,
			Err:  fmt.Errorf("%s value must be a YAML %s, got %s instead", key, expectedKind, gotKind),
		},
	}
	return rule, false, nil
}

func isTag(tag, expected string) bool {
	if tag == "!!null" {
		return true
	}
	return tag == expected
}

func describeTag(tag string) string {
	switch tag {
	case "":
		return "unknown type"
	case "!!str":
		return "string"
	case "!!int":
		return "integer"
	case "!!seq":
		return "list"
	case "!!map":
		return "mapping"
	case "!!binary":
		return "binary data"
	default:
		return strings.TrimLeft(tag, "!")
	}
}

func mappingNodes(node *yaml.Node) map[*yaml.Node]*yaml.Node {
	m := make(map[*yaml.Node]*yaml.Node, len(node.Content))
	var key *yaml.Node
	for _, child := range node.Content {
		if key != nil {
			m[key] = child
			key = nil
		} else {
			key = child
		}
	}
	return m
}
