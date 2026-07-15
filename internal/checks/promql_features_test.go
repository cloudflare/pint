package checks_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/parser/source"
	"github.com/cloudflare/pint/internal/promapi"
)

func newFeaturesCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewFeaturesCheck(prom)
}

func TestFeaturesCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "offline",
			content:     "- record: foo\n  expr: bar @ start()\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			ctx: func(ctx context.Context, _ string) context.Context {
				return promapi.WithOffline(ctx, true)
			},
		},
		// Verifies that rules with syntax errors are skipped.
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
		},
		// Verifies that queries without experimental features produce no problems.
		{
			description: "no features needed",
			content:     "- record: foo\n  expr: sum(rate(bar[5m]))\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
		},
		// Verifies that a flags API error produces a warning problem.
		{
			description: "flags query error",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  respondWithInternalError(),
				},
			},
		},
		// Verifies that an unsupported flags endpoint disables the check silently.
		{
			description: "flags unsupported",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		// Verifies that a missing feature flag produces a problem.
		{
			description: "experimental function missing feature flag",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "2.0.0"},
				},
			},
		},
		// Verifies that an enabled feature flag produces no problem.
		{
			description: "experimental function with feature flag enabled",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "2.49.0"},
				},
			},
		},
		// Verifies that feature flag is detected among multiple comma-separated flags.
		{
			description: "feature enabled among multiple flags",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "exemplar-storage,promql-experimental-functions,native-histograms",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "2.49.0"},
				},
			},
		},
		// Verifies that duration expression feature is detected and reported.
		{
			description: "duration expression missing feature flag",
			content:     "- record: foo\n  expr: rate(bar[5m+1m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "2.0.0"},
				},
			},
		},
		// Verifies that duration expression feature enabled produces no problem.
		{
			description: "duration expression with feature flag enabled",
			content:     "- record: foo\n  expr: rate(bar[5m+1m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-duration-expr",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.4.0"},
				},
			},
		},
		// Verifies that extended range selectors feature is detected and reported.
		{
			description: "smoothed selector missing feature flag",
			content:     "- record: foo\n  expr: rate(bar[5m] smoothed)\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "2.0.0"},
				},
			},
		},
		// Verifies that binop fill modifiers feature is detected and reported.
		{
			description: "fill modifier missing feature flag",
			content:     "- record: foo\n  expr: bar + fill(0) baz\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "2.0.0"},
				},
			},
		},
		// Verifies that multiple missing features produce multiple problems.
		{
			description: "multiple features missing",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m+1m] smoothed) + on(job) fill(0) baz\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "2.0.0"},
				},
			},
		},
		// Verifies that all features enabled produces no problem even with complex query.
		{
			description: "all features enabled",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m+1m] smoothed) + on(job) fill(0) baz\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions,promql-duration-expr,promql-extended-range-selectors,promql-binop-fill-modifiers",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.11.0"},
				},
			},
		},
		// Verifies that flag enabled but version too old still produces a problem.
		{
			description: "feature flag enabled but version too old",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "2.0.0"},
				},
			},
		},
		// Verifies that an unsupported buildinfo endpoint still allows the check
		// to proceed and report missing feature flags.
		{
			description: "buildinfo unsupported still reports problem",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		// Verifies that an unparseable buildinfo version produces a warning.
		{
			description: "buildinfo bad version",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "not-a-version"},
				},
			},
		},
		// Verifies that using the same experimental function twice in one query
		// merges fragments into a single feature requirement.
		{
			description: "duplicate feature fragments merged",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m]) + mad_over_time(baz[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "2.49.0"},
				},
			},
		},
		// Verifies that a graduated feature (server >= StableVersion) produces
		// no problem even without the feature flag enabled.
		{
			description: "graduated feature needs no flag",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			setup: func(t *testing.T) {
				t.Helper()
				f := source.NewFeatures()
				f.Register("mad_over_time", source.FeatureVersion{
					Flag:          source.FeatureExperimentalFunctions,
					MinVersion:    source.PrometheusVersion{Major: 2, Minor: 49, Patch: 0},
					StableVersion: source.PrometheusVersion{Major: 4, Minor: 0, Patch: 0},
				})
				source.SetFeatures(f)
				t.Cleanup(source.ResetFeatures)
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "4.0.0"},
				},
			},
		},
		// Verifies that a feature with StableVersion set still requires a flag
		// when the server version is below StableVersion.
		{
			description: "not yet graduated still needs flag",
			content:     "- record: foo\n  expr: mad_over_time(bar[5m])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			setup: func(t *testing.T) {
				t.Helper()
				f := source.NewFeatures()
				f.Register("mad_over_time", source.FeatureVersion{
					Flag:          source.FeatureExperimentalFunctions,
					MinVersion:    source.PrometheusVersion{Major: 2, Minor: 49, Patch: 0},
					StableVersion: source.PrometheusVersion{Major: 4, Minor: 0, Patch: 0},
				})
				source.SetFeatures(f)
				t.Cleanup(source.ResetFeatures)
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.0.0"},
				},
			},
		},
		// Verifies that start() and end() produce problems when the flag is missing.
		{
			description: "start() missing feature flag",
			content:     "- record: foo\n  expr: foo / (end() - start())\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that start() and end() produce no problems when the flag is enabled.
		{
			description: "start() with feature flag enabled",
			content:     "- record: foo\n  expr: foo / (end() - start())\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that start() on a version too old produces a problem even with the flag.
		{
			description: "start() version too old with flag",
			content:     "- record: foo\n  expr: foo / (end() - start())\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.11.0"},
				},
			},
		},
		// Verifies that start() on a version too old without the flag produces a problem.
		{
			description: "start() version too old without flag",
			content:     "- record: foo\n  expr: foo / (end() - start())\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.11.0"},
				},
			},
		},
		// Verifies that end() in a different expression produces a problem.
		{
			description: "end() missing feature flag",
			content:     "- record: foo\n  expr: foo / (end() - start()) + bar\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that end() in a different expression produces no problem.
		{
			description: "end() with feature flag enabled",
			content:     "- record: foo\n  expr: foo / (end() - start()) + bar\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that range() as a duration in rate() produces a problem.
		{
			description: "range() missing feature flag",
			content:     "- record: foo\n  expr: rate(foo_total[range()])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that range() as a duration in rate() produces no problem.
		{
			description: "range() with feature flag enabled",
			content:     "- record: foo\n  expr: rate(foo_total[range()])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions,promql-duration-expr",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that range() with only duration-expr enabled still reports
		// missing experimental-functions flag.
		{
			description: "range() with only duration-expr flag",
			content:     "- record: foo\n  expr: rate(foo_total[range()])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-duration-expr",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that range() with only experimental-functions enabled still
		// reports missing duration-expr flag.
		{
			description: "range() with only experimental-functions flag",
			content:     "- record: foo\n  expr: rate(foo_total[range()])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that step() as a duration in rate() produces a problem.
		{
			description: "step() missing feature flag",
			content:     "- record: foo\n  expr: rate(foo_total[step() * 4])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that step() as a duration in rate() produces no problem.
		{
			description: "step() with feature flag enabled",
			content:     "- record: foo\n  expr: rate(foo_total[step() * 4])\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions,promql-duration-expr",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// @ start() on a vector selector requires the start feature.
		{
			description: "@ start() missing feature flag",
			content:     "- record: foo\n  expr: bar @ start()\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// @ start() with the flag enabled produces no problem.
		{
			description: "@ start() with feature flag enabled",
			content:     "- record: foo\n  expr: bar @ start()\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// @ end() on a vector selector requires the end feature.
		{
			description: "@ end() missing feature flag",
			content:     "- record: foo\n  expr: bar @ end()\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// @ end() with the flag enabled produces no problem.
		{
			description: "@ end() with feature flag enabled",
			content:     "- record: foo\n  expr: bar @ end()\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// start() on the RHS of a scalar-vector binop (VectorMatching==nil).
		{
			description: "start() in scalar-vector binop",
			content:     "- record: foo\n  expr: foo / (end() - start())\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// start() on the RHS of a one-to-one binop (CardOneToOne).
		{
			description: "start() in one-to-one binop",
			content:     "- record: foo\n  expr: foo + on(job) bar / (end() - start())\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// start() on the RHS of a many-to-one binop (CardManyToOne / group_left).
		{
			description: "start() in group_left binop",
			content:     "- record: foo\n  expr: foo * on(job) group_left() bar / (end() - start())\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// start() on the LHS of a one-to-many binop (CardOneToMany / group_right).
		{
			description: "start() in group_right binop",
			content:     "- record: foo\n  expr: foo / (end() - start()) * on(job) group_right() bar\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// start() on the RHS of a many-to-many binop (or).
		{
			description: "start() in or binop",
			content:     "- record: foo\n  expr: foo or bar / (end() - start())\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// start() on the RHS of a many-to-many binop (and).
		{
			description: "start() in and binop",
			content:     "- record: foo\n  expr: foo and bar / (end() - start())\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// start() on the RHS of a many-to-many binop (unless).
		{
			description: "start() in unless binop",
			content:     "- record: foo\n  expr: foo unless bar / (end() - start())\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that min_of() produces a problem when the flag is missing.
		{
			description: "min_of() missing feature flag",
			content:     "- record: foo\n  expr: min_of(1, 2)\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.13.0"},
				},
			},
		},
		// Verifies that min_of() produces no problem when the flag is enabled.
		{
			description: "min_of() with feature flag enabled",
			content:     "- record: foo\n  expr: min_of(1, 2)\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.13.0"},
				},
			},
		},
		// Verifies that min_of() on a version too old produces a problem.
		{
			description: "min_of() version too old",
			content:     "- record: foo\n  expr: min_of(1, 2)\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
		// Verifies that max_of() produces a problem when the flag is missing.
		{
			description: "max_of() missing feature flag",
			content:     "- record: foo\n  expr: max_of(1, 2)\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp:  flagsResponse{flags: map[string]string{}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.13.0"},
				},
			},
		},
		// Verifies that max_of() produces no problem when the flag is enabled.
		{
			description: "max_of() with feature flag enabled",
			content:     "- record: foo\n  expr: max_of(1, 2)\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.13.0"},
				},
			},
		},
		// Verifies that max_of() on a version too old produces a problem.
		{
			description: "max_of() version too old",
			content:     "- record: foo\n  expr: max_of(1, 2)\n",
			checker:     newFeaturesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireFlagsPath},
					resp: flagsResponse{flags: map[string]string{
						"enable-feature": "promql-experimental-functions",
					}},
				},
				{
					conds: []requestCondition{requireBuildInfoPath},
					resp:  buildInfoResponse{version: "3.12.0"},
				},
			},
		},
	}
	runTests(t, testCases)
}
