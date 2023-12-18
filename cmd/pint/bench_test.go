package main

import (
	"context"
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

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

	ctx := context.Background()
	cfg, _ := config.Load("", false)
	gen := config.NewPrometheusGenerator(cfg, prometheus.NewRegistry())
	for n := 0; n < b.N; n++ {
		_, _ = checkRules(ctx, 10, true, gen, cfg, entries)
	}
}
