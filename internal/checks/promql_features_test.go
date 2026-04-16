package checks_test

import (
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
	}
	runTests(t, testCases)
}
