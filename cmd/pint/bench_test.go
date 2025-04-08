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

func BenchmarkGlobFinder(b *testing.B) {
	log.Setup(slog.LevelError, true)

	finder := discovery.NewGlobFinder(
		[]string{"bench/rules"},
		git.NewPathFilter(nil, nil, nil),
		parser.PrometheusSchema,
		model.UTF8Validation,
		nil,
	)

	b.ResetTimer()
	for b.Loop() {
		_, _ = finder.Find()
	}
}

func BenchmarkGitFinder(b *testing.B) {
	log.Setup(slog.LevelError, true)

	tmp := b.TempDir()
	require.NoError(b, os.CopyFS(tmp, os.DirFS("bench")))
	b.Chdir(tmp)

	b.Setenv("GIT_AUTHOR_NAME", "pint")
	b.Setenv("GIT_AUTHOR_EMAIL", "pint@example.com")
	b.Setenv("GIT_COMMITTER_NAME", "pint")
	b.Setenv("GIT_COMMITTER_EMAIL", "pint")

	_, err := git.RunGit("init", "--initial-branch=main", ".")
	require.NoError(b, err, "git init")

	_, err = git.RunGit("add", "Makefile", "README.md")
	require.NoError(b, err, "git add")
	_, err = git.RunGit("commit", "-am", "commit")
	require.NoError(b, err, "git commit")

	_, err = git.RunGit("checkout", "-b", "v2")
	require.NoError(b, err, "git checkout v2")

	_, err = git.RunGit("add", ".")
	require.NoError(b, err, "git add")

	_, err = git.RunGit("commit", "-am", "commit")
	require.NoError(b, err, "git commit")

	finder := discovery.NewGitBranchFinder(
		git.RunGit,
		git.NewPathFilter(nil, nil, nil),
		"main",
		50,
		parser.PrometheusSchema,
		model.UTF8Validation,
		nil,
	)

	b.ResetTimer()
	for b.Loop() {
		_, _ = finder.Find(nil)
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
			_, _ = w.Write(
				[]byte(`{"status":"success","data":{"yaml":"global:\n  scrape_interval: 30s\n"}}`),
			)
		case promapi.APIPathFlags:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(
				[]byte(`{"status":"success","data":{"storage.tsdb.retention.time": "1d"}}`),
			)
		case promapi.APIPathMetadata:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
		case promapi.APIPathQuery:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(
				[]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`),
			)
		case promapi.APIPathQueryRange:
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(
				[]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`),
			)
		default:
			b.Logf("%s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`Not found`))
		}
	}))
	defer srv.Close()

	tmp := b.TempDir()
	content := fmt.Appendf(nil, `prometheus "prom" {
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
`, srv.URL)
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
