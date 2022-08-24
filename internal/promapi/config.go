package promapi

import (
	"context"
	"crypto/sha1"
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
		Str("uri", q.prom.uri).
		Msg("Getting prometheus configuration")

	ctx, cancel := context.WithTimeout(q.ctx, q.prom.timeout)
	defer cancel()

	qr := queryResult{expires: q.timestamp.Add(cacheExpiry * 2)}

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

	qr.value, qr.err = streamConfig(resp.Body)
	return qr
}

func (q configQuery) Endpoint() string {
	return "/api/v1/status/config"
}

func (q configQuery) String() string {
	return "/api/v1/status/config"
}

func (q configQuery) CacheKey() string {
	h := sha1.New()
	_, _ = io.WriteString(h, q.Endpoint())
	_, _ = io.WriteString(h, "\n")
	_, _ = io.WriteString(h, q.timestamp.Round(cacheExpiry).Format(time.RFC3339))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (p *Prometheus) Config(ctx context.Context) (*ConfigResult, error) {
	log.Debug().Str("uri", p.uri).Msg("Scheduling Prometheus configuration query")

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

	var cfg PrometheusConfig
	if err := yaml.Unmarshal([]byte(result.value.(string)), &cfg); err != nil {
		prometheusQueryErrorsTotal.WithLabelValues(p.name, "/api/v1/status/config", errReason(err)).Inc()
		return nil, fmt.Errorf("failed to decode config data in %s response: %w", p.uri, err)
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

	r := ConfigResult{URI: p.uri, Config: cfg}

	return &r, nil
}

func streamConfig(r io.Reader) (cfg string, err error) {
	defer dummyReadAll(r)

	var status, errType, errText string
	decoder := current.Object(
		func() {},
		current.Key("status", current.Text(func(s string) {
			status = s
		})),
		current.Key("error", current.Text(func(s string) {
			errText = s
		})),
		current.Key("errorType", current.Text(func(s string) {
			errType = s
		})),
		current.Key("data", current.Object(
			func() {},
			current.Key("yaml", current.Text(func(s string) {
				cfg = s
			})),
		)),
	)

	dec := json.NewDecoder(r)
	if err = current.Stream(dec, decoder); err != nil {
		return cfg, APIError{Status: status, ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("JSON parse error: %s", err)}
	}

	if status != "success" {
		return cfg, APIError{Status: status, ErrorType: decodeErrorType(errType), Err: errText}
	}

	return cfg, nil
}
