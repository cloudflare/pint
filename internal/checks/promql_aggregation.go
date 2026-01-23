package checks

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	promParser "github.com/prometheus/prometheus/promql/parser"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser/source"
)

const (
	AggregationCheckName = "promql/aggregate"
)

// labelLiteralPattern matches simple label patterns that are either:
// - A single literal label name (e.g., "job")
// - An alternation of literal label names (e.g., "job|instance")
var labelLiteralPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\|[a-zA-Z_][a-zA-Z0-9_]*)*$`)

// extractLiteralLabels extracts individual label names from a simple pattern.
// Returns nil if the pattern contains complex regex constructs.
func extractLiteralLabels(pattern string) []string {
	if !labelLiteralPattern.MatchString(pattern) {
		return nil
	}
	return strings.Split(pattern, "|")
}

func NewAggregationCheck(nameRegex, labelRegex *TemplatedRegexp, keep bool, comment string, severity Severity) AggregationCheck {
	return AggregationCheck{
		nameRegex:  nameRegex,
		labelRegex: labelRegex,
		keep:       keep,
		comment:    comment,
		severity:   severity,
		instance:   fmt.Sprintf("%s(%s:%v)", AggregationCheckName, labelRegex.original, keep),
	}
}

type AggregationCheck struct {
	nameRegex  *TemplatedRegexp
	labelRegex *TemplatedRegexp
	comment    string
	instance   string
	severity   Severity
	keep       bool
}

func (c AggregationCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        false,
		AlwaysEnabled: false,
	}
}

func (c AggregationCheck) String() string {
	return c.instance
}

func (c AggregationCheck) Reporter() string {
	return AggregationCheckName
}

func (c AggregationCheck) Check(_ context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError() != nil {
		return nil
	}

	if c.nameRegex != nil {
		if entry.Rule.RecordingRule != nil && !c.nameRegex.MustExpand(entry.Rule).MatchString(entry.Rule.RecordingRule.Record.Value) {
			return nil
		}
		if entry.Rule.AlertingRule != nil && !c.nameRegex.MustExpand(entry.Rule).MatchString(entry.Rule.AlertingRule.Alert.Value) {
			return nil
		}
	}

	labelRe := c.labelRegex.MustExpand(entry.Rule)

	// Check if any matching label is statically defined in the rule's labels block.
	// If so, skip the check for that label since it will be preserved regardless of aggregation.
	staticLabels := make(map[string]bool)
	if entry.Rule.RecordingRule != nil && entry.Rule.RecordingRule.Labels != nil {
		for _, item := range entry.Rule.RecordingRule.Labels.Items {
			if labelRe.MatchString(item.Key.Value) {
				staticLabels[item.Key.Value] = true
			}
		}
	}
	if entry.Rule.AlertingRule != nil && entry.Rule.AlertingRule.Labels != nil {
		for _, item := range entry.Rule.AlertingRule.Labels.Items {
			if labelRe.MatchString(item.Key.Value) {
				staticLabels[item.Key.Value] = true
			}
		}
	}

	nameDesc := "`" + c.nameRegex.anchored + "`"
	if nameDesc == "`^.+$`" || nameDesc == "`^.*$`" {
		nameDesc = "all"
	}

	for _, src := range expr.Source() {
		if src.Type != source.AggregateSource {
			continue
		}

		if c.keep {
			// For keep=true: find all labels matching the regex that are being removed.
			// Track which labels we've already reported to avoid duplicates.
			reportedLabels := make(map[string]bool)

			for labelName, labelInfo := range src.Labels {
				// Skip empty label name (used for default exclusion reason).
				if labelName == "" {
					continue
				}
				// Skip labels that don't match the regex.
				if !labelRe.MatchString(labelName) {
					continue
				}
				// Skip labels that are statically defined in the rule.
				if staticLabels[labelName] {
					continue
				}
				// Check if this label is being removed.
				if labelInfo.Kind == source.ImpossibleLabel {
					reason, fragment := src.LabelExcludeReason(labelName)
					problems = append(problems, Problem{
						Anchor:   AnchorAfter,
						Lines:    expr.Value.Pos.Lines(),
						Reporter: c.Reporter(),
						Summary:  "required label is being removed via aggregation",
						Details:  maybeComment(c.comment),
						Diagnostics: []diags.Diagnostic{
							{
								Message: fmt.Sprintf("`%s` label is required and should be preserved when aggregating %s rules.",
									labelName, nameDesc),
								Pos:         expr.Value.Pos,
								FirstColumn: int(fragment.Start) + 1,
								LastColumn:  int(fragment.End),
								Kind:        diags.Issue,
							},
							{
								Message:     reason,
								Pos:         expr.Value.Pos,
								FirstColumn: int(fragment.Start) + 1,
								LastColumn:  int(fragment.End),
								Kind:        diags.Context,
							},
						},
						Severity: c.severity,
					})
					reportedLabels[labelName] = true
				}
			}

			// For simple literal patterns, also check using CanHaveLabel for labels
			// not explicitly tracked. This handles the case where FixedLabels=true
			// but the label wasn't mentioned in the query (e.g., "sum(foo) by(x)").
			if literals := extractLiteralLabels(c.labelRegex.original); literals != nil {
				for _, labelName := range literals {
					// Skip if already reported or statically defined.
					if reportedLabels[labelName] || staticLabels[labelName] {
						continue
					}
					// Skip if the label is already in src.Labels (already handled above).
					if _, ok := src.Labels[labelName]; ok {
						continue
					}
					// Check if this label cannot be present (i.e., it's being removed).
					if !src.CanHaveLabel(labelName) {
						reason, fragment := src.LabelExcludeReason(labelName)
						problems = append(problems, Problem{
							Anchor:   AnchorAfter,
							Lines:    expr.Value.Pos.Lines(),
							Reporter: c.Reporter(),
							Summary:  "required label is being removed via aggregation",
							Details:  maybeComment(c.comment),
							Diagnostics: []diags.Diagnostic{
								{
									Message: fmt.Sprintf("`%s` label is required and should be preserved when aggregating %s rules.",
										labelName, nameDesc),
									Pos:         expr.Value.Pos,
									FirstColumn: int(fragment.Start) + 1,
									LastColumn:  int(fragment.End),
									Kind:        diags.Issue,
								},
								{
									Message:     reason,
									Pos:         expr.Value.Pos,
									FirstColumn: int(fragment.Start) + 1,
									LastColumn:  int(fragment.End),
									Kind:        diags.Context,
								},
							},
							Severity: c.severity,
						})
					}
				}
			}
		} else {
			// For keep=false (strip): find all labels matching the regex that are still present.
			// Track which labels we've already reported to avoid duplicates.
			reportedLabels := make(map[string]bool)

			// First, check labels explicitly tracked in src.Labels.
			for labelName, labelInfo := range src.Labels {
				// Skip empty label name.
				if labelName == "" {
					continue
				}
				// Skip labels that don't match the regex.
				if !labelRe.MatchString(labelName) {
					continue
				}
				// Check if this label is still present.
				if labelInfo.Kind == source.PossibleLabel || labelInfo.Kind == source.GuaranteedLabel {
					posrange := src.Position
					if aggr, ok := source.MostOuterOperation[*promParser.AggregateExpr](src); ok {
						posrange = aggr.PosRange
						if len(aggr.Grouping) != 0 {
							if aggr.Without {
								posrange = source.FindFuncNamePosition(expr.Value.Value, aggr.PosRange, "without")
							} else {
								posrange = source.FindFuncNamePosition(expr.Value.Value, aggr.PosRange, "by")
							}
						}
					}
					if labelInfo.Fragment.Start != 0 || labelInfo.Fragment.End != 0 {
						posrange = labelInfo.Fragment
					}
					problems = append(problems, Problem{
						Anchor:   AnchorAfter,
						Lines:    expr.Value.Pos.Lines(),
						Reporter: c.Reporter(),
						Summary:  "label must be removed in aggregations",
						Details:  maybeComment(c.comment),
						Diagnostics: []diags.Diagnostic{
							{
								Message: fmt.Sprintf("`%s` label should be removed when aggregating %s rules.",
									labelName, nameDesc),
								Pos:         expr.Value.Pos,
								FirstColumn: int(posrange.Start) + 1,
								LastColumn:  int(posrange.End),
								Kind:        diags.Issue,
							},
						},
						Severity: c.severity,
					})
					reportedLabels[labelName] = true
				}
			}

			// For simple literal patterns (like "job" or "job|instance"), also check
			// using CanHaveLabel for labels not explicitly tracked in src.Labels.
			// This handles the case where aggregation with "without(x)" doesn't track
			// other labels that would still be present.
			if literals := extractLiteralLabels(c.labelRegex.original); literals != nil {
				for _, labelName := range literals {
					// Skip if already reported.
					if reportedLabels[labelName] {
						continue
					}
					// Skip if explicitly marked as impossible in src.Labels.
					if labelInfo, ok := src.Labels[labelName]; ok && labelInfo.Kind == source.ImpossibleLabel {
						continue
					}
					// Check if this label can be present.
					if src.CanHaveLabel(labelName) {
						posrange := src.Position
						if aggr, ok := source.MostOuterOperation[*promParser.AggregateExpr](src); ok {
							posrange = aggr.PosRange
							if len(aggr.Grouping) != 0 {
								if aggr.Without {
									posrange = source.FindFuncNamePosition(expr.Value.Value, aggr.PosRange, "without")
								} else {
									posrange = source.FindFuncNamePosition(expr.Value.Value, aggr.PosRange, "by")
								}
							}
						}
						problems = append(problems, Problem{
							Anchor:   AnchorAfter,
							Lines:    expr.Value.Pos.Lines(),
							Reporter: c.Reporter(),
							Summary:  "label must be removed in aggregations",
							Details:  maybeComment(c.comment),
							Diagnostics: []diags.Diagnostic{
								{
									Message: fmt.Sprintf("`%s` label should be removed when aggregating %s rules.",
										labelName, nameDesc),
									Pos:         expr.Value.Pos,
									FirstColumn: int(posrange.Start) + 1,
									LastColumn:  int(posrange.End),
									Kind:        diags.Issue,
								},
							},
							Severity: c.severity,
						})
					}
				}
			}
		}
	}

	return problems
}
