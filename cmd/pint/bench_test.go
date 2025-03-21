package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/log"
	"github.com/cloudflare/pint/internal/parser"
	"github.com/cloudflare/pint/internal/promapi"
)

func BenchmarkFindEntries(b *testing.B) {
	log.Setup(slog.LevelError, true)

	finder := discovery.NewGlobFinder(
		[]string{"bench/rules"},
		git.NewPathFilter(nil, nil, nil),
		parser.PrometheusSchema,
		model.UTF8Validation,
		nil,
	)
	for n := 0; n < b.N; n++ {
		_, _ = finder.Find()
	}
}

func BenchmarkCheckRules(b *testing.B) {
	log.Setup(slog.LevelError, true)

	finder := discovery.NewGlobFinder(
		[]string{"bench/rules"},
		git.NewPathFilter(nil, nil, nil),
		parser.PrometheusSchema,
		model.UTF8Validation,
		nil,
	)
	entries, err := finder.Find()
	if err != nil {
		b.Errorf("Find() error: %s", err)
		b.FailNow()
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case promapi.APIPathConfig:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 30s\n"}}`))
		case promapi.APIPathFlags:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"storage.tsdb.retention.time": "1d"}}`))
		case promapi.APIPathMetadata:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		case promapi.APIPathQuery:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		case promapi.APIPathQueryRange:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		default:
			b.Logf("%s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`Not found`))
		}
	}))
	defer srv.Close()

	tmp := b.TempDir()
	content := []byte(fmt.Sprintf(`prometheus "prom" {
  uri         = "%s"
  timeout     = "30s"
  uptime      = "prometheus_ready"
  concurrency = 10
  rateLimit   = 5000
}

rule {
  alerts {
    range    = "1h"
    step     = "1m"
    resolve  = "5m"
    minCount = 50
  }
}
`, srv.URL))
	require.NoError(b, os.WriteFile(tmp+"/.pint.hcl", content, 0o644))

	cfg, _, err := config.Load(tmp+"/.pint.hcl", false)
	require.NoError(b, err)

	gen := config.NewPrometheusGenerator(cfg, prometheus.NewRegistry())
	require.NoError(b, gen.GenerateStatic())

	b.ResetTimer()
	for b.Loop() {
		_, _ = checkRules(b.Context(), 10, false, gen, cfg, entries)
	}
}
