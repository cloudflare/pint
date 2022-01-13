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

func (p *Prometheus) RangeQuery(expr string, start, end time.Time, step time.Duration) (*RangeQueryResult, error) {

	log.Debug().
		Str("uri", p.uri).
		Str("query", expr).
		Time("start", start).
		Time("end", end).
		Str("step", HumanizeDuration(step)).
		Msg("Scheduling prometheus range query")

	lockKey := "/api/v1/query/range"
	p.lock.Lock(lockKey)
	defer p.lock.Unlock((lockKey))

	cacheKey := strings.Join([]string{expr, start.String(), end.String(), step.String()}, "\n")
	if v, ok := p.cache.Load(cacheKey); ok {
		log.Debug().Str("key", cacheKey).Str("uri", p.uri).Msg("Range query cache hit")
		r := v.(RangeQueryResult)
		return &r, nil
	}

	log.Debug().Str("uri", p.uri).Str("query", expr).Msg("Range query started")

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	r := v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}
	qstart := time.Now()
	result, _, err := p.api.QueryRange(ctx, expr, r)
	duration := time.Since(qstart)
	log.Debug().
		Str("uri", p.uri).
		Str("query", expr).
		Str("duration", HumanizeDuration(duration)).
		Msg("Range query completed")
	if err != nil {
		log.Error().Err(err).Str("uri", p.uri).Str("query", expr).Msg("Range query failed")
		if isRetryable(err) {
			delta := end.Sub(start) / 2
			log.Warn().Str("delta", HumanizeDuration(delta)).Msg("Retrying request with smaller range")
			return p.RangeQuery(expr, start.Add(delta), end, step)
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
		log.Error().Err(err).Str("uri", p.uri).Str("query", expr).Msgf("Range query returned unknown result type: %v", result)
		return nil, fmt.Errorf("unknown result type: %v", result)
	}
	log.Debug().Str("uri", p.uri).Str("query", expr).Int("samples", len(qr.Samples)).Msg("Parsed range response")

	log.Debug().Str("key", cacheKey).Str("uri", p.uri).Msg("Range query cache miss")
	p.cache.Store(cacheKey, qr)

	return &qr, nil
}

func isRetryable(err error) bool {
	var neterr net.Error
	if ok := errors.As(err, &neterr); ok && neterr.Timeout() {
		return true
	}
	if strings.Contains(err.Error(), "query processing would load too many samples into memory in ") {
		return true
	}
	return false
}
