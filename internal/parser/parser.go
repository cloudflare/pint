package parser

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	recordKey      = "record"
	exprKey        = "expr"
	labelsKey      = "labels"
	alertKey       = "alert"
	forKey         = "for"
	annotationsKey = "annotations"
)

func NewParser() Parser {
	return Parser{}
}

type Parser struct{}

func (p Parser) Parse(content []byte) (rules []Rule, err error) {
	if len(content) == 0 {
		return
	}

	var node yaml.Node
	err = yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, err
	}

	return parseNode(content, &node)
}

func parseNode(content []byte, node *yaml.Node) (rules []Rule, err error) {
	ret, isEmpty, err := parseRule(content, node)
	if err != nil {
		return nil, err
	}
	if !isEmpty {
		rules = append(rules, ret)
		return
	}

	for _, root := range node.Content {
		switch root.Kind {
		case yaml.SequenceNode:
			for _, n := range root.Content {
				ret, err := parseNode(content, n)
				if err != nil {
					return nil, err
				}
				rules = append(rules, ret...)
			}
		case yaml.MappingNode:
			rule, isEmpty, err := parseRule(content, root)
			if err != nil {
				return nil, err
			}
			if !isEmpty {
				rules = append(rules, rule)
			} else {
				for _, n := range root.Content {
					ret, err := parseNode(content, n)
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

func parseRule(content []byte, node *yaml.Node) (rule Rule, isEmpty bool, err error) {
	isEmpty = true

	if node.Kind != yaml.MappingNode {
		return
	}

	var recordPart *YamlKeyValue
	var exprPart *PromQLExpr
	var labelsPart *YamlMap

	var alertPart *YamlKeyValue
	var forPart *YamlKeyValue
	var annotationsPart *YamlMap

	var key *yaml.Node
	unknownKeys := []*yaml.Node{}
	for i, part := range node.Content {
		if i%2 == 0 {
			key = part
		} else {
			switch key.Value {
			case recordKey:
				recordPart = newYamlKeyValue(key, part)
			case alertKey:
				alertPart = newYamlKeyValue(key, part)
			case exprKey:
				exprPart = newPromQLExpr(key, part)
			case forKey:
				forPart = newYamlKeyValue(key, part)
			case labelsKey:
				labelsPart = newYamlMap(key, part)
			case annotationsKey:
				annotationsPart = newYamlMap(key, part)
			default:
				unknownKeys = append(unknownKeys, key)
			}
		}
	}

	if exprPart != nil && exprPart.Key.Position.FistLine() != exprPart.Value.Position.FistLine() {
		for {
			start := exprPart.Value.Position.FistLine() - 1
			end := exprPart.Value.Position.LastLine()
			input := strings.Join(strings.Split(string(content), "\n")[start:end], "")
			input = strings.ReplaceAll(input, " ", "")
			output := strings.ReplaceAll(exprPart.Value.Value, "\n", "")
			output = strings.ReplaceAll(output, " ", "")
			if end >= len(strings.Split(string(content), "\n")) {
				break
			}
			if input == output {
				break
			}
			exprPart.Value.Position.Lines = append(exprPart.Value.Position.Lines, end+1)
		}
	}

	if recordPart != nil && alertPart != nil {
		isEmpty = false
		rule = Rule{
			Error: ParseError{
				Line: node.Line,
				Err:  fmt.Errorf("got both %s and %s keys in a single rule", recordKey, alertKey),
			},
		}
		return
	}
	if recordPart != nil && exprPart == nil {
		isEmpty = false
		rule = Rule{
			Error: ParseError{
				Line: recordPart.Key.Position.LastLine(),
				Err:  fmt.Errorf("missing %s key", exprKey),
			},
		}
		return
	}
	if alertPart != nil && exprPart == nil {
		isEmpty = false
		rule = Rule{
			Error: ParseError{
				Line: alertPart.Key.Position.LastLine(),
				Err:  fmt.Errorf("missing %s key", exprKey),
			},
		}
		return
	}
	if exprPart != nil && alertPart == nil && recordPart == nil {
		isEmpty = false
		rule = Rule{
			Error: ParseError{
				Line: exprPart.Key.Position.LastLine(),
				Err:  fmt.Errorf("incomplete rule, no %s or %s key", alertKey, recordKey),
			},
		}
		return
	}
	if (recordPart != nil || alertPart != nil) && len(unknownKeys) > 0 {
		isEmpty = false
		var keys []string
		for _, n := range unknownKeys {
			keys = append(keys, n.Value)
		}
		rule = Rule{
			Error: ParseError{
				Line: unknownKeys[0].Line,
				Err:  fmt.Errorf("invalid key(s) found: %s", strings.Join(keys, ", ")),
			},
		}
		return
	}

	if recordPart != nil && exprPart != nil {
		isEmpty = false
		rule = Rule{RecordingRule: &RecordingRule{
			Record: *recordPart,
			Expr:   *exprPart,
			Labels: labelsPart,
		}}
		return
	}

	if alertPart != nil && exprPart != nil {
		isEmpty = false
		rule = Rule{AlertingRule: &AlertingRule{
			Alert:       *alertPart,
			Expr:        *exprPart,
			For:         forPart,
			Labels:      labelsPart,
			Annotations: annotationsPart,
		}}
		return
	}

	return
}
