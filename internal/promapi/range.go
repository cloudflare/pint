package promapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

type RangeQueryResult struct {
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
		Str("step", HumanizeDuration(step)).
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
		Str("duration", HumanizeDuration(duration)).
		Msg("Range query completed")
	if err != nil {
		log.Error().Err(err).Str("uri", p.uri).Str("query", expr).Msg("Range query failed")
		if delta, retryOK := canRetry(err, end.Sub(start)); retryOK {
			if delta < step*2 {
				log.Error().Str("uri", p.uri).Str("query", expr).Msg("No more retries possible")
				return nil, errors.New("no more retries possible")
			}
			log.Warn().Str("delta", HumanizeDuration(delta)).Msg("Retrying request with smaller range")
			return p.RangeQuery(ctx, expr, start.Add(delta), end, step)
		}
		return nil, err
	}

	qr := RangeQueryResult{
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
		return nil, fmt.Errorf("unknown result type: %v", result.Type())
	}
	log.Debug().Str("uri", p.uri).Str("query", expr).Int("samples", len(qr.Samples)).Msg("Parsed range response")

	log.Debug().Str("query", expr).Str("uri", p.uri).Msg("Range query cache miss")
	p.cache.Add(cacheKey, qr)

	return &qr, nil
}

func canRetry(err error, delta time.Duration) (time.Duration, bool) {
	var neterr net.Error
	if ok := errors.As(err, &neterr); ok && neterr.Timeout() {
		return delta / 2, true
	}
	if strings.Contains(err.Error(), "query processing would load too many samples into memory in ") {
		return (delta / 4) * 3, true
	}
	return delta, false
}
