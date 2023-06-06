package promapi

import (
	"context"
	"regexp"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

func cacheCleaner(cache *queryCache, interval time.Duration, quit chan bool) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-quit:
			return
		case <-ticker.C:
			cache.gc()
		}
	}
}

type FailoverGroup struct {
	name           string
	servers        []*Prometheus
	strictErrors   bool
	uptimeMetric   string
	cacheCollector *cacheCollector
	quitChan       chan bool

	pathsInclude []*regexp.Regexp
	pathsExclude []*regexp.Regexp
	tags         []string
}

func NewFailoverGroup(name string, servers []*Prometheus, strictErrors bool, uptimeMetric string, include, exclude []*regexp.Regexp, tags []string) *FailoverGroup {
	return &FailoverGroup{
		name:         name,
		servers:      servers,
		strictErrors: strictErrors,
		uptimeMetric: uptimeMetric,
		pathsInclude: include,
		pathsExclude: exclude,
		tags:         tags,
	}
}

func (fg *FailoverGroup) Name() string {
	return fg.name
}

func (fg *FailoverGroup) Tags() []string {
	return fg.tags
}

func (fg *FailoverGroup) UptimeMetric() string {
	return fg.uptimeMetric
}

func (fg *FailoverGroup) IsEnabledForPath(path string) bool {
	if len(fg.pathsInclude) == 0 && len(fg.pathsExclude) == 0 {
		return true
	}
	for _, re := range fg.pathsExclude {
		if re.MatchString(path) {
			return false
		}
	}
	for _, re := range fg.pathsInclude {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

func (fg *FailoverGroup) StartWorkers() {
	queryCache := newQueryCache(time.Hour)
	fg.quitChan = make(chan bool)
	go cacheCleaner(queryCache, time.Minute*2, fg.quitChan)

	fg.cacheCollector = newCacheCollector(queryCache, fg.name)
	prometheus.MustRegister(fg.cacheCollector)
	for _, prom := range fg.servers {
		prom.cache = queryCache
		prom.StartWorkers()
	}
}

func (fg *FailoverGroup) Close() {
	for _, prom := range fg.servers {
		prom.Close()
	}
	prometheus.Unregister(fg.cacheCollector)
	fg.quitChan <- true
}

func (fg *FailoverGroup) CleanCache() {
	for _, prom := range fg.servers {
		if prom.cache != nil {
			prom.cache.gc()
			return
		}
	}
}

func (fg *FailoverGroup) Config(ctx context.Context) (cfg *ConfigResult, err error) {
	var uri string
	for _, prom := range fg.servers {
		uri = prom.safeURI
		cfg, err = prom.Config(ctx)
		if err == nil {
			return cfg, nil
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
			return qr, nil
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
			return rqr, nil
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
			return metadata, nil
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
			return flags, nil
		}
		if !IsUnavailableError(err) {
			return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
		}
	}
	return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
}
