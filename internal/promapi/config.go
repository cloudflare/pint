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

func (p *Prometheus) Config(ctx context.Context) (*PrometheusConfig, error) {
	log.Debug().Str("uri", p.uri).Msg("Query Prometheus configuration")

	key := "/api/v1/status/config"
	p.lock.Lock(key)
	defer p.lock.Unlock((key))

	if v, ok := p.cache.Get(key); ok {
		log.Debug().Str("key", key).Str("uri", p.uri).Msg("Config cache hit")
		cfg := v.(PrometheusConfig)
		return &cfg, nil
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	resp, err := p.api.Config(ctx)
	if err != nil {
		log.Error().Err(err).Str("uri", p.uri).Msg("Failed to query Prometheus configuration")
		return nil, fmt.Errorf("failed to query Prometheus config: %w", err)
	}

	var cfg PrometheusConfig
	if err = yaml.Unmarshal([]byte(resp.YAML), &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config data in %s response: %w", key, err)
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

	log.Debug().Str("key", key).Str("uri", p.uri).Msg("Config cache miss")
	p.cache.Add(key, cfg)

	return &cfg, nil
}
