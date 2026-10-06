package promapi

import (
	"errors"
	"net"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	prometheusQueriesRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pint_prometheus_queries_running",
			Help: "Total number of in-flight prometheus queries.",
		},
		[]string{"name", "endpoint"},
	)
	prometheusQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pint_prometheus_queries_total",
			Help: "Total number of all prometheus queries.",
		},
		[]string{"name", "endpoint"},
	)
	prometheusQueryErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pint_prometheus_query_errors_total",
			Help: "Total number of failed prometheus queries.",
		},
		[]string{"name", "endpoint", "reason"},
	)
	prometheusQuerySamplesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pint_prometheus_query_samples_total",
			Help: "Total number of samples Prometheus had to read to process queries.",
		},
		[]string{"name"},
	)
	prometheusSampleRateLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pint_prometheus_sample_rate_limit",
			Help: "Configured sampleRateLimit for this Prometheus server.",
		},
		[]string{"name"},
	)
)

func RegisterMetrics(reg *prometheus.Registry) {
	reg.MustRegister(prometheusQueriesRunning)
	reg.MustRegister(prometheusQueriesTotal)
	reg.MustRegister(prometheusQueryErrorsTotal)
	reg.MustRegister(prometheusQuerySamplesTotal)
	reg.MustRegister(prometheusSampleRateLimit)
}

func errReason(err error) string {
	if neterr, ok := errors.AsType[net.Error](err); ok && neterr.Timeout() {
		return "connection/timeout"
	}

	if e1, ok := errors.AsType[APIError](err); ok {
		return "api/" + string(e1.ErrorType)
	}

	return "connection/error"
}
