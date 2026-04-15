package source

import (
	"fmt"
	"strconv"
	"strings"

	promParser "github.com/prometheus/prometheus/promql/parser"
)

type PrometheusVersion struct {
	Major int
	Minor int
	Patch int
}

func (v PrometheusVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v PrometheusVersion) IsZero() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0
}

func (v PrometheusVersion) IsLessThan(other PrometheusVersion) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

// ParseVersion parses a Prometheus version string like "2.49.0" or "3.5.0-rc.1"
// into a PrometheusVersion.
func ParseVersion(s string) (PrometheusVersion, error) {
	raw := s
	// Strip leading "v" if present (e.g. "v2.49.0").
	s = strings.TrimPrefix(s, "v")
	// Strip any pre-release suffix (e.g. "3.5.0-rc.1" -> "3.5.0").
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		s = s[:idx]
	}
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return PrometheusVersion{Major: 0, Minor: 0, Patch: 0},
			fmt.Errorf("failed to parse Prometheus version %q: expected major.minor.patch format", raw)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return PrometheusVersion{Major: 0, Minor: 0, Patch: 0},
			fmt.Errorf("failed to parse Prometheus version %q: %w", raw, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return PrometheusVersion{Major: 0, Minor: 0, Patch: 0},
			fmt.Errorf("failed to parse Prometheus version %q: %w", raw, err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return PrometheusVersion{Major: 0, Minor: 0, Patch: 0},
			fmt.Errorf("failed to parse Prometheus version %q: %w", raw, err)
	}
	return PrometheusVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

type FeatureVersion struct {
	Flag          string
	MinVersion    PrometheusVersion
	StableVersion PrometheusVersion
}

func ver(major, minor, patch int) PrometheusVersion {
	return PrometheusVersion{Major: major, Minor: minor, Patch: patch}
}

// Features is a registry of PromQL features that require feature flags.
// It maps feature element names to their version and flag requirements.
// Use the package-level default via LookupFeatureVersion, or create a
// custom registry in tests via NewFeatures and SetFeatures.
type Features struct {
	entries map[string]FeatureVersion
}

// NewFeatures creates an empty feature registry.
func NewFeatures() *Features {
	return &Features{entries: map[string]FeatureVersion{}}
}

// Register adds a feature to the registry.
func (f *Features) Register(name string, fv FeatureVersion) {
	f.entries[name] = fv
}

// Lookup returns the FeatureVersion for a given feature element name, if registered.
func (f *Features) Lookup(name string) (FeatureVersion, bool) {
	v, ok := f.entries[name]
	return v, ok
}

// defaultFeatures is the package-level feature registry used by
// LookupFeatureVersion and requireFeature. Tests can replace it
// via SetFeatures.
var defaultFeatures = newDefaultFeatures()

// newDefaultFeatures builds the standard feature registry with all
// known experimental PromQL features.
func newDefaultFeatures() *Features {
	f := NewFeatures()

	// Experimental functions behind promql-experimental-functions.
	f.Register("mad_over_time", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(2, 49, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register("sort_by_label", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(2, 49, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register("sort_by_label_desc", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(2, 49, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register("info", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(2, 55, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register("double_exponential_smoothing", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(3, 0, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register("ts_of_max_over_time", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(3, 5, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register("ts_of_min_over_time", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(3, 5, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register("ts_of_last_over_time", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(3, 5, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register("first_over_time", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(3, 7, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register("ts_of_first_over_time", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(3, 7, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register("histogram_quantiles", FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(3, 11, 0),
		StableVersion: ver(0, 0, 0),
	})

	// Experimental aggregation operators behind promql-experimental-functions.
	f.Register(promParser.ItemTypeStr[promParser.LIMITK], FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(2, 54, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register(promParser.ItemTypeStr[promParser.LIMIT_RATIO], FeatureVersion{
		Flag:          FeatureExperimentalFunctions,
		MinVersion:    ver(2, 54, 0),
		StableVersion: ver(0, 0, 0),
	})

	// Duration expressions behind promql-duration-expr.
	f.Register("duration_expr", FeatureVersion{
		Flag:          FeatureDurationExpr,
		MinVersion:    ver(3, 4, 0),
		StableVersion: ver(0, 0, 0),
	})

	// Extended range selectors behind promql-extended-range-selectors.
	f.Register(promParser.ItemTypeStr[promParser.ANCHORED], FeatureVersion{
		Flag:          FeatureExtendedRangeSelectors,
		MinVersion:    ver(3, 7, 0),
		StableVersion: ver(0, 0, 0),
	})
	f.Register(promParser.ItemTypeStr[promParser.SMOOTHED], FeatureVersion{
		Flag:          FeatureExtendedRangeSelectors,
		MinVersion:    ver(3, 7, 0),
		StableVersion: ver(0, 0, 0),
	})

	// Fill modifiers behind promql-binop-fill-modifiers.
	f.Register(promParser.ItemTypeStr[promParser.FILL], FeatureVersion{
		Flag:          FeatureBinopFillModifiers,
		MinVersion:    ver(3, 10, 0),
		StableVersion: ver(0, 0, 0),
	})

	return f
}

// SetFeatures replaces the package-level feature registry.
// Intended for use in tests to inject fake features.
func SetFeatures(f *Features) {
	defaultFeatures = f
}

// ResetFeatures restores the package-level feature registry to the
// built-in defaults. Call this in test cleanup.
func ResetFeatures() {
	defaultFeatures = newDefaultFeatures()
}

// LookupFeatureVersion returns the FeatureVersion for a given feature
// element name from the default registry, if it exists.
func LookupFeatureVersion(name string) (FeatureVersion, bool) {
	return defaultFeatures.Lookup(name)
}
