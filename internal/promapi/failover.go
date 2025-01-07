package promapi

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"slices"
	"sync"
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

type disabledChecks struct {
	// Key is the name of the unsupported API, value is the list of checks disabled because of it.
	apis map[string][]string
	mtx  sync.Mutex
}

func (dc *disabledChecks) disable(api, check string) {
	dc.mtx.Lock()
	if _, ok := dc.apis[api]; !ok {
		dc.apis[api] = []string{}
	}
	if !slices.Contains(dc.apis[api], check) {
		dc.apis[api] = append(dc.apis[api], check)
	}
	dc.mtx.Unlock()
}

func (dc *disabledChecks) read() map[string][]string {
	dc.mtx.Lock()
	defer dc.mtx.Unlock()
	return dc.apis
}

type FailoverGroup struct {
	disabledChecks disabledChecks

	name           string
	uri            string
	servers        []*Prometheus
	uptimeMetric   string
	cacheCollector *cacheCollector
	quitChan       chan bool

	pathsInclude []*regexp.Regexp
	pathsExclude []*regexp.Regexp
	tags         []string
	started      bool
	strictErrors bool
}

func NewFailoverGroup(name, uri string, servers []*Prometheus, strictErrors bool, uptimeMetric string, include, exclude []*regexp.Regexp, tags []string) *FailoverGroup {
	return &FailoverGroup{ // nolint: exhaustruct
		name:           name,
		uri:            uri,
		servers:        servers,
		strictErrors:   strictErrors,
		uptimeMetric:   uptimeMetric,
		pathsInclude:   include,
		pathsExclude:   exclude,
		tags:           tags,
		disabledChecks: disabledChecks{apis: map[string][]string{}}, // nolint: exhaustruct
	}
}

func (fg *FailoverGroup) Name() string {
	return fg.name
}

func (fg *FailoverGroup) URI() string {
	return fg.uri
}

func (fg *FailoverGroup) DisableCheck(api, s string) {
	fg.disabledChecks.disable(api, s)
}

func (fg *FailoverGroup) GetDisabledChecks() map[string][]string {
	return fg.disabledChecks.read()
}

func (fg *FailoverGroup) Include() []string {
	sl := []string{}
	for _, re := range fg.pathsInclude {
		sl = append(sl, re.String())
	}
	return sl
}

func (fg *FailoverGroup) Exclude() []string {
	sl := []string{}
	for _, re := range fg.pathsExclude {
		sl = append(sl, re.String())
	}
	return sl
}

func (fg *FailoverGroup) Tags() []string {
	return fg.tags
}

func (fg *FailoverGroup) UptimeMetric() string {
	return fg.uptimeMetric
}

func (fg *FailoverGroup) ServerCount() int {
	return len(fg.servers)
}

func (fg *FailoverGroup) MergeUpstreams(src *FailoverGroup) {
	for _, ns := range src.servers {
		var present bool
		for _, ol := range fg.servers {
			if ol.unsafeURI == ns.unsafeURI {
				present = true
				break
			}
		}
		if !present {
			fg.servers = append(fg.servers, ns)
			slog.Debug(
				"Added new failover URI",
				slog.String("name", ns.name),
				slog.String("uri", ns.safeURI),
			)
		}
	}
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

func (fg *FailoverGroup) StartWorkers(reg *prometheus.Registry) {
	if fg.started {
		return
	}

	queryCache := newQueryCache(time.Hour)
	fg.quitChan = make(chan bool)
	go cacheCleaner(queryCache, time.Minute*2, fg.quitChan)

	fg.cacheCollector = newCacheCollector(queryCache, fg.name)
	reg.MustRegister(fg.cacheCollector)
	for _, prom := range fg.servers {
		prom.cache = queryCache
		prom.StartWorkers()
	}
	fg.started = true
}

func (fg *FailoverGroup) Close(reg *prometheus.Registry) {
	if !fg.started {
		return
	}
	for _, prom := range fg.servers {
		prom.Close()
	}
	reg.Unregister(fg.cacheCollector)
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

func (fg *FailoverGroup) Config(ctx context.Context, cacheTTL time.Duration) (cfg *ConfigResult, err error) {
	var uri string
	for _, prom := range fg.servers {
		uri = prom.safeURI
		cfg, err = prom.Config(ctx, cacheTTL)
		if err == nil {
			return cfg, nil
		}
		if !IsUnavailableError(err) && !errors.Is(err, ErrUnsupported) {
			return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
		}
	}
	return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
}

func (fg *FailoverGroup) Query(ctx context.Context, expr string) (qr *QueryResult, err error) {
	var uri string
	for try, prom := range fg.servers {
		if try > 0 {
			slog.Debug(
				"Using failover URI",
				slog.String("name", fg.name),
				slog.Int("retry", try),
				slog.String("uri", prom.safeURI),
			)
		}
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
		if !IsUnavailableError(err) && !errors.Is(err, ErrUnsupported) {
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
		if !IsUnavailableError(err) && !errors.Is(err, ErrUnsupported) {
			return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
		}
	}
	return nil, &FailoverGroupError{err: err, uri: uri, isStrict: fg.strictErrors}
}
