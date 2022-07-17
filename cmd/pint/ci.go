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

var baseBranchFlag = "base-branch"

var ciCmd = &cli.Command{
	Name:   "ci",
	Usage:  "Lint CI changes",
	Action: actionCI,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    requireOwnerFlag,
			Aliases: []string{"r"},
			Value:   false,
			Usage:   "Require all rules to have an owner set via comment",
		},
		&cli.StringFlag{
			Name:    baseBranchFlag,
			Aliases: []string{"b"},
			Value:   "",
			Usage:   "Set base branch to use for PR checks (main, master, ...)",
		},
	},
}

func actionCI(c *cli.Context) error {
	meta, err := actionSetup(c)
	if err != nil {
		return err
	}

	includeRe := []*regexp.Regexp{}
	for _, pattern := range meta.cfg.CI.Include {
		includeRe = append(includeRe, regexp.MustCompile("^"+pattern+"$"))
	}

	baseBranch := strings.Split(meta.cfg.CI.BaseBranch, "/")[len(strings.Split(meta.cfg.CI.BaseBranch, "/"))-1]
	if c.String(baseBranchFlag) != "" {
		baseBranch = c.String(baseBranchFlag)
	}
	currentBranch, err := git.CurrentBranch(git.RunGit)
	if err != nil {
		return fmt.Errorf("failed to get the name of current branch")
	}
	log.Debug().Str("current", currentBranch).Str("base", baseBranch).Msg("Got branch information")
	if currentBranch == baseBranch {
		log.Info().Str("branch", currentBranch).Msg("Running from base branch, skipping checks")
		return nil
	}

	finder := discovery.NewGitBranchFinder(git.RunGit, includeRe, meta.cfg.CI.BaseBranch, meta.cfg.CI.MaxCommits, meta.cfg.Parser.CompileRelaxed())
	entries, err := finder.Find()
	if err != nil {
		return err
	}

	for _, prom := range meta.cfg.PrometheusServers {
		prom.StartWorkers()
	}
	defer meta.cleanup()

	ctx := context.WithValue(context.Background(), config.CommandKey, config.CICommand)
	summary := checkRules(ctx, meta.workers, meta.cfg, entries)

	if c.Bool(requireOwnerFlag) {
		summary.Reports = append(summary.Reports, verifyOwners(entries)...)
	}

	reps := []reporter.Reporter{
		reporter.NewConsoleReporter(os.Stderr),
	}

	if meta.cfg.Repository != nil && meta.cfg.Repository.BitBucket != nil {
		token, ok := os.LookupEnv("BITBUCKET_AUTH_TOKEN")
		if !ok {
			return fmt.Errorf("BITBUCKET_AUTH_TOKEN env variable is required when reporting to BitBucket")
		}

		timeout, _ := time.ParseDuration(meta.cfg.Repository.BitBucket.Timeout)
		br := reporter.NewBitBucketReporter(
			version,
			meta.cfg.Repository.BitBucket.URI,
			timeout,
			token,
			meta.cfg.Repository.BitBucket.Project,
			meta.cfg.Repository.BitBucket.Repository,
			git.RunGit,
		)
		reps = append(reps, br)
	}

	if meta.cfg.Repository != nil && meta.cfg.Repository.GitHub != nil {
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

		timeout, _ := time.ParseDuration(meta.cfg.Repository.GitHub.Timeout)
		gr := reporter.NewGithubReporter(
			meta.cfg.Repository.GitHub.BaseURI,
			meta.cfg.Repository.GitHub.UploadURI,
			timeout,
			token,
			meta.cfg.Repository.GitHub.Owner,
			meta.cfg.Repository.GitHub.Repo,
			prNum,
			git.RunGit,
		)
		reps = append(reps, gr)
	}

	foundBugOrHigher := false
	bySeverity := map[string]interface{}{} // interface{} is needed for log.Fields()
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
