package promapi

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type ConfigSectionGlobal struct {
	ScrapeInterval     time.Duration     `yaml:"scrape_interval"`
	ScrapeTimeout      time.Duration     `yaml:"scrape_timeout"`
	EvaluationInterval time.Duration     `yaml:"evaluation_interval"`
	ExternalLabels     map[string]string `yaml:"external_labels"`
}

type PrometheusConfig struct {
	Global ConfigSectionGlobal `yaml:"global"`
}

type ConfigResult struct {
	URI    string
	Config PrometheusConfig
}

type configQuery struct {
	prom *Prometheus
	ctx  context.Context
}

func (q configQuery) Run() (any, error) {
	log.Debug().
		Str("uri", q.prom.uri).
		Msg("Getting prometheus configuration")

	ctx, cancel := context.WithTimeout(q.ctx, q.prom.timeout)
	defer cancel()

	v, err := q.prom.api.Config(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus config: %w", err)
	}
	return v, nil
}

func (q configQuery) Endpoint() string {
	return "/api/v1/status/config"
}

func (q configQuery) String() string {
	return "/api/v1/status/config"
}

func (q configQuery) CacheKey() string {
	return hash("/api/v1/status/config")
}

func (p *Prometheus) Config(ctx context.Context) (*ConfigResult, error) {
	log.Debug().Str("uri", p.uri).Msg("Scheduling Prometheus configuration query")

	resultChan := make(chan queryResult)
	p.queries <- queryRequest{
		query:  configQuery{prom: p, ctx: ctx},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	var cfg PrometheusConfig
	if err := yaml.Unmarshal([]byte(result.value.(v1.ConfigResult).YAML), &cfg); err != nil {
		prometheusQueryErrorsTotal.WithLabelValues(p.name, "/api/v1/status/config", errReason(err)).Inc()
		return nil, fmt.Errorf("failed to decode config data in %s response: %w", p.uri, err)
	}

	if cfg.Global.ScrapeInterval == 0 {
		cfg.Global.ScrapeInterval = time.Minute
	}
	if cfg.Global.ScrapeTimeout == 0 {
		cfg.Global.ScrapeTimeout = time.Second * 10
	}
	if cfg.Global.EvaluationInterval == 0 {
		cfg.Global.EvaluationInterval = time.Minute
	}

	r := ConfigResult{URI: p.uri, Config: cfg}

	return &r, nil
}
