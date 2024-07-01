package parser

import (
	"fmt"
	"strings"

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

type validator interface {
	validate(*yaml.Node) []ParseError
}

func mustKind(node *yaml.Node, kind yaml.Kind) error {
	if node == nil {
		return fmt.Errorf("%s is required here but got nil", describeKind(kind))
	}
	if node.Kind != kind {
		return fmt.Errorf("%s is not allowed here, expected a %s", describeKind(node.Kind), describeKind(kind))
	}
	return nil
}

func mustKindTag(node *yaml.Node, kind yaml.Kind, tag string) (err error) {
	if err = mustKind(node, kind); err != nil {
		return err
	}
	if tag != "" && node.ShortTag() != tag {
		return fmt.Errorf("expected a YAML %s here, got %s instead", descirbeTag(tag), descirbeTag(node.ShortTag()))
	}
	return nil
}

type requireDocument struct {
	v validator
}

func (rd requireDocument) validate(node *yaml.Node) (errs []ParseError) {
	if err := mustKind(node, yaml.DocumentNode); err != nil {
		return []ParseError{{Err: err, Line: node.Line}}
	}
	for _, child := range node.Content {
		errs = append(errs, rd.v.validate(child)...)
	}
	return errs
}

type requireScalar struct {
	tag string
}

func (rs requireScalar) validate(node *yaml.Node) (errs []ParseError) {
	if err := mustKindTag(node, yaml.ScalarNode, rs.tag); err != nil {
		return []ParseError{{Err: err, Line: node.Line}}
	}
	return nil
}

type requireList struct {
	v validator
}

func (rl requireList) validate(node *yaml.Node) (errs []ParseError) {
	if err := mustKind(node, yaml.SequenceNode); err != nil {
		return []ParseError{{Err: err, Line: node.Line}}
	}
	for _, n := range unpackNodes(node) {
		errs = append(errs, rl.v.validate(n)...)
	}
	return errs
}

type requireMap struct {
	v validator
}

func (rm requireMap) validate(node *yaml.Node) (errs []ParseError) {
	if err := mustKind(node, yaml.MappingNode); err != nil {
		return []ParseError{{Err: err, Line: node.Line}}
	}

	setKeys := map[string]struct{}{}

	var ok bool
	for _, n := range unpackNodes(node) {
		errs = append(errs, rm.v.validate(n)...)
		if _, ok = setKeys[n.Value]; ok {
			errs = append(errs, ParseError{
				Err:  fmt.Errorf("duplicated key `%s`", n.Value),
				Line: n.Line,
			})
		}
		setKeys[n.Value] = struct{}{}
	}

	return errs
}

type requireExactMap struct {
	nameCallback map[string]func(string, int)
	keys         map[string]validator
	required     []string
}

func (rm requireExactMap) validate(node *yaml.Node) (errs []ParseError) {
	if err := mustKind(node, yaml.MappingNode); err != nil {
		return []ParseError{{Err: err, Line: node.Line}}
	}

	setKeys := make(map[string]struct{}, len(rm.required))

	var v validator
	var ok bool
	var fns []func(string, int)
	for i, n := range unpackNodes(node) {
		if i%2 == 0 {
			if err := mustKindTag(n, yaml.ScalarNode, strTag); err != nil {
				errs = append(errs, ParseError{Err: err, Line: n.Line})
				continue
			}
			v, ok = rm.keys[n.Value]
			if !ok {
				errs = append(errs, ParseError{
					Err:  fmt.Errorf("unexpected key `%s`", n.Value),
					Line: n.Line,
				})
			}
			if _, ok = setKeys[n.Value]; ok {
				errs = append(errs, ParseError{
					Err:  fmt.Errorf("duplicated key `%s`", n.Value),
					Line: n.Line,
				})
			}
			setKeys[n.Value] = struct{}{}

			for name, fn := range rm.nameCallback {
				if name == n.Value {
					fns = append(fns, fn)
				}
			}
		} else if v != nil {
			errs = append(errs, v.validate(n)...)
			for _, fn := range fns {
				fn(n.Value, n.Line)
			}
			fns = nil
		}
	}

	for _, key := range rm.required {
		if _, ok = setKeys[key]; !ok {
			errs = append(errs, ParseError{
				Err:  fmt.Errorf("`%s` key is required and must be set", key),
				Line: node.Line,
			})
		}
	}

	return errs
}

func validateRuleFile(node *yaml.Node) (errs []ParseError) {
	groupNames := map[string][]int{}
	nameCallback := func(s string, l int) {
		groupNames[s] = append(groupNames[s], l)
	}

	v := requireDocument{
		v: requireExactMap{
			keys: map[string]validator{
				"groups": requireList{
					v: requireExactMap{
						keys: map[string]validator{
							"name":         requireScalar{tag: strTag},
							"interval":     requireScalar{tag: strTag},
							"limit":        requireScalar{tag: intTag},
							"query_offset": requireScalar{tag: strTag},
							"rules": requireList{
								v: requireExactMap{
									keys: map[string]validator{
										"record":          requireScalar{tag: strTag},
										"alert":           requireScalar{tag: strTag},
										"expr":            requireScalar{tag: strTag},
										"for":             requireScalar{tag: strTag},
										"keep_firing_for": requireScalar{tag: strTag},
										"labels":          requireMap{v: requireScalar{tag: strTag}},
										"annotations":     requireMap{v: requireScalar{tag: strTag}},
									},
								},
							},
						},
						required: []string{"name", "rules"},
						nameCallback: map[string]func(string, int){
							"name": nameCallback,
						},
					},
				},
			},
		},
	}

	errs = append(errs, v.validate(node)...)

	for name, lines := range groupNames {
		for i, line := range lines {
			if i == 0 {
				continue
			}
			errs = append(errs, ParseError{
				Err:  fmt.Errorf("duplicated group name `%s`", name),
				Line: line,
			})
		}
	}

	return errs
}

func descirbeTag(tag string) string {
	switch tag {
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

func describeKind(kind yaml.Kind) string {
	switch kind {
	case yaml.DocumentNode:
		return "YAML document"
	case yaml.SequenceNode:
		return "YAML list"
	case yaml.MappingNode:
		return "YAML mapping"
	case yaml.ScalarNode:
		return "YAML scalar value"
	case yaml.AliasNode:
		return "YAML alias"
	}
	return "unknown node"
}
