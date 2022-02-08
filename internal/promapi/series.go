package promapi

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

type SeriesResult struct {
	Series model.Vector
}

func (p *Prometheus) Series(ctx context.Context, matches []string) ([]model.LabelSet, error) {
	log.Debug().Str("uri", p.uri).Strs("matches", matches).Msg("Scheduling prometheus series query")

	lockKey := strings.Join(matches, ",")
	p.lock.lock(lockKey)
	defer p.lock.unlock((lockKey))

	if v, ok := p.cache.Get(lockKey); ok {
		log.Debug().Str("key", lockKey).Str("uri", p.uri).Msg("Query cache hit")
		r := v.([]model.LabelSet)
		return r, nil
	}

	log.Debug().Str("uri", p.uri).Strs("matches", matches).Msg("Query started")

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	start := time.Now()
	result, _, err := p.api.Series(ctx, matches, start.Add(time.Minute*-5), start)
	duration := time.Since(start)
	log.Debug().
		Str("uri", p.uri).
		Strs("matches", matches).
		Str("duration", HumanizeDuration(duration)).
		Msg("Query completed")
	if err != nil {
		log.Error().Err(err).
			Str("uri", p.uri).
			Strs("matches", matches).
			Msg("Query failed")
		return nil, err
	}

	log.Debug().Str("uri", p.uri).Strs("matches", matches).Int("series", len(result)).Msg("Parsed response")

	log.Debug().Str("key", lockKey).Str("uri", p.uri).Msg("Query cache miss")
	p.cache.Add(lockKey, result)

	return result, nil
}
