package promapi

import (
	"context"
	"time"
)

type FailoverGroupError struct {
	err      error
	uri      string
	isStrict bool
}

func (e *FailoverGroupError) Unwrap() error {
	return e.err
}

func (e *FailoverGroupError) Error() string {
	return e.err.Error()
}

func (e *FailoverGroupError) URI() string {
	return e.uri
}

func (e *FailoverGroupError) IsStrict() bool {
	return e.isStrict
}

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

func (fg *FailoverGroup) Config(ctx context.Context) (cfg *ConfigResult, err error) {
	var uri string
	for _, prom := range fg.servers {
		uri = prom.uri
		cfg, err = prom.Config(ctx)
		if err == nil {
			return
		}
		if !IsUnavailableError(err) {
			return cfg, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
		}
	}
	return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
}

func (fg *FailoverGroup) Query(ctx context.Context, expr string) (qr *QueryResult, err error) {
	var uri string
	for _, prom := range fg.servers {
		uri = prom.uri
		qr, err = prom.Query(ctx, expr)
		if err == nil {
			return
		}
		if !IsUnavailableError(err) {
			return qr, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
		}
	}
	return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
}

func (fg *FailoverGroup) RangeQuery(ctx context.Context, expr string, start, end time.Time, step time.Duration) (rqr *RangeQueryResult, err error) {
	var uri string
	for _, prom := range fg.servers {
		uri = prom.uri
		rqr, err = prom.RangeQuery(ctx, expr, start, end, step)
		if err == nil {
			return
		}
		if !IsUnavailableError(err) {
			return rqr, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
		}
	}
	return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
}
