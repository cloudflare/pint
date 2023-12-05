package config

import (
	"log/slog"
	"strconv"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryConfig(t *testing.T) {
	type testCaseT struct {
		conf Discovery
		err  string
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
			err: `failed to parse prometheus query "foo{": unexpected end of input inside braces`,
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
		template PrometheusTemplate
		data     map[string]string
		err      string
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
