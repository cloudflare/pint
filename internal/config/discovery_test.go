package config

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/neilotoole/slogt"
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

func TestDiscovery(t *testing.T) {
	type expectedGroupT struct {
		name         string
		uri          string
		uptimeMetric string
		include      []string
		exclude      []string
		tags         []string
		serverCount  int
	}

	type testCaseT struct {
		setup       func(t *testing.T) Discovery
		description string
		expectErr   string
		expect      []expectedGroupT
	}

	testCases := []testCaseT{
		{
			description: "non-existent directory",
			setup: func(_ *testing.T) Discovery {
				return Discovery{FilePath: []FilePath{{
					Directory: "/this/does/not/exist",
					Match:     ".+",
					Template:  []PrometheusTemplate{{Name: "prom", URI: "http://localhost"}},
				}}}
			},
			expectErr: "filepath discovery error: lstat /this/does/not/exist: no such file or directory",
		},
		{
			description: "discovers matching files",
			setup: func(t *testing.T) Discovery {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "prom-1.yaml"), []byte(""), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "prom-2.yaml"), []byte(""), 0o644))
				return Discovery{FilePath: []FilePath{{
					Directory: dir,
					Match:     `prom-(?P<idx>\d+)\.yaml`,
					Template:  []PrometheusTemplate{{Name: "prom-{{ $idx }}", URI: "http://prom-{{ $idx }}"}},
				}}}
			},
			expect: []expectedGroupT{
				{
					name:         "prom-1",
					uri:          "http://prom-1",
					uptimeMetric: "up",
					include:      []string{},
					exclude:      []string{},
					tags:         []string{},
					serverCount:  1,
				},
				{
					name:         "prom-2",
					uri:          "http://prom-2",
					uptimeMetric: "up",
					include:      []string{},
					exclude:      []string{},
					tags:         []string{},
					serverCount:  1,
				},
			},
		},
		{
			description: "ignores non-matching files",
			setup: func(t *testing.T) Discovery {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "prom-1.yaml"), []byte(""), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "other.yaml"), []byte(""), 0o644))
				return Discovery{FilePath: []FilePath{{
					Directory: dir,
					Match:     `prom-(?P<idx>\d+)\.yaml`,
					Template:  []PrometheusTemplate{{Name: "prom-{{ $idx }}", URI: "http://prom-{{ $idx }}"}},
				}}}
			},
			expect: []expectedGroupT{{
				name:         "prom-1",
				uri:          "http://prom-1",
				uptimeMetric: "up",
				include:      []string{},
				exclude:      []string{},
				tags:         []string{},
				serverCount:  1,
			}},
		},
		{
			description: "merges identical servers from different file paths",
			setup: func(t *testing.T) Discovery {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "prom-1.yaml"), []byte(""), 0o644))
				return Discovery{FilePath: []FilePath{
					{
						Directory: dir,
						Match:     `prom-(?P<idx>\d+)\.yaml`,
						Template:  []PrometheusTemplate{{Name: "prom-{{ $idx }}", URI: "http://prom-{{ $idx }}"}},
					},
					{
						Directory: dir,
						Match:     `prom-(?P<idx>\d+)\.yaml`,
						Template:  []PrometheusTemplate{{Name: "prom-{{ $idx }}", URI: "http://prom-{{ $idx }}"}},
					},
				}}
			},
			expect: []expectedGroupT{{
				name:         "prom-1",
				uri:          "http://prom-1",
				uptimeMetric: "up",
				include:      []string{},
				exclude:      []string{},
				tags:         []string{},
				serverCount:  1,
			}},
		},
		{
			description: "keeps servers with different names from different file paths",
			setup: func(t *testing.T) Discovery {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "prom-1.yaml"), []byte(""), 0o644))
				return Discovery{FilePath: []FilePath{
					{
						Directory: dir,
						Match:     `prom-(?P<idx>\d+)\.yaml`,
						Template:  []PrometheusTemplate{{Name: "prom-a-{{ $idx }}", URI: "http://prom-{{ $idx }}"}},
					},
					{
						Directory: dir,
						Match:     `prom-(?P<idx>\d+)\.yaml`,
						Template:  []PrometheusTemplate{{Name: "prom-b-{{ $idx }}", URI: "http://prom-{{ $idx }}"}},
					},
				}}
			},
			expect: []expectedGroupT{
				{
					name:         "prom-a-1",
					uri:          "http://prom-1",
					uptimeMetric: "up",
					include:      []string{},
					exclude:      []string{},
					tags:         []string{},
					serverCount:  1,
				},
				{
					name:         "prom-b-1",
					uri:          "http://prom-1",
					uptimeMetric: "up",
					include:      []string{},
					exclude:      []string{},
					tags:         []string{},
					serverCount:  1,
				},
			},
		},
		{
			description: "file path template render fails with missing key",
			setup: func(t *testing.T) Discovery {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(filepath.Join(dir, "prom-1.yaml"), []byte(""), 0o644))
				return Discovery{FilePath: []FilePath{{
					Directory: dir,
					Match:     `prom-(?P<idx>\d+)\.yaml`,
					Template: []PrometheusTemplate{{
						Name: "prom-{{ .missing_key }}",
						URI:  "http://prom-{{ $idx }}",
					}},
				}}}
			},
			expectErr: `filepath discovery failed to generate Prometheus config from a template: bad name template "prom-{{ .missing_key }}": template: discovery:1:26: executing "discovery" at <.missing_key>: map has no entry for key "missing_key"`,
		},
		{
			description: "discovers servers from Prometheus query",
			setup: func(t *testing.T) Discovery {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"instance":"1"},"value":[1614859502.068,"1"]}]}}`))
				}))
				t.Cleanup(srv.Close)
				return Discovery{PrometheusQuery: []PrometheusQuery{{
					URI:      srv.URL,
					Query:    "up",
					Template: []PrometheusTemplate{{Name: "prom-{{ $instance }}", URI: "http://prom-{{ $instance }}"}},
				}}}
			},
			expect: []expectedGroupT{{
				name:         "prom-1",
				uri:          "http://prom-1",
				uptimeMetric: "up",
				include:      []string{},
				exclude:      []string{},
				tags:         []string{},
				serverCount:  1,
			}},
		},
		{
			description: "Prometheus query fails",
			setup: func(t *testing.T) Discovery {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"status":"error","errorType":"internal","error":"something went wrong"}`))
				}))
				t.Cleanup(srv.Close)
				return Discovery{PrometheusQuery: []PrometheusQuery{{
					URI:      srv.URL,
					Query:    "up",
					Template: []PrometheusTemplate{{Name: "prom", URI: "http://prom"}},
				}}}
			},
			expectErr: "prometheusQuery discovery failed to execute Prometheus query: unknown: something went wrong",
		},
		{
			description: "template render fails with missing key",
			setup: func(t *testing.T) Discovery {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"instance":"1"},"value":[1614859502.068,"1"]}]}}`))
				}))
				t.Cleanup(srv.Close)
				return Discovery{PrometheusQuery: []PrometheusQuery{{
					URI:      srv.URL,
					Query:    "up",
					Template: []PrometheusTemplate{{Name: "prom-{{ .missing_key }}", URI: "http://prom"}},
				}}}
			},
			expectErr: `prometheusQuery discovery  failed to generate Prometheus config from a template: bad name template "prom-{{ .missing_key }}": template: discovery:1:36: executing "discovery" at <.missing_key>: map has no entry for key "missing_key"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			slog.SetDefault(slogt.New(t))

			d := tc.setup(t)
			servers, err := d.Discover(t.Context())
			if tc.expectErr != "" {
				require.EqualError(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			actual := make([]expectedGroupT, 0, len(servers))
			for _, server := range servers {
				actual = append(actual, expectedGroupT{
					name:         server.Name(),
					uri:          server.URI(),
					include:      server.Include(),
					exclude:      server.Exclude(),
					tags:         server.Tags(),
					uptimeMetric: server.UptimeMetric(),
					serverCount:  server.ServerCount(),
				})
			}
			require.Equal(t, tc.expect, actual)
		})
	}
}
