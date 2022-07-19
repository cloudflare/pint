package promapi

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/rs/zerolog/log"
)

type FlagsResult struct {
	URI   string
	Flags v1.FlagsResult
}

type flagsQuery struct {
	prom      *Prometheus
	ctx       context.Context
	timestamp time.Time
}

func (q flagsQuery) Run() (any, error) {
	log.Debug().
		Str("uri", q.prom.uri).
		Msg("Getting prometheus flags")

	ctx, cancel := context.WithTimeout(q.ctx, q.prom.timeout)
	defer cancel()

	v, err := q.prom.api.Flags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus flags: %w", err)
	}
	return v, nil
}

func (q flagsQuery) Endpoint() string {
	return "/api/v1/status/flags"
}

func (q flagsQuery) String() string {
	return "/api/v1/status/flags"
}

func (q flagsQuery) CacheKey() string {
	h := sha1.New()
	_, _ = io.WriteString(h, q.Endpoint())
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, q.timestamp.Round(cacheExpiry).Format(time.RFC3339))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Prometheus) Flags(ctx context.Context) (*FlagsResult, error) {
	log.Debug().Str("uri", p.uri).Msg("Scheduling Prometheus flags query")

	key := "/api/v1/status/flags"
	p.locker.lock(key)
	defer p.locker.unlock(key)

	resultChan := make(chan queryResult)
	p.queries <- queryRequest{
		query:  flagsQuery{prom: p, ctx: ctx, timestamp: time.Now()},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	r := FlagsResult{URI: p.uri, Flags: result.value.(v1.FlagsResult)}

	return &r, nil
}
