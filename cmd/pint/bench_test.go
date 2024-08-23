package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/log"
)

func BenchmarkFindEntries(b *testing.B) {
	log.Setup(slog.LevelError, true)

	finder := discovery.NewGlobFinder(
		[]string{"bench/rules"},
		git.NewPathFilter(nil, nil, nil),
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
	)
	entries, err := finder.Find()
	if err != nil {
		b.Errorf("Find() error: %s", err)
		b.FailNow()
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/status/config":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 30s\n"}}`))
		case "/api/v1/status/flags":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"storage.tsdb.retention.time": "1d"}}`))
		case "/api/v1/metadata":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		case "/api/v1/query":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		case "/api/v1/query_range":
			w.WriteHeader(200)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		default:
			b.Logf("%s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
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

	ctx := context.Background()
	cfg, _, err := config.Load(tmp+"/.pint.hcl", false)
	require.NoError(b, err)

	gen := config.NewPrometheusGenerator(cfg, prometheus.NewRegistry())
	require.NoError(b, gen.GenerateStatic())

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, _ = checkRules(ctx, "", 10, false, gen, cfg, entries)
	}
}
