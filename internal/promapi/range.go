package promapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"

	"github.com/cloudflare/pint/internal/output"
)

type RangeQueryResult struct {
	URI             string
	Samples         []*model.SampleStream
	Start           time.Time
	End             time.Time
	DurationSeconds float64
}

func (p *Prometheus) RangeQuery(ctx context.Context, expr string, start, end time.Time, step time.Duration) (*RangeQueryResult, error) {
	log.Debug().
		Str("uri", p.uri).
		Str("query", expr).
		Time("start", start).
		Time("end", end).
		Str("step", output.HumanizeDuration(step)).
		Msg("Scheduling prometheus range query")

	lockKey := "/api/v1/query/range"
	p.lock.lock(lockKey)

	cacheKey := strings.Join([]string{expr, start.String(), end.String(), step.String()}, "\n")
	if v, ok := p.cache.Get(cacheKey); ok {
		log.Debug().Str("key", cacheKey).Str("uri", p.uri).Msg("Range query cache hit")
		r := v.(RangeQueryResult)
		p.lock.unlock((lockKey))
		return &r, nil
	}

	rctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	prometheusQueriesTotal.WithLabelValues(p.name, "/api/v1/query_range").Inc()
	r := v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}
	qstart := time.Now()
	result, _, err := p.api.QueryRange(rctx, expr, r)
	duration := time.Since(qstart)
	p.lock.unlock((lockKey))
	log.Debug().
		Str("uri", p.uri).
		Str("query", expr).
		Str("duration", output.HumanizeDuration(duration)).
		Msg("Range query completed")
	if err != nil {
		log.Error().Err(err).Str("uri", p.uri).Str("query", expr).Msg("Range query failed")
		prometheusQueryErrorsTotal.WithLabelValues(p.name, "/api/v1/query_range", errReason(err)).Inc()
		if delta, retryOK := CanRetryError(err, end.Sub(start)); retryOK {
			if delta < step*2 {
				log.Error().Str("uri", p.uri).Str("query", expr).Msg("No more retries possible")
				return nil, errors.New("no more retries possible")
			}
			log.Warn().Str("delta", output.HumanizeDuration(delta)).Msg("Retrying request with smaller range")
			return p.RangeQuery(ctx, expr, start.Add(delta), end, step)
		}
		return nil, err
	}

	qr := RangeQueryResult{
		URI:             p.uri,
		DurationSeconds: duration.Seconds(),
		Start:           start,
		End:             end,
	}

	switch result.Type() {
	case model.ValMatrix:
		samples := result.(model.Matrix)
		qr.Samples = samples
	default:
		log.Error().Err(err).Str("uri", p.uri).Str("query", expr).Msgf("Range query returned unknown result type: %v", result.Type())
		prometheusQueryErrorsTotal.WithLabelValues(p.name, "/api/v1/query_range", "unknown result type").Inc()
		return nil, fmt.Errorf("unknown result type: %v", result.Type())
	}
	log.Debug().Str("uri", p.uri).Str("query", expr).Int("samples", len(qr.Samples)).Msg("Parsed range response")

	log.Debug().Str("query", expr).Str("uri", p.uri).Msg("Range query cache miss")
	p.cache.Add(cacheKey, qr)

	return &qr, nil
}
