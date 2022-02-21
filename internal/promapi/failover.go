package promapi

import (
	"context"
	"time"
)

type FailoverGroup struct {
	name         string
	servers      []*Prometheus
	strictErrors bool
}

func NewFailoverGroup(name string, servers []*Prometheus, strictErrors bool) *FailoverGroup {
	return &FailoverGroup{
		name:         name,
		servers:      servers,
		strictErrors: strictErrors,
	}
}

func (fg *FailoverGroup) Name() string {
	return fg.name
}

func (fg *FailoverGroup) ClearCache() {
	for _, prom := range fg.servers {
		prom.cache.Purge()
	}
}

func (fg *FailoverGroup) Config(ctx context.Context) (cfg *PrometheusConfig, err error) {
	for _, prom := range fg.servers {
		cfg, err = prom.Config(ctx)
		if err == nil || !IsUnavailableError(err) {
			return
		}
	}
	return nil, &Error{err: err, isStrict: fg.strictErrors}
}

func (fg *FailoverGroup) Query(ctx context.Context, expr string) (qr *QueryResult, err error) {
	for _, prom := range fg.servers {
		qr, err = prom.Query(ctx, expr)
		if err == nil || !IsUnavailableError(err) {
			return
		}
	}
	return nil, &Error{err: err, isStrict: fg.strictErrors}
}

func (fg *FailoverGroup) RangeQuery(ctx context.Context, expr string, start, end time.Time, step time.Duration) (rqr *RangeQueryResult, err error) {
	for _, prom := range fg.servers {
		rqr, err = prom.RangeQuery(ctx, expr, start, end, step)
		if err == nil || !IsUnavailableError(err) {
			return
		}
	}
	return nil, &Error{err: err, isStrict: fg.strictErrors}
}
