package promapi

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"go.yaml.in/yaml/v4"
)

const (
	APIPathConfig = "/api/v1/status/config"
)

type ConfigSectionGlobal struct {
	ExternalLabels     map[string]string `yaml:"external_labels"`
	ScrapeInterval     time.Duration     `yaml:"scrape_interval"`
	ScrapeTimeout      time.Duration     `yaml:"scrape_timeout"`
	EvaluationInterval time.Duration     `yaml:"evaluation_interval"`
}

type PrometheusConfig struct {
	RuleFiles []string            `yaml:"rule_files"`
	Global    ConfigSectionGlobal `yaml:"global"`
}

type PrometheusConfigResponse struct {
	PrometheusResponse
	Data v1.ConfigResult `json:"data"`
}

type ConfigResult struct {
	URI    string
	Config PrometheusConfig
}

type configQuery struct {
	prom      *Prometheus
	ctx       context.Context
	timestamp time.Time
	cacheTTL  time.Duration
}

func (q configQuery) Run() queryResult {
	slog.LogAttrs(q.ctx, slog.LevelDebug, "Getting prometheus configuration", slog.String("uri", q.prom.safeURI))

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

	qr.value, err = parseConfig(resp.Body)
	if err != nil {
		prometheusQueryErrorsTotal.WithLabelValues(q.prom.name, APIPathConfig, errReason(err)).Inc()
		qr.err = fmt.Errorf("failed to decode config data in %s response: %w", q.prom.safeURI, err)
	}
	return qr
}

func (q configQuery) Endpoint() string {
	return APIPathConfig
}

func (q configQuery) String() string {
	return APIPathConfig
}

func (q configQuery) CacheKey() uint64 {
	return hash(q.prom.unsafeURI, q.Endpoint())
}

func (q configQuery) CacheTTL() time.Duration {
	return q.cacheTTL
}

func (prom *Prometheus) Config(ctx context.Context, cacheTTL time.Duration) (*ConfigResult, error) {
	slog.LogAttrs(ctx, slog.LevelDebug, "Scheduling Prometheus configuration query", slog.String("uri", prom.safeURI))

	prom.locker.lock(APIPathConfig)
	defer prom.locker.unlock(APIPathConfig)

	if cacheTTL == 0 {
		cacheTTL = time.Minute
	}

	result, err := prom.runQuery(ctx, configQuery{
		prom:      prom,
		ctx:       ctx,
		timestamp: time.Now(),
		cacheTTL:  cacheTTL,
	})
	if err != nil {
		return nil, QueryError{err: err, msg: decodeError(err)}
	}

	r := ConfigResult{
		URI:    prom.publicURI,
		Config: result.value.(PrometheusConfig),
	}

	return &r, nil
}

func parseConfig(r io.Reader) (cfg PrometheusConfig, err error) {
	defer dummyReadAll(r)

	var data PrometheusConfigResponse
	if err = json.UnmarshalRead(r, &data); err != nil {
		return cfg, APIError{
			Status:    data.Status,
			ErrorType: v1.ErrBadResponse,
			Err:       fmt.Errorf("JSON parse error: %w", err),
		}
	}

	if data.Status != "success" {
		if data.Error == "" {
			data.Error = "empty response object"
		}
		return cfg, APIError{
			Status:    data.Status,
			ErrorType: decodeErrorType(data.ErrorType),
			Err:       errors.New(data.Error),
		}
	}

	if err = yaml.Load([]byte(data.Data.YAML), &cfg); err != nil {
		return cfg, yamlErrorMessage(err)
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

// yamlErrorMessage formats yaml load errors without their stage and column
// details, which make the error too long to show in a report.
// Every error returned by yaml.Load is a *yaml.LoadErrors or a *yaml.LoadError.
func yamlErrorMessage(err error) error {
	loadErrs := []*yaml.LoadError{}
	if loadErrors, ok := errors.AsType[*yaml.LoadErrors](err); ok {
		loadErrs = loadErrors.Errors
	} else {
		loadErr, _ := errors.AsType[*yaml.LoadError](err)
		loadErrs = append(loadErrs, loadErr)
	}

	msgs := make([]error, 0, len(loadErrs))
	for _, le := range loadErrs {
		// Some go-yaml messages already start with this prefix.
		msg := strings.TrimPrefix(le.Message, "yaml: ")
		if le.ContextMsg != "" {
			msg = le.ContextMsg + ": " + msg
		}
		if le.Mark.Line > 0 {
			msg += fmt.Sprintf(" at line %d", le.Mark.Line)
		}
		msgs = append(msgs, errors.New("yaml: "+msg))
	}
	return errors.Join(msgs...)
}
