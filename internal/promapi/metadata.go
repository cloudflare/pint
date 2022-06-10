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

type metadataQuery struct {
	prom   *Prometheus
	ctx    context.Context
	metric string
}

func (q metadataQuery) Run() (any, error) {
	log.Debug().
		Str("uri", q.prom.uri).
		Str("metric", q.metric).
		Msg("Getting prometheus metrics metadata")

	ctx, cancel := context.WithTimeout(q.ctx, q.prom.timeout)
	defer cancel()

	v, err := q.prom.api.Metadata(ctx, q.metric, "")
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus metrics metadata: %w", err)
	}
	return v, nil
}

func (q metadataQuery) Endpoint() string {
	return "/api/v1/metadata"
}

func (q metadataQuery) String() string {
	return q.metric
}

func (q metadataQuery) CacheKey() string {
	return hash("/api/v1/metadata/" + q.metric)
}

func (p *Prometheus) Metadata(ctx context.Context, metric string) (*MetadataResult, error) {
	log.Debug().Str("uri", p.uri).Str("metric", metric).Msg("Scheduling Prometheus metrics metadata query")

	resultChan := make(chan queryResult)
	p.queries <- queryRequest{
		query:  metadataQuery{prom: p, ctx: ctx, metric: metric},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	metadata := MetadataResult{URI: p.uri, Metadata: result.value.(map[string][]v1.Metadata)[metric]}

	return &metadata, nil
}
