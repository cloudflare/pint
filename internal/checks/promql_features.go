package checks

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cloudflare/pint/internal/diags"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/parser/source"
	"github.com/cloudflare/pint/internal/promapi"
)

const (
	FeaturesCheckName    = "promql/features"
	FeaturesCheckDetails = `This query uses PromQL features that require feature flags to be enabled on the Prometheus server via ` + "`--enable-feature=...`" + ` flag.
[Click here](https://prometheus.io/docs/prometheus/latest/feature_flags/) to see Prometheus feature flags documentation.`
)

func NewFeaturesCheck(prom *promapi.FailoverGroup) FeaturesCheck {
	return FeaturesCheck{
		prom:     prom,
		instance: fmt.Sprintf("%s(%s)", FeaturesCheckName, prom.Name()),
	}
}

type FeaturesCheck struct {
	prom     *promapi.FailoverGroup
	instance string
}

func (c FeaturesCheck) Meta() CheckMeta {
	return CheckMeta{
		States: []discovery.ChangeType{
			discovery.Noop,
			discovery.Added,
			discovery.Modified,
			discovery.Moved,
		},
		Online:        true,
		AlwaysEnabled: false,
	}
}

func (c FeaturesCheck) String() string {
	return c.instance
}

func (c FeaturesCheck) Reporter() string {
	return FeaturesCheckName
}

func (c FeaturesCheck) Check(ctx context.Context, entry *discovery.Entry, _ []*discovery.Entry) (problems []Problem) {
	expr := entry.Rule.Expr()
	if expr.SyntaxError() != nil {
		return problems
	}

	needed := requiredFeatures(expr)
	if len(needed) == 0 {
		return problems
	}

	flags, err := c.prom.Flags(ctx)
	if err != nil {
		if errors.Is(err, promapi.ErrUnsupported) {
			c.prom.DisableCheck(promapi.APIPathFlags, c.Reporter())
			return problems
		}
		problems = append(problems, problemFromError(err, entry.Rule, c.Reporter(), c.prom.Name(), Warning))
		return problems
	}

	var serverVersion source.PrometheusVersion
	bi, err := c.prom.BuildInfo(ctx)
	if err != nil {
		if errors.Is(err, promapi.ErrUnsupported) {
			c.prom.DisableCheck(promapi.APIPathBuildInfo, c.Reporter())
		}
	} else {
		serverVersion, err = source.ParseVersion(bi.Version)
		if err != nil {
			problems = append(problems, problemFromError(err, entry.Rule, c.Reporter(), c.prom.Name(), Warning))
			return problems
		}
	}

	enabled := enabledFeatures(flags.Flags)
	for _, req := range needed {
		fv, known := source.LookupFeatureVersion(req.Name)

		versionTooOld := false
		graduated := false
		if known && !serverVersion.IsZero() {
			versionTooOld = serverVersion.IsLessThan(fv.MinVersion)
			graduated = !fv.StableVersion.IsZero() && !serverVersion.IsLessThan(fv.StableVersion)
		}

		if graduated {
			continue
		}

		if !versionTooOld && enabled[req.Feature] {
			continue
		}

		msg := featureMessage(req, c.prom.Name(), flags.URI, serverVersion)
		d := make([]diags.Diagnostic, 0, len(req.Fragments))
		for _, frag := range req.Fragments {
			d = append(d, diags.Diagnostic{
				Message:     msg,
				Pos:         expr.Value.Pos,
				FirstColumn: int(frag.Start) + 1,
				LastColumn:  int(frag.End),
				Kind:        diags.Issue,
			})
		}
		problems = append(problems, Problem{
			Anchor:      AnchorAfter,
			Lines:       expr.Value.Pos.Lines(),
			Reporter:    c.Reporter(),
			Summary:     "required feature flag not enabled",
			Details:     FeaturesCheckDetails,
			Severity:    Bug,
			Diagnostics: d,
		})
	}

	return problems
}

type featureKey struct {
	feature string
	name    string
}

func requiredFeatures(expr *parser.PromQLExpr) []source.FeatureRequirement {
	seen := map[featureKey]*source.FeatureRequirement{}
	for _, src := range expr.Source() {
		src.WalkSources(func(s source.Source, _ *source.Join, _ *source.Unless) {
			for _, req := range s.NeedsFeatures {
				key := featureKey{feature: req.Feature, name: req.Name}
				if existing, ok := seen[key]; ok {
					existing.Fragments = append(existing.Fragments, req.Fragments...)
				} else {
					cp := req
					seen[key] = &cp
				}
			}
		})
	}
	features := make([]source.FeatureRequirement, 0, len(seen))
	for _, req := range seen {
		features = append(features, *req)
	}
	slices.SortFunc(features, func(a, b source.FeatureRequirement) int {
		return cmp.Compare(a.Feature, b.Feature)
	})
	return features
}

func featureMessage(req source.FeatureRequirement, name, uri string, sv source.PrometheusVersion) string {
	fv, _ := source.LookupFeatureVersion(req.Name)
	if !sv.IsZero() && sv.IsLessThan(fv.MinVersion) {
		return fmt.Sprintf(
			"`%s` requires Prometheus %s or later but %s is running %s.",
			req.Name, fv.MinVersion, promText(name, uri), sv,
		)
	}
	if sv.IsZero() {
		return fmt.Sprintf(
			"`%s` was added in Prometheus %s and requires `--enable-feature=%s` to be set on %s.",
			req.Name, fv.MinVersion,
			req.Feature, promText(name, uri),
		)
	}
	return fmt.Sprintf(
		"`%s` requires `--enable-feature=%s` to be set on %s.",
		req.Name,
		req.Feature, promText(name, uri),
	)
}

func enabledFeatures(flags map[string]string) map[string]bool {
	enabled := map[string]bool{}
	if raw, ok := flags["enable-feature"]; ok {
		for f := range strings.SplitSeq(raw, ",") {
			enabled[strings.TrimSpace(f)] = true
		}
	}
	return enabled
}
