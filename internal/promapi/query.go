package promapi

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"time"

	"github.com/prometheus/common/model"
	"github.com/rs/zerolog/log"
)

type QueryResult struct {
	URI    string
	Series model.Vector
}

type instantQuery struct {
	prom      *Prometheus
	ctx       context.Context
	expr      string
	timestamp time.Time
}

func (q instantQuery) Run() queryResult {
	log.Debug().
		Str("uri", q.prom.uri).
		Str("query", q.expr).
		Msg("Running prometheus query")

	ctx, cancel := context.WithTimeout(q.ctx, q.prom.timeout)
	defer cancel()

	v, _, err := q.prom.api.Query(ctx, q.expr, time.Now())
	return queryResult{value: v, err: err, expires: q.timestamp.Add(cacheExpiry * 2)}
}

func (q instantQuery) Endpoint() string {
	return "/api/v1/query"
}

func (q instantQuery) String() string {
	return q.expr
}

func (q instantQuery) CacheKey() string {
	h := sha1.New()
	_, _ = io.WriteString(h, q.Endpoint())
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, q.expr)
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, q.timestamp.Round(cacheExpiry).Format(time.RFC3339))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Prometheus) Query(ctx context.Context, expr string) (*QueryResult, error) {
	log.Debug().Str("uri", p.uri).Str("query", expr).Msg("Scheduling prometheus query")

	key := fmt.Sprintf("/api/v1/query/%s", expr)
	p.locker.lock(key)
	defer p.locker.unlock(key)

	resultChan := make(chan queryResult)
	p.queries <- queryRequest{
		query:  instantQuery{prom: p, ctx: ctx, expr: expr, timestamp: time.Now()},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	qr := QueryResult{URI: p.uri}

	// nolint: exhaustive
	switch result.value.(model.Value).Type() {
	case model.ValVector:
		vectorVal := result.value.(model.Vector)
		qr.Series = vectorVal
	default:
		log.Error().Str("uri", p.uri).Str("query", expr).Msgf("Query returned unknown result type: %v", result.value.(model.Value).Type())
		prometheusQueryErrorsTotal.WithLabelValues(p.name, "/api/v1/query", "unknown result type").Inc()
		return nil, fmt.Errorf("unknown result type: %v", result.value.(model.Value).Type())
	}
	log.Debug().Str("uri", p.uri).Str("query", expr).Int("series", len(qr.Series)).Msg("Parsed response")

	return &qr, nil
}
