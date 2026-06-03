package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/neilotoole/slogt/v2"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryConfig(t *testing.T) {
	type testCaseT struct {
		err  string
		conf Discovery
	}

	testCases := []testCaseT{
		{
			conf: Discovery{},
		},
		{
			conf: Discovery{
				FilePath: []FilePath{
					{
						Directory: ".",
						Match:     "foo-(.+++).yaml",
					},
				},
			},
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			conf: Discovery{
				FilePath: []FilePath{
					{
						Directory: ".",
						Match:     "foo-(.+).yaml",
						Ignore: []string{
							".+",
							"foo-(.+++).yaml",
						},
					},
				},
			},
			err: "error parsing regexp: invalid nested repetition operator: `++`",
		},
		{
			conf: Discovery{
				FilePath: []FilePath{
					{
						Directory: ".",
						Match:     "foo-(.+).yaml",
					},
				},
			},
			err: "prometheusQuery discovery requires at least one template",
		},
		{
			conf: Discovery{
				FilePath: []FilePath{
					{
						Directory: ".",
						Match:     "foo-(.+).yaml",
						Template: []PrometheusTemplate{
							{},
						},
					},
				},
			},
			err: "prometheus template name cannot be empty",
		},
		{
			conf: Discovery{
				FilePath: []FilePath{
					{
						Directory: ".",
						Match:     "foo-(.+).yaml",
						Template: []PrometheusTemplate{
							{Name: "foo"},
						},
					},
				},
			},
			err: "prometheus template URI cannot be empty",
		},
		{
			conf: Discovery{
				FilePath: []FilePath{
					{
						Directory: ".",
						Match:     "foo-(.+).yaml",
						Template: []PrometheusTemplate{
							{
								Name:      "foo",
								URI:       "http://localhost",
								PublicURI: "http://localhost",
							},
						},
					},
				},
			},
		},
		{
			conf: Discovery{
				PrometheusQuery: []PrometheusQuery{
					{
						Timeout: "2z",
					},
				},
			},
			err: `unknown unit "z" in duration "2z"`,
		},
		{
			conf: Discovery{
				PrometheusQuery: []PrometheusQuery{
					{
						TLS: &TLSConfig{
							ClientKey: "xxx",
						},
					},
				},
			},
			err: `clientCert and clientKey must be set together`,
		},
		{
			conf: Discovery{
				PrometheusQuery: []PrometheusQuery{
					{
						Query: "foo{",
					},
				},
			},
			err: `failed to parse prometheus query "foo{": 1:5: parse error: unexpected end of input inside braces`,
		},
		{
			conf: Discovery{
				PrometheusQuery: []PrometheusQuery{
					{
						Query: "foo",
					},
				},
			},
			err: "prometheusQuery discovery requires at least one template",
		},
		{
			conf: Discovery{
				PrometheusQuery: []PrometheusQuery{
					{
						Query: "foo",
						Template: []PrometheusTemplate{
							{},
						},
					},
				},
			},
			err: "prometheus template name cannot be empty",
		},
		{
			conf: Discovery{
				PrometheusQuery: []PrometheusQuery{
					{
						Query: "foo",
						Template: []PrometheusTemplate{
							{Name: "foo"},
						},
					},
				},
			},
			err: "prometheus template URI cannot be empty",
		},
		{
			conf: Discovery{
				PrometheusQuery: []PrometheusQuery{
					{
						Query: "foo",
						Template: []PrometheusTemplate{
							{
								Name:    "foo",
								URI:     "http://localhost",
								Timeout: "1z",
							},
						},
					},
				},
			},
			err: `unknown unit "z" in duration "1z"`,
		},
		{
			conf: Discovery{
				PrometheusQuery: []PrometheusQuery{
					{
						Query: "foo",
						Template: []PrometheusTemplate{
							{
								Name: "foo",
								URI:  "http://localhost",
								TLS: &TLSConfig{
									ClientCert: "xxx",
								},
							},
						},
					},
				},
			},
			err: "clientCert and clientKey must be set together",
		},
		{
			conf: Discovery{
				PrometheusQuery: []PrometheusQuery{
					{
						Query: "foo",
						Template: []PrometheusTemplate{
							{
								Name: "foo",
								URI:  "http://localhost",
							},
						},
					},
				},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			err := tc.conf.validate()
			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.err)
			}
		})
	}
}

func TestPrometheusTemplateRender(t *testing.T) {
	type testCaseT struct {
		data     map[string]string
		err      string
		template PrometheusTemplate
	}

	testCases := []testCaseT{
		{
			template: PrometheusTemplate{},
			data:     map[string]string{},
			err:      "prometheus URI cannot be empty",
		},
		{
			template: PrometheusTemplate{
				Name: "{{ $name }}",
				URI:  "http://",
			},
			data: map[string]string{},
			err:  `bad name template "{{ $name }}": template: discovery:1: undefined variable "$name"`,
		},
		{
			template: PrometheusTemplate{
				URI: "http://{{ $name }}",
			},
			data: map[string]string{},
			err:  `bad uri template "http://{{ $name }}": template: discovery:1: undefined variable "$name"`,
		},
		{
			template: PrometheusTemplate{
				URI:       "http://{{ $name }}",
				PublicURI: "http://{{ $foo }}",
			},
			data: map[string]string{"name": "foo"},
			err:  `bad publicURI template "http://{{ $foo }}": template: discovery:1: undefined variable "$foo"`,
		},
		{
			template: PrometheusTemplate{
				Name:    "foo",
				URI:     "http://{{ $name }}",
				Timeout: "1z",
			},
			data: map[string]string{"name": "foo", "cluster": "bar"},
			err:  `unknown unit "z" in duration "1z"`,
		},
		{
			template: PrometheusTemplate{
				Name:    "foo",
				URI:     "http://{{ $name }}",
				Headers: map[string]string{"X-Cluster": "{{ $cluster }}"},
				Tags:    []string{"x", "cluster/{{ $cluster }}"},
			},
			data: map[string]string{"name": "foo", "cluster": "bar"},
		},
		{
			template: PrometheusTemplate{
				Name:     "foo",
				URI:      "http://",
				Failover: []string{"foo", "{{ $bob }}"},
			},
			data: map[string]string{},
			err:  `bad failover template "{{ $bob }}": template: discovery:1: undefined variable "$bob"`,
		},
		{
			template: PrometheusTemplate{
				Name:    "foo",
				URI:     "http://",
				Include: []string{"foo", "{{ $bob }}"},
			},
			data: map[string]string{},
			err:  `bad include template "{{ $bob }}": template: discovery:1: undefined variable "$bob"`,
		},
		{
			template: PrometheusTemplate{
				Name:    "foo",
				URI:     "http://",
				Exclude: []string{"foo", "{{ $bob }}"},
			},
			data: map[string]string{},
			err:  `bad exclude template "{{ $bob }}": template: discovery:1: undefined variable "$bob"`,
		},
		{
			template: PrometheusTemplate{
				Name:    "foo",
				URI:     "http://",
				Headers: map[string]string{"{{ $bob }}": "val"},
			},
			data: map[string]string{},
			err:  `bad header key template "{{ $bob }}": template: discovery:1: undefined variable "$bob"`,
		},
		{
			template: PrometheusTemplate{
				Name:    "foo",
				URI:     "http://",
				Headers: map[string]string{"key": "{{ $bob }}"},
			},
			data: map[string]string{},
			err:  `bad header value template "{{ $bob }}": template: discovery:1: undefined variable "$bob"`,
		},
		{
			template: PrometheusTemplate{
				Name: "foo",
				URI:  "http://",
				Tags: []string{"key", "{{ $bob }}"},
			},
			data: map[string]string{},
			err:  `bad tag template "{{ $bob }}": template: discovery:1: undefined variable "$bob"`,
		},
		{
			template: PrometheusTemplate{
				Name: "foo",
				URI:  "http://",
				Tags: []string{"key", "{{ $bob }}"},
				TLS: &TLSConfig{
					ClientKey: "xxx",
				},
			},
			data: map[string]string{"bob": "bob"},
			err:  "clientCert and clientKey must be set together",
		},
		{
			template: PrometheusTemplate{
				Name: "foo",
				URI:  "http://{{ .missing }}",
			},
			data: map[string]string{"name": "foo"},
			err:  `bad uri template "http://{{ .missing }}": template: discovery:1:30: executing "discovery" at <.missing>: map has no entry for key "missing"`,
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			slog.SetDefault(slogt.New(t))
			_, err := tc.template.Render(tc.data)
			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.err)
			}
		})
	}
}

func TestFilePathDiscover(t *testing.T) {
	type testCaseT struct {
		setup       func(t *testing.T) (FilePath, string)
		description string
		names       []string
	}

	testCases := []testCaseT{
		{
			description: "walk error on non-existent directory",
			setup: func(t *testing.T) (FilePath, string) {
				t.Helper()
				dir := filepath.Join(t.TempDir(), "missing")
				return FilePath{
						Directory: dir,
						Match:     ".+",
						Template: []PrometheusTemplate{
							{
								Name: "test",
								URI:  "http://localhost",
							},
						},
					}, fmt.Sprintf(
						"filepath discovery error: lstat %s: no such file or directory",
						dir,
					)
			},
		},
		{
			description: "matching file produces a server",
			setup: func(t *testing.T) (FilePath, string) {
				t.Helper()
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, "rules.yaml"),
					[]byte("x"), 0o644,
				))
				return FilePath{
					Directory: dir,
					Match:     "(?P<name>.+)\\.yaml",
					Template: []PrometheusTemplate{
						{
							Name: "{{ $name }}",
							URI:  "http://localhost",
						},
					},
				}, ""
			},
			names: []string{"rules"},
		},
		{
			description: "non-matching file produces no servers",
			setup: func(t *testing.T) (FilePath, string) {
				t.Helper()
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, "readme.txt"),
					[]byte("x"), 0o644,
				))
				return FilePath{
					Directory: dir,
					Match:     ".*\\.yaml",
					Template: []PrometheusTemplate{
						{
							Name: "test",
							URI:  "http://localhost",
						},
					},
				}, ""
			},
			names: []string{},
		},
		{
			description: "ignored file produces no servers",
			setup: func(t *testing.T) (FilePath, string) {
				t.Helper()
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, "rules.yaml"),
					[]byte("x"), 0o644,
				))
				return FilePath{
					Directory: dir,
					Match:     ".+\\.yaml",
					Ignore:    []string{"rules\\.yaml"},
					Template: []PrometheusTemplate{
						{
							Name: "test",
							URI:  "http://localhost",
						},
					},
				}, ""
			},
			names: []string{},
		},
		{
			description: "template render error",
			setup: func(t *testing.T) (FilePath, string) {
				t.Helper()
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, "rules.yaml"),
					[]byte("x"), 0o644,
				))
				return FilePath{
					Directory: dir,
					Match:     ".+\\.yaml",
					Template: []PrometheusTemplate{
						{
							Name: "{{ $missing }}",
							URI:  "http://localhost",
						},
					},
				}, `filepath discovery failed to generate Prometheus config from a template: bad name template "{{ $missing }}": template: discovery:1: undefined variable "$missing"`
			},
		},
		{
			description: "nested file produces a server",
			setup: func(t *testing.T) (FilePath, string) {
				t.Helper()
				dir := t.TempDir()
				sub := filepath.Join(dir, "sub")
				require.NoError(t, os.Mkdir(sub, 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(sub, "rules.yaml"),
					[]byte("x"), 0o644,
				))
				return FilePath{
					Directory: dir,
					Match:     "sub/(?P<name>.+)\\.yaml",
					Template: []PrometheusTemplate{
						{
							Name: "{{ $name }}",
							URI:  "http://localhost",
						},
					},
				}, ""
			},
			names: []string{"rules"},
		},
		{
			description: "multiple matching files",
			setup: func(t *testing.T) (FilePath, string) {
				t.Helper()
				dir := t.TempDir()
				for _, name := range []string{"bar.yaml", "baz.yaml", "foo.yaml"} {
					require.NoError(t, os.WriteFile(
						filepath.Join(dir, name),
						[]byte("x"), 0o644,
					))
				}
				return FilePath{
					Directory: dir,
					Match:     "(?P<name>.+)\\.yaml",
					Template: []PrometheusTemplate{
						{
							Name: "{{ $name }}",
							URI:  "http://localhost",
						},
					},
				}, ""
			},
			names: []string{"bar", "baz", "foo"},
		},
		{
			description: "multiple templates per match",
			setup: func(t *testing.T) (FilePath, string) {
				t.Helper()
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, "rules.yaml"),
					[]byte("x"), 0o644,
				))
				return FilePath{
					Directory: dir,
					Match:     "(?P<name>.+)\\.yaml",
					Template: []PrometheusTemplate{
						{
							Name: "{{ $name }}-primary",
							URI:  "http://primary",
						},
						{
							Name: "{{ $name }}-secondary",
							URI:  "http://secondary",
						},
					},
				}, ""
			},
			names: []string{
				"rules-primary",
				"rules-secondary",
			},
		},
		{
			description: "directory is a file",
			setup: func(t *testing.T) (FilePath, string) {
				t.Helper()
				dir := t.TempDir()
				f := filepath.Join(dir, "rules.yaml")
				require.NoError(t, os.WriteFile(f, []byte("x"), 0o644))
				return FilePath{
					Directory: f,
					Match:     "\\.",
					Template: []PrometheusTemplate{
						{
							Name: "test",
							URI:  "http://localhost",
						},
					},
				}, ""
			},
			names: []string{"test"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))
			fp, expectedErr := tc.setup(t)
			servers, err := fp.Discover(context.Background())
			if expectedErr != "" {
				require.EqualError(t, err, expectedErr)
			} else {
				require.NoError(t, err)
				names := make([]string, len(servers))
				for i, s := range servers {
					names[i] = s.Name()
				}
				require.Equal(t, tc.names, names)
			}
		})
	}
}
