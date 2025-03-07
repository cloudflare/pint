package parser

import (
	"errors"
	"fmt"
	"strings"

	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"
)

// https://github.com/go-yaml/yaml/blob/v3.0.1/resolve.go#L70-L81
const (
	nullTag      = "!!null"
	boolTag      = "!!bool"
	strTag       = "!!str"
	intTag       = "!!int"
	floatTag     = "!!float"
	timestampTag = "!!timestamp"
	seqTag       = "!!seq"
	mapTag       = "!!map"
	binaryTag    = "!!binary"
	mergeTag     = "!!merge"
)

func describeTag(tag string) string {
	switch tag {
	case strTag:
		return "string"
	case intTag:
		return "integer"
	case seqTag:
		return "list"
	case mapTag:
		return "mapping"
	case binaryTag:
		return "binary data"
	default:
		return strings.TrimLeft(tag, "!")
	}
}

func parseGroups(contentLines []string, doc *yaml.Node, schema Schema) (rules []Rule, err ParseError) {
	names := map[string]struct{}{}

	for _, node := range unpackNodes(doc) {
		if !isTag(node.ShortTag(), mapTag) {
			return nil, ParseError{
				Line: node.Line,
				Err:  fmt.Errorf("top level field must be a groups key, got %s", describeTag(node.ShortTag())),
			}
		}

		for _, entry := range mappingNodes(node) {
			if entry.key.ShortTag() != strTag {
				return nil, ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("groups key must be a %s, got a %s", describeTag(strTag), describeTag(entry.key.ShortTag())),
				}
			}
			if entry.key.Value != "groups" {
				return nil, ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("unexpected key %s", entry.key.Value),
				}
			}
			if !isTag(entry.val.ShortTag(), seqTag) {
				return nil, ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("groups value must be a %s, got %s", describeTag(seqTag), describeTag(entry.val.ShortTag())),
				}
			}
			for _, group := range unpackNodes(entry.val) {
				name, r, err := parseGroup(contentLines, group, schema)
				if err.Err != nil {
					return rules, err
				}
				if _, ok := names[name]; ok {
					return nil, ParseError{
						Line: group.Line,
						Err:  errors.New("duplicated group name"),
					}
				}
				names[name] = struct{}{}
				rules = append(rules, r...)
			}
		}
	}
	return rules, ParseError{}
}

func parseGroup(contentLines []string, group *yaml.Node, schema Schema) (name string, rules []Rule, err ParseError) {
	if !isTag(group.ShortTag(), mapTag) {
		return "", nil, ParseError{
			Line: group.Line,
			Err:  fmt.Errorf("group must be a %s, got %s", describeTag(mapTag), describeTag(group.ShortTag())),
		}
	}

	setKeys := make(map[string]struct{}, len(group.Content))

	for _, entry := range mappingNodes(group) {
		switch entry.key.Value {
		case "name":
			if entry.val.Kind != yaml.ScalarNode || entry.val.ShortTag() != strTag {
				return "", nil, ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("group name must be a %s, got %s", describeTag(strTag), describeTag(entry.val.ShortTag())),
				}
			}
			if entry.val.Value == "" {
				return "", nil, ParseError{
					Line: entry.key.Line,
					Err:  errors.New("group name cannot be empty"),
				}
			}
			name = entry.val.Value
		case "interval", "query_offset":
			if entry.val.Kind != yaml.ScalarNode || entry.val.ShortTag() != strTag {
				return "", nil, ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("group %s must be a %s, got %s", entry.key.Value, describeTag(strTag), describeTag(entry.val.ShortTag())),
				}
			}
			if _, err := model.ParseDuration(entry.val.Value); err != nil {
				return "", nil, ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("invalid %s value: %w", entry.key.Value, err),
				}
			}
		case "limit":
			if entry.val.Kind != yaml.ScalarNode || entry.val.ShortTag() != intTag {
				return "", nil, ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("group limit must be a %s, got %s", describeTag(intTag), describeTag(entry.val.ShortTag())),
				}
			}
		case "rules":
			if !isTag(entry.val.ShortTag(), seqTag) {
				return "", nil, ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("rules must be a %s, got %s", describeTag(seqTag), describeTag(entry.val.ShortTag())),
				}
			}
			for _, rule := range unpackNodes(entry.val) {
				r, err := parseRuleStrict(contentLines, rule)
				if err.Err != nil {
					return "", nil, err
				}
				rules = append(rules, r)
			}
		case "partial_response_strategy":
			if schema != ThanosSchema {
				return "", nil, ParseError{
					Line: entry.key.Line,
					Err:  errors.New("partial_response_strategy is only valid when parser is configured to use the Thanos rule schema"),
				}
			}
			if !isTag(entry.val.ShortTag(), strTag) {
				return "", nil, ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("partial_response_strategy must be a %s, got %s", describeTag(strTag), describeTag(entry.val.ShortTag())),
				}
			}
			switch val := nodeValue(entry.val); val {
			case "warn":
			case "abort":
			default:
				return "", nil, ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("invalid partial_response_strategy value: %s", val),
				}
			}
		default:
			return "", nil, ParseError{
				Line: entry.key.Line,
				Err:  fmt.Errorf("invalid group key %s", entry.key.Value),
			}
		}

		if _, ok := setKeys[entry.key.Value]; ok {
			return "", nil, ParseError{
				Line: entry.key.Line,
				Err:  fmt.Errorf("duplicated key %s", entry.key.Value),
			}
		}
		setKeys[entry.key.Value] = struct{}{}
	}

	if _, ok := setKeys["rules"]; ok {
		if _, ok := setKeys["name"]; !ok {
			return "", nil, ParseError{
				Line: group.Line,
				Err:  errors.New("incomplete group definition, name is required and must be set"),
			}
		}
	}

	return name, rules, ParseError{}
}

func parseRuleStrict(contentLines []string, rule *yaml.Node) (Rule, ParseError) {
	if !isTag(rule.ShortTag(), mapTag) {
		return Rule{}, ParseError{
			Line: rule.Line,
			Err:  fmt.Errorf("rule definion must be a %s, got %s", describeTag(mapTag), describeTag(rule.ShortTag())),
		}
	}

	for i, node := range unpackNodes(rule) {
		if i%2 != 0 {
			continue
		}
		switch node.Value {
		case recordKey:
		case alertKey:
		case exprKey:
		case forKey:
		case keepFiringForKey:
		case labelsKey:
		case annotationsKey:
		default:
			return Rule{}, ParseError{
				Line: node.Line,
				Err:  fmt.Errorf("invalid rule key %s", node.Value),
			}
		}
	}

	r, _ := parseRule(contentLines, rule, 0, 0)
	return r, ParseError{}
}
