package promapi

import (
	"context"
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

func (fg *FailoverGroup) StartWorkers() {
	for _, prom := range fg.servers {
		prom.StartWorkers()
	}
}

func (fg *FailoverGroup) Close() {
	for _, prom := range fg.servers {
		prom.Close()
	}
}

func (fg *FailoverGroup) CleanCache() {
	for _, prom := range fg.servers {
		prom.purgeExpiredCache()
	}
}

func (fg *FailoverGroup) Config(ctx context.Context) (cfg *ConfigResult, err error) {
	var uri string
	for _, prom := range fg.servers {
		uri = prom.safeURI
		cfg, err = prom.Config(ctx)
		if err == nil {
			return
		}
		if !IsUnavailableError(err) {
			return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
		}
	}
	return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
}

func (fg *FailoverGroup) Query(ctx context.Context, expr string) (qr *QueryResult, err error) {
	var uri string
	for _, prom := range fg.servers {
		uri = prom.safeURI
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

func (fg *FailoverGroup) RangeQuery(ctx context.Context, expr string, params RangeQueryTimes) (rqr *RangeQueryResult, err error) {
	var uri string
	for _, prom := range fg.servers {
		uri = prom.safeURI
		rqr, err = prom.RangeQuery(ctx, expr, params)
		if err == nil {
			return
		}
		if !IsUnavailableError(err) {
			return rqr, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
		}
	}
	return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
}

func (fg *FailoverGroup) Metadata(ctx context.Context, metric string) (metadata *MetadataResult, err error) {
	var uri string
	for _, prom := range fg.servers {
		uri = prom.safeURI
		metadata, err = prom.Metadata(ctx, metric)
		if err == nil {
			return
		}
		if !IsUnavailableError(err) {
			return metadata, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
		}
	}
	return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
}

func (fg *FailoverGroup) Flags(ctx context.Context) (flags *FlagsResult, err error) {
	var uri string
	for _, prom := range fg.servers {
		uri = prom.safeURI
		flags, err = prom.Flags(ctx)
		if err == nil {
			return
		}
		if !IsUnavailableError(err) {
			return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
		}
	}
	return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
}
