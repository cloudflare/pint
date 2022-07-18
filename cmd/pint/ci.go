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

	meta.cfg.CI = detectCI(meta.cfg.CI)
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

	meta.cfg.Repository = detectRepository(meta.cfg.Repository)
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

func detectCI(cfg *config.CI) *config.CI {
	var isNil, isDirty bool

	if cfg == nil {
		isNil = true
		cfg = &config.CI{}
	}

	if bb := os.Getenv("GITHUB_BASE_REF"); bb != "" {
		isDirty = true
		cfg.BaseBranch = bb
	}

	if isNil && !isDirty {
		return nil
	}
	return cfg
}

func detectRepository(cfg *config.Repository) *config.Repository {
	var isNil, isDirty bool

	if cfg == nil {
		isNil = true
		cfg = &config.Repository{}
	}

	if os.Getenv("GITHUB_ACTION") != "" {
		isDirty = true
		cfg.GitHub = detectGithubActions(cfg.GitHub)
	}

	if isNil && !isDirty {
		return nil
	}
	return cfg
}

func detectGithubActions(gh *config.GitHub) *config.GitHub {
	if os.Getenv("GITHUB_PULL_REQUEST_NUMBER") == "" &&
		os.Getenv("GITHUB_EVENT_NAME") == "pull_request" &&
		os.Getenv("GITHUB_REF") != "" {
		parts := strings.Split(os.Getenv("GITHUB_REF"), "/")
		if len(parts) >= 4 {
			log.Info().Str("pr", parts[2]).Msg("Setting GITHUB_PULL_REQUEST_NUMBER from GITHUB_REF env variable")
			os.Setenv("GITHUB_PULL_REQUEST_NUMBER", parts[2])
		}
	}

	var isDirty, isNil bool

	if gh == nil {
		isNil = true
		gh = &config.GitHub{Timeout: time.Minute.String()}
	}

	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) == 2 {
			if gh.Owner == "" {
				log.Info().Str("owner", parts[0]).Msg("Setting repository owner from GITHUB_REPOSITORY env variable")
				gh.Owner = parts[0]
				isDirty = true
			}
			if gh.Repo == "" {
				log.Info().Str("repo", parts[1]).Msg("Setting repository name from GITHUB_REPOSITORY env variable")
				gh.Repo = parts[1]
				isDirty = true
			}
		}
	}

	if api := os.Getenv("GITHUB_API_URL"); api != "" {
		if gh.BaseURI == "" {
			log.Info().Str("baseuri", api).Msg("Setting repository base URI from GITHUB_API_URL env variable")
			gh.BaseURI = api
		}
		if gh.UploadURI == "" {
			log.Info().Str("uploaduri", api).Msg("Setting repository upload URI from GITHUB_API_URL env variable")
			gh.UploadURI = api
		}
	}

	if isNil && !isDirty {
		return nil
	}
	return gh
}
