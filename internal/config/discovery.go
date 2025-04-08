package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

type Discoverer interface {
	Discover(_ context.Context) ([]*promapi.FailoverGroup, error)
}

func isEqualFailoverGroup(a, b *promapi.FailoverGroup) bool {
	if a.Name() != b.Name() {
		return false
	}
	if !slices.Equal(a.Include(), b.Include()) {
		slog.Warn(
			"Duplicated prometheus server with different include",
			slog.String("name", a.Name()),
			slog.Any("a", a.Include()),
			slog.Any("b", b.Include()),
		)
		return false
	}
	if !slices.Equal(a.Exclude(), b.Exclude()) {
		slog.Warn(
			"Duplicated prometheus server with different exclude",
			slog.String("name", a.Name()),
			slog.Any("a", a.Exclude()),
			slog.Any("b", b.Exclude()),
		)
		return false
	}
	if !slices.Equal(a.Tags(), b.Tags()) {
		slog.Warn(
			"Duplicated prometheus server with different tags",
			slog.String("name", a.Name()),
			slog.Any("a", a.Tags()),
			slog.Any("b", b.Tags()),
		)
		return false
	}
	return true
}

type Discovery struct {
	FilePath        []FilePath        `hcl:"filepath,block"        json:"filepath,omitempty"`
	PrometheusQuery []PrometheusQuery `hcl:"prometheusQuery,block" json:"prometheusQuery,omitempty"`
}

func (d Discovery) validate() (err error) {
	for _, fp := range d.FilePath {
		if err = fp.validate(); err != nil {
			return err
		}
	}
	for _, pq := range d.PrometheusQuery {
		if err = pq.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (d *Discovery) discover(
	ctx context.Context,
	pd Discoverer,
	servers []*promapi.FailoverGroup,
) ([]*promapi.FailoverGroup, error) {
	ds, err := pd.Discover(ctx)
	if err != nil {
		return nil, err
	}
	return d.merge(servers, ds)
}

func (d *Discovery) Discover(ctx context.Context) ([]*promapi.FailoverGroup, error) {
	var err error
	servers := []*promapi.FailoverGroup{}
	for _, pd := range d.FilePath {
		servers, err = d.discover(ctx, pd, servers)
		if err != nil {
			return nil, err
		}
	}
	for _, pd := range d.PrometheusQuery {
		servers, err = d.discover(ctx, pd, servers)
		if err != nil {
			return nil, err
		}
	}
	return servers, nil
}

func (d *Discovery) merge(dst, src []*promapi.FailoverGroup) ([]*promapi.FailoverGroup, error) {
	for _, ns := range src {
		var found bool
		for _, ol := range dst {
			if isEqualFailoverGroup(ns, ol) {
				found = true
				ol.MergeUpstreams(ns)
			}
		}
		if !found {
			dst = append(dst, ns)
		}
	}
	return dst, nil
}

type PrometheusTemplate struct {
	Headers     map[string]string `hcl:"headers,optional"     json:"headers,omitempty"`
	TLS         *TLSConfig        `hcl:"tls,block"            json:"tls,omitempty"`
	Name        string            `hcl:"name"                 json:"name"`
	URI         string            `hcl:"uri"                  json:"uri"`
	PublicURI   string            `hcl:"publicURI,optional"   json:"publicURI,omitempty"`
	Timeout     string            `hcl:"timeout,optional"     json:"timeout"`
	Uptime      string            `hcl:"uptime,optional"      json:"uptime"`
	Failover    []string          `hcl:"failover,optional"    json:"failover,omitempty"`
	Include     []string          `hcl:"include,optional"     json:"include,omitempty"`
	Exclude     []string          `hcl:"exclude,optional"     json:"exclude,omitempty"`
	Tags        []string          `hcl:"tags,optional"        json:"tags,omitempty"`
	Concurrency int               `hcl:"concurrency,optional" json:"concurrency"`
	RateLimit   int               `hcl:"rateLimit,optional"   json:"rateLimit"`
	Required    bool              `hcl:"required,optional"    json:"required"`
}

func (pt PrometheusTemplate) validate() (err error) {
	if pt.Name == "" {
		return errors.New("prometheus template name cannot be empty")
	}

	if pt.URI == "" {
		return errors.New("prometheus template URI cannot be empty")
	}

	if pt.Timeout != "" {
		if _, err = parseDuration(pt.Timeout); err != nil {
			return err
		}
	}

	if pt.TLS != nil {
		if err := pt.TLS.validate(); err != nil {
			return err
		}
	}

	return nil
}

func (pt PrometheusTemplate) Render(data map[string]string) (*promapi.FailoverGroup, error) {
	var err error
	var name, uri, publicURI string
	if name, err = renderTemplate(pt.Name, data); err != nil {
		return nil, fmt.Errorf("bad name template %q: %w", pt.Name, err)
	}
	if uri, err = renderTemplate(pt.URI, data); err != nil {
		return nil, fmt.Errorf("bad uri template %q: %w", pt.URI, err)
	}
	if pt.PublicURI != "" {
		if publicURI, err = renderTemplate(pt.PublicURI, data); err != nil {
			return nil, fmt.Errorf("bad publicURI template %q: %w", pt.PublicURI, err)
		}
	} else {
		publicURI = uri
	}

	failover := make([]string, 0, len(pt.Failover))
	var furi string
	for _, f := range pt.Failover {
		furi, err = renderTemplate(f, data)
		if err != nil {
			return nil, fmt.Errorf("bad failover template %q: %w", f, err)
		}
		failover = append(failover, furi)
	}

	headerNames := make([]string, 0, len(pt.Headers))
	headers := make(map[string]string, len(pt.Headers))
	var key, val string
	for k, v := range pt.Headers {
		key, err = renderTemplate(k, data)
		if err != nil {
			return nil, fmt.Errorf("bad header key template %q: %w", k, err)
		}
		val, err = renderTemplate(v, data)
		if err != nil {
			return nil, fmt.Errorf("bad header value template %q: %w", v, err)
		}
		headerNames = append(headerNames, key)
		headers[key] = val
	}

	include := make([]string, 0, len(pt.Include))
	var inc string
	for _, t := range pt.Include {
		inc, err = renderTemplate(t, data)
		if err != nil {
			return nil, fmt.Errorf("bad include template %q: %w", t, err)
		}
		include = append(include, inc)
	}

	exclude := make([]string, 0, len(pt.Exclude))
	var exc string
	for _, t := range pt.Exclude {
		exc, err = renderTemplate(t, data)
		if err != nil {
			return nil, fmt.Errorf("bad exclude template %q: %w", t, err)
		}
		exclude = append(exclude, exc)
	}

	tags := make([]string, 0, len(pt.Tags))
	var tag string
	for _, t := range pt.Tags {
		tag, err = renderTemplate(t, data)
		if err != nil {
			return nil, fmt.Errorf("bad tag template %q: %w", t, err)
		}
		tags = append(tags, tag)
	}

	prom := PrometheusConfig{
		Name:        name,
		URI:         strings.TrimSuffix(uri, "/"),
		PublicURI:   strings.TrimSuffix(publicURI, "/"),
		Headers:     headers,
		Failover:    failover,
		Timeout:     pt.Timeout,
		Concurrency: pt.Concurrency,
		RateLimit:   pt.RateLimit,
		Uptime:      pt.Uptime,
		Include:     include,
		Exclude:     exclude,
		Tags:        tags,
		Required:    pt.Required,
		TLS:         pt.TLS,
	}
	prom.applyDefaults()
	if err = prom.validate(); err != nil {
		return nil, err
	}

	slog.Debug(
		"Rendered Prometheus server",
		slog.String("name", prom.Name),
		slog.String("uri", prom.URI),
		slog.Any("headers", headerNames),
		slog.String("timeout", prom.Timeout),
		slog.Int("concurrency", prom.Concurrency),
		slog.Int("rateLimit", prom.RateLimit),
		slog.String("uptime", prom.Uptime),
		slog.Any("tags", prom.Tags),
		slog.Bool("required", prom.Required),
	)

	return newFailoverGroup(prom), nil
}

type FilePath struct {
	Directory string               `hcl:"directory"       json:"directory"`
	Match     string               `hcl:"match"           json:"match"`
	Ignore    []string             `hcl:"ignore,optional" json:"ignore,omitempty"`
	Template  []PrometheusTemplate `hcl:"template,block"  json:"template"`
}

func (fp FilePath) validate() (err error) {
	if _, err = regexp.Compile(fp.Match); err != nil {
		return err
	}
	for _, pattern := range fp.Ignore {
		if _, err = regexp.Compile(pattern); err != nil {
			return err
		}
	}
	if len(fp.Template) == 0 {
		return errors.New("prometheusQuery discovery requires at least one template")
	}
	for _, pt := range fp.Template {
		if err = pt.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (fp FilePath) isIgnored(path string) bool {
	for _, pattern := range fp.Ignore {
		if strictRegex(pattern).MatchString(path) {
			return true
		}
	}
	return false
}

func (fp FilePath) Discover(_ context.Context) ([]*promapi.FailoverGroup, error) {
	re := strictRegex(fp.Match)
	servers := []*promapi.FailoverGroup{}
	slog.Info(
		"Finding Prometheus servers using file paths",
		slog.String("dir", fp.Directory),
		slog.String("match", re.String()),
	)
	err := filepath.WalkDir(fp.Directory,
		func(path string, _ fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("filepath discovery error: %w", err)
			}
			path, err = filepath.Rel(fp.Directory, path)
			if err != nil {
				return fmt.Errorf("filepath discovery error: %w", err)
			}
			if fp.isIgnored(path) {
				return nil
			}
			if re.MatchString(path) {
				slog.Debug(
					"Path discovery match",
					slog.String("match", re.String()),
					slog.String("path", path),
				)
				data := findNamedMatches(re, path)
				for _, t := range fp.Template {
					server, err := t.Render(data)
					if err != nil {
						return fmt.Errorf(
							"filepath discovery failed to generate Prometheus config from a template: %w",
							err,
						)
					}
					servers = append(servers, server)
				}
			}
			return nil
		})

	return servers, err
}

type PrometheusQuery struct {
	URI      string               `hcl:"uri"              json:"uri"`
	Headers  map[string]string    `hcl:"headers,optional" json:"headers,omitempty"`
	Timeout  string               `hcl:"timeout,optional" json:"timeout"`
	TLS      *TLSConfig           `hcl:"tls,block"        json:"tls,omitempty"`
	Query    string               `hcl:"query"            json:"query"`
	Template []PrometheusTemplate `hcl:"template,block"   json:"template"`
}

func (pq PrometheusQuery) validate() (err error) {
	if pq.Timeout != "" {
		if _, err = parseDuration(pq.Timeout); err != nil {
			return err
		}
	}
	if pq.TLS != nil {
		if err = pq.TLS.validate(); err != nil {
			return err
		}
	}
	if _, err = parser.DecodeExpr(pq.Query); err != nil {
		return fmt.Errorf("failed to parse prometheus query %q: %w", pq.Query, err)
	}
	if len(pq.Template) == 0 {
		return errors.New("prometheusQuery discovery requires at least one template")
	}
	for _, t := range pq.Template {
		if err = t.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (pq PrometheusQuery) Discover(ctx context.Context) ([]*promapi.FailoverGroup, error) {
	if pq.Timeout == "" {
		pq.Timeout = (time.Minute * 2).String()
	}

	timeout, _ := parseDuration(pq.Timeout)
	tls, _ := pq.TLS.toHTTPConfig()

	prom := promapi.NewPrometheus("discovery", pq.URI, "", pq.Headers, timeout, 1, 100, tls)
	prom.StartWorkers()
	defer prom.Close()

	slog.Info(
		"Finding Prometheus servers using Prometheus API query",
		slog.String("uri", prom.SafeURI()),
		slog.String("query", pq.Query),
	)
	res, err := prom.Query(ctx, pq.Query)
	if err != nil {
		return nil, fmt.Errorf(
			"prometheusQuery discovery failed to execute Prometheus query: %w",
			err,
		)
	}

	servers := []*promapi.FailoverGroup{}
	for _, s := range res.Series {
		for _, t := range pq.Template {
			server, err := t.Render(s.Labels.Map())
			if err != nil {
				return nil, fmt.Errorf(
					"prometheusQuery discovery  failed to generate Prometheus config from a template: %w",
					err,
				)
			}
			servers = append(servers, server)
		}
	}

	return servers, nil
}

func formatAliases(data map[string]string, t string) string {
	var vars strings.Builder
	for k := range data {
		vars.WriteString(fmt.Sprintf("{{ $%s := .%s }}", k, k))
	}
	vars.WriteString(t)
	return vars.String()
}

func renderTemplate(t string, data map[string]string) (string, error) {
	tmpl, err := template.New("discovery").Parse(formatAliases(data, t))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Option("missingkey=error").Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func findNamedMatches(re *regexp.Regexp, str string) map[string]string {
	names := re.SubexpNames()
	match := re.FindStringSubmatch(str)
	results := map[string]string{}
	for i, val := range match[1:] {
		key := names[i+1]
		if key != "" {
			results[key] = val
		}
	}
	slog.Debug(
		"Extracted regexp variables",
		slog.String("regexp", re.String()),
		slog.Any("vars", results),
	)
	return results
}
