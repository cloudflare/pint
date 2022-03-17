package promapi

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"

	"github.com/cloudflare/pint/internal/output"
)

type QueryResult struct {
	URI             string
	Series          model.Vector
	DurationSeconds float64
}

func (p *Prometheus) Query(ctx context.Context, expr string) (*QueryResult, error) {
	log.Debug().Str("uri", p.uri).Str("query", expr).Msg("Scheduling prometheus query")

	lockKey := expr
	p.lock.lock(lockKey)
	defer p.lock.unlock((lockKey))

	if v, ok := p.cache.Get(expr); ok {
		log.Debug().Str("key", expr).Str("uri", p.uri).Msg("Query cache hit")
		prometheusCacheHitsTotal.WithLabelValues(p.name, "/api/v1/query").Inc()
		r := v.(QueryResult)
		return &r, nil
	}

	log.Debug().Str("uri", p.uri).Str("query", expr).Msg("Query started")

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	prometheusQueriesTotal.WithLabelValues(p.name, "/api/v1/query").Inc()
	start := time.Now()
	result, _, err := p.api.Query(ctx, expr, start)
	duration := time.Since(start)
	log.Debug().
		Str("uri", p.uri).
		Str("query", expr).
		Str("duration", output.HumanizeDuration(duration)).
		Msg("Query completed")
	if err != nil {
		log.Error().Err(err).
			Str("uri", p.uri).
			Str("query", expr).
			Msg("Query failed")
		prometheusQueryErrorsTotal.WithLabelValues(p.name, "/api/v1/query", errReason(err)).Inc()
		return nil, err
	}

	qr := QueryResult{
		URI:             p.uri,
		DurationSeconds: duration.Seconds(),
	}

	switch result.Type() {
	case model.ValVector:
		vectorVal := result.(model.Vector)
		qr.Series = vectorVal
	default:
		log.Error().Str("uri", p.uri).Str("query", expr).Msgf("Query returned unknown result type: %v", result.Type())
		prometheusQueryErrorsTotal.WithLabelValues(p.name, "/api/v1/query", "unknown result type").Inc()
		return nil, fmt.Errorf("unknown result type: %v", result.Type())
	}
	log.Debug().Str("uri", p.uri).Str("query", expr).Int("series", len(qr.Series)).Msg("Parsed response")

	log.Debug().Str("key", expr).Str("uri", p.uri).Msg("Query cache miss")
	p.cache.Add(expr, qr)

	return &qr, nil
}
