package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"go/parser"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/cloudflare/pint/internal/promapi"
)

type TLSConfig struct {
	ServerName         string `hcl:"serverName,optional" json:"serverName,omitempty"`
	CaCert             string `hcl:"caCert,optional" json:"caCert,omitempty"`
	ClientCert         string `hcl:"clientCert,optional" json:"clientCert,omitempty"`
	ClientKey          string `hcl:"clientKey,optional" json:"clientKey,omitempty"`
	InsecureSkipVerify bool   `hcl:"skipVerify,optional" json:"skipVerify,omitempty"`
}

func (t TLSConfig) validate() error {
	if (t.ClientCert != "") != (t.ClientKey != "") {
		return fmt.Errorf("clientCert and clientKey must be set together")
	}
	return nil
}

func (t *TLSConfig) toHTTPConfig() (*tls.Config, error) {
	if t == nil {
		return nil, nil
	}

	var isDirty bool
	cfg := &tls.Config{}

	if t.ServerName != "" {
		cfg.ServerName = t.ServerName
		isDirty = true
	}

	if t.CaCert != "" {
		caCert, err := os.ReadFile(t.CaCert)
		if err != nil {
			return nil, err
		}
		cfg.RootCAs = x509.NewCertPool()
		cfg.RootCAs.AppendCertsFromPEM(caCert)
		isDirty = true
	}

	if t.ClientCert != "" && t.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(t.ClientCert, t.ClientKey)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{cert}
		isDirty = true
	}

	if t.InsecureSkipVerify {
		cfg.InsecureSkipVerify = true
		isDirty = true
	}

	if isDirty {
		return cfg, nil
	}

	return nil, nil
}

type PrometheusConfig struct {
	Headers     map[string]string `hcl:"headers,optional" json:"headers,omitempty"`
	TLS         *TLSConfig        `hcl:"tls,block" json:"tls,omitempty"`
	Name        string            `hcl:",label" json:"name"`
	URI         string            `hcl:"uri" json:"uri"`
	PublicURI   string            `hcl:"publicURI,optional" json:"publicURI,omitempty"`
	Timeout     string            `hcl:"timeout,optional"  json:"timeout"`
	Uptime      string            `hcl:"uptime,optional" json:"uptime"`
	Failover    []string          `hcl:"failover,optional" json:"failover,omitempty"`
	Include     []string          `hcl:"include,optional" json:"include,omitempty"`
	Exclude     []string          `hcl:"exclude,optional" json:"exclude,omitempty"`
	Tags        []string          `hcl:"tags,optional" json:"tags,omitempty"`
	Concurrency int               `hcl:"concurrency,optional" json:"concurrency"`
	RateLimit   int               `hcl:"rateLimit,optional" json:"rateLimit"`
	Required    bool              `hcl:"required,optional" json:"required"`
}

func (pc PrometheusConfig) validate() error {
	if pc.URI == "" {
		return errors.New("prometheus URI cannot be empty")
	}
	if _, err := url.Parse(pc.URI); err != nil {
		return fmt.Errorf("prometheus URI %q is invalid: %w", pc.URI, err)
	}

	if pc.Timeout != "" {
		if _, err := parseDuration(pc.Timeout); err != nil {
			return err
		}
	}

	if pc.Uptime != "" {
		if _, err := parser.ParseExpr(pc.Uptime); err != nil {
			return fmt.Errorf("invalid Prometheus uptime metric selector %q: %w", pc.Uptime, err)
		}
	}

	for _, path := range pc.Include {
		if _, err := regexp.Compile(path); err != nil {
			return err
		}
	}

	for _, path := range pc.Exclude {
		if _, err := regexp.Compile(path); err != nil {
			return err
		}
	}

	for _, tag := range pc.Tags {
		for _, s := range []string{" ", "\n"} {
			if strings.Contains(tag, s) {
				return fmt.Errorf("prometheus tag %q cannot contain %q", tag, s)
			}
		}
	}

	if pc.TLS != nil {
		if err := pc.TLS.validate(); err != nil {
			return err
		}
	}

	return nil
}

func (pc *PrometheusConfig) applyDefaults() {
	if pc.Timeout == "" {
		pc.Timeout = (time.Minute * 2).String()
	}

	if pc.Concurrency <= 0 {
		pc.Concurrency = 16
	}

	if pc.RateLimit <= 0 {
		pc.RateLimit = 100
	}

	if pc.Uptime == "" {
		pc.Uptime = "up"
	}
}

func newFailoverGroup(prom PrometheusConfig) *promapi.FailoverGroup {
	timeout, _ := parseDuration(prom.Timeout)

	var tlsConf *tls.Config
	tlsConf, _ = prom.TLS.toHTTPConfig()
	upstreams := []*promapi.Prometheus{
		promapi.NewPrometheus(prom.Name, prom.URI, prom.PublicURI, prom.Headers, timeout, prom.Concurrency, prom.RateLimit, tlsConf),
	}
	for _, uri := range prom.Failover {
		upstreams = append(upstreams, promapi.NewPrometheus(prom.Name, uri, prom.PublicURI, prom.Headers, timeout, prom.Concurrency, prom.RateLimit, tlsConf))
	}
	include := make([]*regexp.Regexp, 0, len(prom.Include))
	for _, path := range prom.Include {
		include = append(include, strictRegex(path))
	}
	exclude := make([]*regexp.Regexp, 0, len(prom.Exclude))
	for _, path := range prom.Exclude {
		exclude = append(exclude, strictRegex(path))
	}
	tags := make([]string, 0, len(prom.Tags))
	tags = append(tags, prom.Tags...)
	return promapi.NewFailoverGroup(prom.Name, prom.PublicURI, upstreams, prom.Required, prom.Uptime, include, exclude, tags)
}

func NewPrometheusGenerator(cfg Config, metricsRegistry *prometheus.Registry) *PrometheusGenerator {
	return &PrometheusGenerator{
		metricsRegistry: metricsRegistry,
		cfg:             cfg,
	}
}

type PrometheusGenerator struct {
	cfg             Config
	metricsRegistry *prometheus.Registry
	servers         []*promapi.FailoverGroup
}

func (pg *PrometheusGenerator) Servers() []*promapi.FailoverGroup {
	return pg.servers
}

func (pg *PrometheusGenerator) Count() int {
	return len(pg.servers)
}

func (pg *PrometheusGenerator) Stop() {
	for _, server := range pg.servers {
		server.Close(pg.metricsRegistry)
	}
	pg.servers = nil
}

func (pg *PrometheusGenerator) ServersForPath(path string) []*promapi.FailoverGroup {
	var servers []*promapi.FailoverGroup
	for _, server := range pg.servers {
		if server.IsEnabledForPath(path) {
			servers = append(servers, server)
		}
	}
	return servers
}

func (pg *PrometheusGenerator) addServer(server *promapi.FailoverGroup) error {
	for _, s := range pg.servers {
		if s.Name() == server.Name() {
			return fmt.Errorf("Duplicated name for Prometheus server definition: %s", s.Name())
		}
	}
	pg.servers = append(pg.servers, server)
	slog.Info(
		"Configured new Prometheus server",
		slog.String("name", server.Name()),
		slog.Int("uris", server.ServerCount()),
		slog.String("uptime", server.UptimeMetric()),
		slog.Any("tags", server.Tags()),
		slog.Any("include", server.Include()),
		slog.Any("exclude", server.Exclude()),
	)
	server.StartWorkers(pg.metricsRegistry)
	return nil
}

func (pg *PrometheusGenerator) GenerateStatic() (err error) {
	for _, pc := range pg.cfg.Prometheus {
		err = pg.addServer(newFailoverGroup(pc))
		if err != nil {
			return err
		}
	}
	return nil
}

func (pg *PrometheusGenerator) GenerateDynamic(ctx context.Context) (err error) {
	if pg.cfg.Discovery != nil {
		servers, err := pg.cfg.Discovery.Discover(ctx)
		if err != nil {
			return err
		}
		for _, server := range servers {
			err = pg.addServer(server)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
