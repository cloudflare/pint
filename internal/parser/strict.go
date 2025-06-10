package parser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v3"

	"github.com/cloudflare/pint/internal/diags"
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

func (p Parser) parseGroups(doc *yaml.Node, offsetLine, offsetColumn int, contentLines []string) (groups []Group, _ ParseError) {
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
				g := p.parseGroup(group, offsetLine, offsetColumn, contentLines)
				if _, ok := names[g.Name]; ok {
					return nil, ParseError{
						Line: group.Line,
						Err:  errors.New("duplicated group name"),
					}
				}
				names[g.Name] = struct{}{}
				groups = append(groups, g)
			}
		}
	}
	return groups, ParseError{}
}

func (p Parser) parseGroup(node *yaml.Node, offsetLine, offsetColumn int, contentLines []string) (group Group) {
	if !isTag(node.ShortTag(), mapTag) {
		group.Error = ParseError{
			Line: node.Line,
			Err:  fmt.Errorf("group must be a %s, got %s", describeTag(mapTag), describeTag(node.ShortTag())),
		}
		return group
	}

	var err error
	setKeys := make(map[string]struct{}, len(node.Content))

	for _, entry := range mappingNodes(node) {
		switch entry.key.Value {
		case "name":
			if entry.val.Kind != yaml.ScalarNode || entry.val.ShortTag() != strTag {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("group name must be a %s, got %s", describeTag(strTag), describeTag(entry.val.ShortTag())),
				}
				return group
			}
			if entry.val.Value == "" {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  errors.New("group name cannot be empty"),
				}
				return group
			}
			group.Name = entry.val.Value
		case "interval":
			if entry.val.Kind != yaml.ScalarNode || entry.val.ShortTag() != strTag {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("group %s must be a %s, got %s", entry.key.Value, describeTag(strTag), describeTag(entry.val.ShortTag())),
				}
				return group
			}
			var interval model.Duration
			if interval, err = model.ParseDuration(entry.val.Value); err != nil {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("invalid %s value: %w", entry.key.Value, err),
				}
				return group
			}
			group.Interval = time.Duration(interval)
		case "query_offset":
			if entry.val.Kind != yaml.ScalarNode || entry.val.ShortTag() != strTag {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("group %s must be a %s, got %s", entry.key.Value, describeTag(strTag), describeTag(entry.val.ShortTag())),
				}
				return group
			}
			var queryOffset model.Duration
			if queryOffset, err = model.ParseDuration(entry.val.Value); err != nil {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("invalid %s value: %w", entry.key.Value, err),
				}
				return group
			}
			group.QueryOffset = time.Duration(queryOffset)
		case "limit":
			if entry.val.Kind != yaml.ScalarNode || entry.val.ShortTag() != intTag {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("group limit must be a %s, got %s", describeTag(intTag), describeTag(entry.val.ShortTag())),
				}
				return group
			}
			group.Limit, _ = strconv.Atoi(nodeValue(entry.val))
		case "labels":
			if entry.val.ShortTag() != mapTag {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("group labels must be a %s, got %s", describeTag(mapTag), describeTag(entry.val.ShortTag())),
				}
				return group
			}
			nodes := mappingNodes(entry.val)
			if ok, err, _ := validateStringMap(
				"labels",
				nodes,
				offsetLine,
				diags.LineRange{First: entry.key.Line, Last: rangeFromYamlMaps(nodes).Last},
			); !ok {
				group.Error = err
				return group
			}
			group.Labels = newYamlMap(entry.key, entry.val, offsetLine, offsetColumn, contentLines)
		case "rules":
			if !isTag(entry.val.ShortTag(), seqTag) {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("rules must be a %s, got %s", describeTag(seqTag), describeTag(entry.val.ShortTag())),
				}
				return group
			}
			for _, rule := range unpackNodes(entry.val) {
				group.Rules = append(group.Rules, p.parseRuleStrict(rule, contentLines))
			}
		case "partial_response_strategy":
			if p.schema != ThanosSchema {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  errors.New("partial_response_strategy is only valid when parser is configured to use the Thanos rule schema"),
				}
				return group
			}
			if !isTag(entry.val.ShortTag(), strTag) {
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("partial_response_strategy must be a %s, got %s", describeTag(strTag), describeTag(entry.val.ShortTag())),
				}
				return group
			}
			switch val := nodeValue(entry.val); val {
			case "warn":
			case "abort":
			default:
				group.Error = ParseError{
					Line: entry.key.Line,
					Err:  fmt.Errorf("invalid partial_response_strategy value: %s", val),
				}
				return group
			}
		default:
			group.Error = ParseError{
				Line: entry.key.Line,
				Err:  fmt.Errorf("invalid group key %s", entry.key.Value),
			}
			return group
		}

		if _, ok := setKeys[entry.key.Value]; ok {
			group.Error = ParseError{
				Line: entry.key.Line,
				Err:  fmt.Errorf("duplicated key %s", entry.key.Value),
			}
			return group
		}
		setKeys[entry.key.Value] = struct{}{}
	}

	if _, ok := setKeys["rules"]; ok {
		if _, ok := setKeys["name"]; !ok {
			group.Error = ParseError{
				Line: node.Line,
				Err:  errors.New("incomplete group definition, name is required and must be set"),
			}
			return group
		}
	}

	return group
}

func (p Parser) parseRuleStrict(rule *yaml.Node, contentLines []string) Rule {
	if !isTag(rule.ShortTag(), mapTag) {
		return Rule{
			Error: ParseError{
				Line: rule.Line,
				Err:  fmt.Errorf("rule definion must be a %s, got %s", describeTag(mapTag), describeTag(rule.ShortTag())),
			},
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
			return Rule{
				Error: ParseError{
					Line: node.Line,
					Err:  fmt.Errorf("invalid rule key %s", node.Value),
				},
			}
		}
	}

	pr, _ := p.parseRule(rule, 0, 0, contentLines)
	return pr
}
