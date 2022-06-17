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
	URI             string
	Series          model.Vector
	DurationSeconds float64
}

type instantQuery struct {
	prom *Prometheus
	ctx  context.Context
	expr string
}

func (q instantQuery) Run() (any, error) {
	log.Debug().
		Str("uri", q.prom.uri).
		Str("query", q.expr).
		Msg("Running prometheus query")

	ctx, cancel := context.WithTimeout(q.ctx, q.prom.timeout)
	defer cancel()

	v, _, err := q.prom.api.Query(ctx, q.expr, time.Now())
	return v, err
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
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Prometheus) Query(ctx context.Context, expr string) (*QueryResult, error) {
	log.Debug().Str("uri", p.uri).Str("query", expr).Msg("Scheduling prometheus query")

	resultChan := make(chan queryResult)
	p.queries <- queryRequest{
		query:  instantQuery{prom: p, ctx: ctx, expr: expr},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	qr := QueryResult{URI: p.uri}

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
