package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

var ciCmd = &cli.Command{
	Name:   "ci",
	Usage:  "Lint CI changes",
	Action: actionCI,
}

func actionCI(c *cli.Context) (err error) {
	err = initLogger(c.String(logLevelFlag), c.Bool(noColorFlag))
	if err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}

	cfg, err := config.Load(c.Path(configFlag), c.IsSet(configFlag))
	if err != nil {
		return fmt.Errorf("failed to load config file %q: %w", c.Path(configFlag), err)
	}
	if c.Bool(offlineFlag) {
		cfg.DisableOnlineChecks()
	}

	includeRe := []*regexp.Regexp{}
	for _, pattern := range cfg.CI.Include {
		includeRe = append(includeRe, regexp.MustCompile("^"+pattern+"$"))
	}

	baseBranch := strings.Split(cfg.CI.BaseBranch, "/")[len(strings.Split(cfg.CI.BaseBranch, "/"))-1]
	currentBranch, err := git.CurrentBranch(git.RunGit)
	if err != nil {
		return fmt.Errorf("failed to get the name of current branch")
	}
	log.Debug().Str("current", currentBranch).Str("base", baseBranch).Msg("Got branch information")
	if currentBranch == baseBranch {
		log.Info().Str("branch", currentBranch).Msg("Running from base branch, skipping checks")
		return nil
	}

	gitDiscovery := discovery.NewGitBranchFileFinder(git.RunGit, includeRe, cfg.CI.BaseBranch)
	toScan, err := gitDiscovery.Find()
	if err != nil {
		return fmt.Errorf("failed to get the list of modified files: %w", err)
	}
	if len(toScan.Commits()) > cfg.CI.MaxCommits {
		return fmt.Errorf("number of commits to check (%d) is higher than maxCommits (%d), exiting", len(toScan.Commits()), cfg.CI.MaxCommits)
	}

	for _, fc := range toScan.Results() {
		log.Debug().Strs("commits", fc.Commits).Str("path", fc.Path).Msg("File to scan")
	}
	log.Debug().Strs("commits", toScan.Commits()).Msg("Found commits to scan")

	gitBlame := discovery.NewGitBlameLineFinder(git.RunGit, toScan.Commits())
	summary := scanFiles(context.Background(), cfg, toScan, gitBlame)

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

	if cfg.Repository != nil && cfg.Repository.GitHub != nil {
		token, ok := os.LookupEnv("GITHUB_AUTH_TOKEN")
		if !ok {
			return fmt.Errorf("GITHUB_AUTH_TOKEN env variable is required when reporting to GitHub")
		}

		prVal, ok := os.LookupEnv("GITHUB_PULL_REQUEST_NUMBER")
		if !ok {
			return fmt.Errorf("GITHUB_PULL_REQUEST_NUMBER env variable is required when reporting to GitHub")
		}

		prNum, err := strconv.Atoi(prVal)
		if err != nil {
			return fmt.Errorf("got not a valid number via GITHUB_PULL_REQUEST_NUMBER: %w", err)
		}

		timeout, _ := time.ParseDuration(cfg.Repository.GitHub.Timeout)
		gr := reporter.NewGithubReporter(
			cfg.Repository.GitHub.BaseURI,
			cfg.Repository.GitHub.UploadURI,
			timeout,
			token,
			cfg.Repository.GitHub.Owner,
			cfg.Repository.GitHub.Repo,
			prNum,
			git.RunGit,
		)
		reps = append(reps, gr)
	}

	foundBugOrHigher := false
	bySeverity := map[string]interface{}{}
	for s, c := range summary.CountBySeverity() {
		if s >= checks.Bug {
			foundBugOrHigher = true
		}
		bySeverity[s.String()] = c
	}
	if len(bySeverity) > 0 {
		log.Info().Fields(bySeverity).Msg("Problems found")
	}

	if err := submitReports(reps, summary); err != nil {
		return fmt.Errorf("submitting reports: %w", err)
	}

	if foundBugOrHigher {
		return fmt.Errorf("problems found")
	}

	return nil
}
