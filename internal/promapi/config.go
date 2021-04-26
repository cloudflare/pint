package promapi

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/api"
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

func Config(uri string, timeout time.Duration) (*PrometheusConfig, error) {
	log.Debug().Str("uri", uri).Msg("Query Prometheus configuration")

	client, err := api.NewClient(api.Config{Address: uri})
	if err != nil {
		return nil, err
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := v1api.Config(ctx)
	if err != nil {
		log.Error().Err(err).Str("uri", uri).Msg("Failed to query Prometheus configuration")
		return nil, fmt.Errorf("failed to query Prometheus config: %v", err)
	}

	var cfg PrometheusConfig
	if err = yaml.Unmarshal([]byte(resp.YAML), &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config data in /api/v1/status/config response: %s", err)
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

	return &cfg, nil
}
