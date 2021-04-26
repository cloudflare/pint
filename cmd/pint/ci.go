package main

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func actionCI(c *cli.Context) (err error) {
	err = initLogger(c.String(logLevelFlag))
	if err != nil {
		return fmt.Errorf("failed to set log level: %s", err)
	}

	cfg, err := config.Load(c.Path(configFlag))
	if err != nil {
		return fmt.Errorf("failed to load config file %q: %s", c.Path(configFlag), err)
	}

	includeRe := []*regexp.Regexp{}
	for _, pattern := range cfg.CI.Include {
		includeRe = append(includeRe, regexp.MustCompile("^"+pattern+"$"))
	}

	gitDiscovery := discovery.NewGitBranchFileFinder(git.RunGit, includeRe, cfg.CI.BaseBranch)
	toScan, err := gitDiscovery.Find()
	if err != nil {
		return fmt.Errorf("failed to get the list of modified files: %v", err)
	}
	if len(toScan.Commits()) > cfg.CI.MaxCommits {
		return fmt.Errorf("number of commits to check (%d) is higher than maxCommits(%d), exiting", len(toScan.Commits()), cfg.CI.MaxCommits)
	}

	for _, fc := range toScan.Results() {
		log.Debug().Strs("commits", fc.Commits).Str("path", fc.Path).Msg("File to scan")
	}
	log.Debug().Strs("commits", toScan.Commits()).Msg("Found commits to scan")

	gitBlame := discovery.NewGitBlameLineFinder(git.RunGit, toScan.Commits())
	summary := scanFiles(cfg, toScan, gitBlame)

	reps := []reporter.Reporter{
		reporter.NewConsoleReporter(os.Stderr),
	}

	if cfg.Repository != nil && cfg.Repository.BitBucket != nil {
		token, ok := os.LookupEnv("BITBUCKET_AUTH_TOKEN")
		if !ok {
			return fmt.Errorf("BITBUCKET_AUTH_TOKEN env variable is required when reporting to BitBucket")
		}

		timeout, _ := time.ParseDuration(cfg.Repository.BitBucket.Timeout)
		br := reporter.NewBitBucketReporter(
			cfg.Repository.BitBucket.URI,
			timeout,
			token,
			cfg.Repository.BitBucket.Project,
			cfg.Repository.BitBucket.Repository,
			git.RunGit,
		)
		reps = append(reps, br)
	}

	bySeverity := map[string]interface{}{}
	for s, c := range summary.CountBySeverity() {
		bySeverity[s.String()] = c
	}
	if len(bySeverity) > 0 {
		log.Info().Fields(bySeverity).Msg("Problems found")
	}

	return submitReports(reps, summary)
}
