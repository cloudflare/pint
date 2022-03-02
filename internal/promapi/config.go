package promapi

import (
	"context"
	"fmt"
	"time"

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

func (p *Prometheus) Config(ctx context.Context) (*ConfigResult, error) {
	log.Debug().Str("uri", p.uri).Msg("Query Prometheus configuration")

	key := "/api/v1/status/config"
	p.lock.lock(key)
	defer p.lock.unlock((key))

	if v, ok := p.cache.Get(key); ok {
		log.Debug().Str("key", key).Str("uri", p.uri).Msg("Config cache hit")
		cfg := v.(ConfigResult)
		return &cfg, nil
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	prometheusQueriesTotal.WithLabelValues(p.name, "/api/v1/status/config")
	resp, err := p.api.Config(ctx)
	if err != nil {
		log.Error().Err(err).Str("uri", p.uri).Msg("Failed to query Prometheus configuration")
		prometheusQueryErrorsTotal.WithLabelValues(p.name, "/api/v1/status/config", errReason(err)).Inc()
		return nil, fmt.Errorf("failed to query Prometheus config: %w", err)
	}

	var cfg PrometheusConfig
	if err = yaml.Unmarshal([]byte(resp.YAML), &cfg); err != nil {
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

	log.Debug().Str("key", key).Str("uri", p.uri).Msg("Config cache miss")
	p.cache.Add(key, r)

	return &r, nil
}
