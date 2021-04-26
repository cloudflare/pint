package promapi

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/prometheus/client_golang/api"
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

func RangeQuery(uri string, timeout time.Duration, expr string, start, end time.Time, step time.Duration, lockKey *string) (*RangeQueryResult, error) {
	key := uri
	if lockKey != nil {
		key = *lockKey
	}

	log.Debug().
		Str("key", key).
		Str("uri", uri).
		Str("query", expr).
		Time("start", start).
		Time("end", end).
		Str("step", HumanizeDuration(step)).
		Msg("Scheduling prometheus range query")

	km.Lock(key)
	defer km.Unlock((key))

	log.Debug().Str("uri", uri).Str("query", expr).Msg("Range query started")

	client, err := api.NewClient(api.Config{Address: uri})
	if err != nil {
		log.Error().Err(err).Msg("Failed to setup new Prometheus API client")
		return nil, err
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	r := v1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}
	qstart := time.Now()
	result, _, err := v1api.QueryRange(ctx, expr, r)
	duration := time.Since(qstart)
	log.Debug().
		Str("uri", uri).
		Str("query", expr).
		Str("duration", HumanizeDuration(duration)).
		Msg("Range query completed")
	if err != nil {
		log.Error().Err(err).Str("uri", uri).Str("query", expr).Msg("Range query failed")
		if err, ok := err.(net.Error); ok && err.Timeout() {
			delta := end.Sub(start) / 2
			log.Warn().Str("delta", HumanizeDuration(delta)).Msg("Retrying request with smaller range")
			b, _ := start.MarshalText()
			newKey := fmt.Sprintf("%s/retry/%s", key, string(b))
			return RangeQuery(uri, timeout, expr, start.Add(delta), end, step, &newKey)
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

	case model.ValString:
		fmt.Println("ValString")
	default:
		log.Error().Err(err).Str("uri", uri).Str("query", expr).Msgf("Range query returned unknown result type: %v", result)
		return nil, fmt.Errorf("unknown result type: %v", result)
	}
	log.Debug().Str("uri", uri).Str("query", expr).Int("samples", len(qr.Samples)).Msg("Parsed range response")

	return &qr, nil
}
