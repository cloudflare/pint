package promapi

import (
	"errors"
	"fmt"
	"net"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	prometheusQueriesRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pint_prometheus_queries_running",
			Help: "Total number of in-flight prometheus queries",
		},
		[]string{"name", "endpoint"},
	)
	prometheusCacheSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pint_prometheus_cache_size",
			Help: "Total number of entries currently stored in Prometheus query cache",
		},
		[]string{"name", "endpoint"},
	)
	prometheusCacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pint_prometheus_cache_hits_total",
			Help: "Total number of all prometheus queries served from a cache",
		},
		[]string{"name", "endpoint"},
	)
	prometheusQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pint_prometheus_queries_total",
			Help: "Total number of all prometheus queries",
		},
		[]string{"name", "endpoint"},
	)
	prometheusQueryErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pint_prometheus_query_errors_total",
			Help: "Total number of failed prometheus queries",
		},
		[]string{"name", "endpoint", "reason"},
	)
)

func RegisterMetrics() {
	prometheus.MustRegister(prometheusQueriesRunning)
	prometheus.MustRegister(prometheusCacheSize)
	prometheus.MustRegister(prometheusCacheHitsTotal)
	prometheus.MustRegister(prometheusQueriesTotal)
	prometheus.MustRegister(prometheusQueryErrorsTotal)
}

func errReason(err error) string {
	var neterr net.Error
	if ok := errors.As(err, &neterr); ok && neterr.Timeout() {
		return "connection/timeout"
	}

	var v1err *v1.Error
	if ok := errors.As(err, &v1err); ok {
		return fmt.Sprintf("api/%s", v1err.Type)
	}

	return "connection/error"
}
