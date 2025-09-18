package parser

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

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

type Schema uint8

const (
	PrometheusSchema Schema = iota
	ThanosSchema
)

func NewParser(isStrict bool, schema Schema, names model.ValidationScheme) Parser {
	if names == model.UnsetValidation {
		names = model.LegacyValidation
	}
	return Parser{
		isStrict: isStrict,
		schema:   schema,
		names:    names,
	}
}

type Parser struct {
	names    model.ValidationScheme
	schema   Schema
	isStrict bool
}

func (p Parser) Parse(src io.Reader) (f File) {
	cr := newContentReader(src)
	dec := yaml.NewDecoder(cr)

	defer func() {
		f.Diagnostics = cr.diagnostics
		f.Comments = cr.comments
		f.TotalLines = cr.lineno
	}()

	f.IsRelaxed = !p.isStrict

	var index int
	var g []Group
	for {
		var doc yaml.Node
		decodeErr := dec.Decode(&doc)
		if errors.Is(decodeErr, io.EOF) {
			break
		}

		if decodeErr != nil {
			f.Error = tryDecodingYamlError(decodeErr)
			return f
		}
		index++

		if p.isStrict {
			g, f.Error = p.parseGroups(&doc, 0, 0, cr.lines)
			if f.Error.Err != nil {
				return f
			}
			f.Groups = append(f.Groups, g...)
		} else {
			f.Groups = append(f.Groups, p.parseNode(&doc, nil, nil, 0, 0, cr.lines)...)
		}

		if index > 1 && p.isStrict {
			f.Error = ParseError{
				Err: errors.New("multi-document YAML files are not allowed"),
				Details: `This is a multi-document YAML file. Prometheus will only parse the first document and silently ignore the rest.
To allow for multi-document YAML files set parser->relaxed option in pint config file.`,
				Line: doc.Line,
			}
		}
	}
	return f
}

func (p *Parser) parseNode(node, parent *yaml.Node, group *Group, offsetLine, offsetColumn int, contentLines []string) (groups []Group) {
	switch node.Kind { // nolint: exhaustive
	case yaml.SequenceNode:
		// First check for group list
		for _, n := range unpackNodes(node) {
			if g, rulesMap, ok := tryParseGroup(n, offsetLine, offsetColumn, contentLines); ok {
				groups = append(groups, p.parseNode(rulesMap.val, rulesMap.key, &g, offsetLine, offsetColumn, contentLines)...)
			}
		}
		if len(groups) > 0 {
			return groups
		}

		if group == nil {
			group = &Group{} // nolint: exhaustruct
		}
		// Try parsing rules.
		for _, n := range unpackNodes(node) {
			if ret, isEmpty := p.parseRule(n, offsetLine, offsetColumn, contentLines); !isEmpty {
				group.Rules = append(group.Rules, ret)
			}
		}
		// Handle empty rules within a group.
		if len(group.Rules) > 0 || (parent != nil && nodeValue(parent) == "rules" && len(groups) == 0 && group != nil) {
			groups = append(groups, *group)
		}
		if len(groups) > 0 {
			return groups
		}
	case yaml.MappingNode:
		for _, field := range mappingNodes(node) {
			groups = append(groups, p.parseNode(field.val, field.key, group, offsetLine, offsetColumn, contentLines)...)
		}
		return groups
	case yaml.ScalarNode:
		if strings.Count(node.Value, "\n") > 1 && node.Value != strings.Join(contentLines, "\n") && node.Line < len(contentLines) {
			var n yaml.Node
			// FIXME there must be a better way.
			// If we have YAML inside YAML:
			// alerts: |
			//   groups:
			//     rules: ...
			// Then we need to get the offset of `groups` inside the FILE, not inside the YAML node value.
			// Right now we read the line where it's in the file and count leading spaces.
			if err := yaml.Unmarshal([]byte(node.Value), &n); err == nil {
				groups = append(groups,
					p.parseNode(
						&n,
						node,
						group,
						offsetLine+node.Line,
						offsetColumn+countLeadingSpace(contentLines[node.Line]),
						strings.Split(node.Value, "\n"),
					)...,
				)
				return groups
			}
		}
	}

	for _, child := range unpackNodes(node) {
		groups = append(groups, p.parseNode(child, node, group, offsetLine, offsetColumn, contentLines)...)
	}

	return groups
}

func tryParseGroup(node *yaml.Node, offsetLine, offsetColumn int, contentLines []string) (g Group, rules yamlMap, _ bool) {
	for _, e := range mappingNodes(node) {
		switch val := nodeValue(e.key); val {
		case "name":
			g.Name = nodeValue(e.val)
		case "interval":
			if interval, err := model.ParseDuration(nodeValue(e.val)); err == nil {
				g.Interval = time.Duration(interval)
			}
		case "limit":
			if limit, err := strconv.Atoi(nodeValue(e.val)); err == nil {
				g.Limit = limit
			}
		case "query_offset":
			if queryOffset, err := model.ParseDuration(nodeValue(e.val)); err == nil {
				g.QueryOffset = time.Duration(queryOffset)
			}
		case "labels":
			g.Labels = newYamlMap(e.key, e.val, offsetLine, offsetColumn, contentLines)
		case "rules":
			if e.val.Kind == yaml.SequenceNode {
				rules = e
			}
		}
	}
	return g, rules, g.Name != "" && rules.key != nil
}

func (p Parser) parseRule(node *yaml.Node, offsetLine, offsetColumn int, contentLines []string) (rule Rule, _ bool) {
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
				lines.Last = max(lines.Last, recordPart.Pos.Lines().Last)
			case alertKey:
				if alertPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, alertKey)
				}
				alertNode = part
				alertPart = newYamlNode(part, offsetLine, offsetColumn, contentLines, key.Column+2)
				lines.Last = max(lines.Last, alertPart.Pos.Lines().Last)
			case exprKey:
				if exprPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, exprKey)
				}
				exprNode = part
				exprPart = newPromQLExpr(part, offsetLine, offsetColumn, contentLines, key.Column+2)
				lines.Last = max(lines.Last, exprPart.Value.Pos.Lines().Last)
			case forKey:
				if forPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, forKey)
				}
				forNode = part
				forPart = newYamlNode(part, offsetLine, offsetColumn, contentLines, key.Column+2)
				lines.Last = max(lines.Last, forPart.Pos.Lines().Last)
			case keepFiringForKey:
				if keepFiringForPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, keepFiringForKey)
				}
				keepFiringForNode = part
				keepFiringForPart = newYamlNode(part, offsetLine, offsetColumn, contentLines, key.Column+2)
				lines.Last = max(lines.Last, keepFiringForPart.Pos.Lines().Last)
			case labelsKey:
				if labelsPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, labelsKey)
				}
				labelsNode = part
				labelsNodes = mappingNodes(part)
				labelsPart = newYamlMap(key, part, offsetLine, offsetColumn, contentLines)
				lines.Last = max(lines.Last, labelsPart.Lines().Last)
			case annotationsKey:
				if annotationsPart != nil {
					return duplicatedKeyError(lines, part.Line+offsetLine, annotationsKey)
				}
				annotationsNode = part
				annotationsNodes = mappingNodes(part)
				annotationsPart = newYamlMap(key, part, offsetLine, offsetColumn, contentLines)
				lines.Last = max(lines.Last, annotationsPart.Lines().Last)
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
				Line: exprPart.Value.Pos.Lines().Last,
				Err:  fmt.Errorf("incomplete rule, no %s or %s key", alertKey, recordKey),
			},
		}
		return rule, false
	}
	if recordPart != nil && forPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: forPart.Pos.Lines().First,
				Err:  fmt.Errorf("invalid field '%s' in recording rule", forKey),
			},
		}
		return rule, false
	}
	if recordPart != nil && keepFiringForPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: keepFiringForPart.Pos.Lines().First,
				Err:  fmt.Errorf("invalid field '%s' in recording rule", keepFiringForKey),
			},
		}
		return rule, false
	}
	if recordPart != nil && annotationsPart != nil {
		rule = Rule{
			Lines: lines,
			Error: ParseError{
				Line: annotationsPart.Lines().First,
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

	if ok, perr, plines := validateStringMap(labelsKey, labelsNodes, offsetLine, lines); !ok {
		return Rule{
			Lines: plines,
			Error: perr,
		}, false
	}

	if ok, perr, plines := validateStringMap(annotationsKey, annotationsNodes, offsetLine, lines); !ok {
		return Rule{
			Lines: plines,
			Error: perr,
		}, false
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

	if recordPart != nil && !p.names.IsValidMetricName(recordPart.Value) {
		return Rule{
			Lines: lines,
			Error: ParseError{
				Line: recordPart.Pos.Lines().First,
				Err:  fmt.Errorf("invalid recording rule name: %s", recordPart.Value),
			},
		}, false
	}

	if (recordPart != nil || alertPart != nil) && labelsPart != nil {
		for _, lab := range labelsPart.Items {
			if !p.names.IsValidLabelName(lab.Key.Value) || lab.Key.Value == model.MetricNameLabel {
				return Rule{
					Lines: lines,
					Error: ParseError{
						Line: lab.Key.Pos.Lines().First,
						Err:  fmt.Errorf("invalid label name: %s", lab.Key.Value),
					},
				}, false
			}
			if !model.LabelValue(lab.Value.Value).IsValid() {
				return Rule{
					Lines: lines,
					Error: ParseError{
						Line: lab.Key.Pos.Lines().First,
						Err:  fmt.Errorf("invalid label value: %s", lab.Value.Value),
					},
				}, false
			}
		}
	}

	if alertPart != nil && annotationsPart != nil {
		for _, ann := range annotationsPart.Items {
			if !p.names.IsValidLabelName(ann.Key.Value) {
				return Rule{
					Lines: lines,
					Error: ParseError{
						Line: ann.Key.Pos.Lines().First,
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
	return slices.Contains(nodeKeys(node), key)
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
				Line: keyVal.Pos.Lines().Last,
				Err:  fmt.Errorf("%s value cannot be empty", key),
			},
		}, false
	}
	if expr == nil {
		return Rule{
			Lines: lines,
			Error: ParseError{
				Line: keyVal.Pos.Lines().Last,
				Err:  fmt.Errorf("missing %s key", exprKey),
			},
		}, false
	}
	if !hasValue(expr.Value) {
		return Rule{
			Lines: lines,
			Error: ParseError{
				Line: expr.Value.Pos.Lines().Last,
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

func validateStringMap(field string, nodes []yamlMap, offsetLine int, lines diags.LineRange) (bool, ParseError, diags.LineRange) {
	names := map[string]struct{}{}
	for _, entry := range nodes {
		if !isTag(entry.val.ShortTag(), strTag) {
			return false, ParseError{
				Line: entry.val.Line + offsetLine,
				Err:  fmt.Errorf("%s %s value must be a %s, got %s instead", field, entry.key.Value, describeTag(strTag), describeTag(entry.val.ShortTag())),
			}, lines
		}
		if _, ok := names[entry.key.Value]; ok {
			return false, ParseError{
				Line: entry.key.Line,
				Err:  fmt.Errorf("duplicated %s key %s", field, entry.key.Value),
			}, rangeFromYamlMaps(nodes)
		}
		names[entry.key.Value] = struct{}{}
	}
	return true, ParseError{}, lines
}
