package promapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prymitive/current"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type ConfigSectionGlobal struct {
	ScrapeInterval     time.Duration     `yaml:"scrape_interval"`
	ScrapeTimeout      time.Duration     `yaml:"scrape_timeout"`
	EvaluationInterval time.Duration     `yaml:"evaluation_interval"`
	ExternalLabels     map[string]string `yaml:"external_labels"`
}

type PrometheusConfig struct {
	Global ConfigSectionGlobal `yaml:"global"`
}

type ConfigResult struct {
	URI    string
	Config PrometheusConfig
}

type configQuery struct {
	prom      *Prometheus
	ctx       context.Context
	timestamp time.Time
}

func (q configQuery) Run() queryResult {
	log.Debug().
		Str("uri", q.prom.safeURI).
		Msg("Getting prometheus configuration")

	ctx, cancel := q.prom.requestContext(q.ctx)
	defer cancel()

	var qr queryResult

	args := url.Values{}
	resp, err := q.prom.doRequest(ctx, http.MethodGet, q.Endpoint(), args)
	if err != nil {
		qr.err = fmt.Errorf("failed to query Prometheus config: %w", err)
		return qr
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		qr.err = tryDecodingAPIError(resp)
		return qr
	}

	qr.value, err = streamConfig(resp.Body)
	if err != nil {
		prometheusQueryErrorsTotal.WithLabelValues(q.prom.name, "/api/v1/status/config", errReason(err)).Inc()
		qr.err = fmt.Errorf("failed to decode config data in %s response: %w", q.prom.safeURI, err)
	}
	return qr
}

func (q configQuery) Endpoint() string {
	return "/api/v1/status/config"
}

func (q configQuery) String() string {
	return "/api/v1/status/config"
}

func (q configQuery) CacheKey() uint64 {
	return hash(q.prom.unsafeURI, q.Endpoint())
}

func (q configQuery) CacheTTL() time.Duration {
	return time.Minute * 10
}

func (p *Prometheus) Config(ctx context.Context) (*ConfigResult, error) {
	log.Debug().Str("uri", p.safeURI).Msg("Scheduling Prometheus configuration query")

	key := "/api/v1/status/config"
	p.locker.lock(key)
	defer p.locker.unlock(key)

	resultChan := make(chan queryResult)
	p.queries <- queryRequest{
		query:  configQuery{prom: p, ctx: ctx, timestamp: time.Now()},
		result: resultChan,
	}

	result := <-resultChan
	if result.err != nil {
		return nil, QueryError{err: result.err, msg: decodeError(result.err)}
	}

	r := ConfigResult{URI: p.safeURI, Config: result.value.(PrometheusConfig)}

	return &r, nil
}

func streamConfig(r io.Reader) (cfg PrometheusConfig, err error) {
	defer dummyReadAll(r)

	var yamlBody, status, errType, errText string
	decoder := current.Object(
		current.Key("status", current.Value(func(s string, isNil bool) {
			status = s
		})),
		current.Key("error", current.Value(func(s string, isNil bool) {
			errText = s
		})),
		current.Key("errorType", current.Value(func(s string, isNil bool) {
			errType = s
		})),
		current.Key("data", current.Object(
			current.Key("yaml", current.Value(func(s string, isNil bool) {
				yamlBody = s
			})),
		)),
	)

	dec := json.NewDecoder(r)
	if err = decoder.Stream(dec); err != nil {
		return cfg, APIError{Status: status, ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("JSON parse error: %s", err)}
	}

	if status != "success" {
		return cfg, APIError{Status: status, ErrorType: decodeErrorType(errType), Err: errText}
	}

	if err = yaml.Unmarshal([]byte(yamlBody), &cfg); err != nil {
		return cfg, err
	}

	if cfg.Global.ScrapeInterval == 0 {
		cfg.Global.ScrapeInterval = time.Minute
	}
	if cfg.Global.ScrapeTimeout == 0 {
		cfg.Global.ScrapeTimeout = time.Second * 10
	}
	if cfg.Global.EvaluationInterval == 0 {
		cfg.Global.EvaluationInterval = time.Minute
	}

	return cfg, nil
}
