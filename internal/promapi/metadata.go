package promapi

import (
	"context"
	"fmt"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/rs/zerolog/log"
)

type MetadataResult struct {
	URI      string
	Metadata []v1.Metadata
}

func (p *Prometheus) Metadata(ctx context.Context, metric string) (*MetadataResult, error) {
	log.Debug().Str("uri", p.uri).Str("metric", metric).Msg("Query Prometheus metric metadata")

	key := fmt.Sprintf("/api/v1/metadata/%s", metric)
	p.lock.lock(key)
	defer p.lock.unlock((key))

	if v, ok := p.cache.Get(key); ok {
		log.Debug().Str("key", key).Str("uri", p.uri).Str("metric", metric).Msg("Metric metadata cache hit")
		prometheusCacheHitsTotal.WithLabelValues(p.name, "/api/v1/metadata").Inc()
		metadata := v.(MetadataResult)
		return &metadata, nil
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	prometheusQueriesTotal.WithLabelValues(p.name, "/api/v1/metadata").Inc()
	resp, err := p.api.Metadata(ctx, metric, "")
	if err != nil {
		log.Error().Err(err).Str("uri", p.uri).Msg("Failed to query Prometheus metric metadata")
		prometheusQueryErrorsTotal.WithLabelValues(p.name, "/api/v1/metadata", errReason(err)).Inc()
		return nil, fmt.Errorf("failed to query Prometheus metric metadata: %w", err)
	}

	metadata := MetadataResult{URI: p.uri, Metadata: resp[metric]}

	log.Debug().Str("key", key).Str("uri", p.uri).Str("metric", metric).Msg("Metric metadata cache miss")
	p.cache.Add(key, metadata)

	return &metadata, nil
}
