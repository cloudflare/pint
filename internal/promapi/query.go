package promapi

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudflare/pint/internal/keylock"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

var km = keylock.NewPartitionLocker((&sync.Mutex{}))

type QueryResult struct {
	Series          model.Vector
	DurationSeconds float64
}

func Query(uri string, timeout time.Duration, expr string, lockKey *string) (*QueryResult, error) {
	log.Debug().Str("uri", uri).Str("query", expr).Msg("Scheduling prometheus query")
	key := uri
	if lockKey != nil {
		key = *lockKey
	}
	km.Lock(key)
	defer km.Unlock((key))

	log.Debug().Str("uri", uri).Str("query", expr).Msg("Query started")

	client, err := api.NewClient(api.Config{Address: uri})
	if err != nil {
		log.Error().Err(err).Msg("Failed to setup new Prometheus API client")
		return nil, err
	}

	v1api := v1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	result, _, err := v1api.Query(ctx, expr, start)
	duration := time.Since(start)
	log.Debug().
		Str("uri", uri).
		Str("query", expr).
		Str("duration", HumanizeDuration(duration)).
		Msg("Query completed")
	if err != nil {
		log.Error().Err(err).
			Str("uri", uri).
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
		log.Error().Err(err).Str("uri", uri).Str("query", expr).Msgf("Query returned unknown result type: %v", result)
		return nil, fmt.Errorf("unknown result type: %v", result)
	}
	log.Debug().Str("uri", uri).Str("query", expr).Int("series", len(qr.Series)).Msg("Parsed response")

	return &qr, nil
}
