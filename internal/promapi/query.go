package promapi

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

type QueryResult struct {
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
		r := v.(QueryResult)
		return &r, nil
	}

	log.Debug().Str("uri", p.uri).Str("query", expr).Msg("Query started")

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	start := time.Now()
	result, _, err := p.api.Query(ctx, expr, start)
	duration := time.Since(start)
	log.Debug().
		Str("uri", p.uri).
		Str("query", expr).
		Str("duration", HumanizeDuration(duration)).
		Msg("Query completed")
	if err != nil {
		log.Error().Err(err).
			Str("uri", p.uri).
			Str("query", expr).
			Msg("Query failed")
		return nil, err
	}

	qr := QueryResult{
		DurationSeconds: duration.Seconds(),
	}

	switch result.Type() {
	case model.ValVector:
		vectorVal := result.(model.Vector)
		qr.Series = vectorVal
	default:
		log.Error().Err(err).Str("uri", p.uri).Str("query", expr).Msgf("Query returned unknown result type: %v", result.Type())
		return nil, fmt.Errorf("unknown result type: %v", result.Type())
	}
	log.Debug().Str("uri", p.uri).Str("query", expr).Int("series", len(qr.Series)).Msg("Parsed response")

	log.Debug().Str("key", expr).Str("uri", p.uri).Msg("Query cache miss")
	p.cache.Add(expr, qr)

	return &qr, nil
}
