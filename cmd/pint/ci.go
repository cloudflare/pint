package main

import (
	"context"
	"fmt"
	"log/slog"
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

	"github.com/urfave/cli/v2"
)

var (
	baseBranchFlag = "base-branch"
	devFlag        = "dev"
	failOnFlag     = "fail-on"
)

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
		&cli.BoolFlag{
			Name:    devFlag,
			Aliases: []string{"n"},
			Value:   false,
			Usage:   "Use experimental change detection",
		},
		&cli.StringFlag{
			Name:    failOnFlag,
			Aliases: []string{"w"},
			Value:   "bug",
			Usage:   "Exit with non-zero code if there are problems with given severity (or higher) detected",
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

	excludeRe := []*regexp.Regexp{}
	for _, pattern := range meta.cfg.CI.Exclude {
		excludeRe = append(excludeRe, regexp.MustCompile("^"+pattern+"$"))
	}

	meta.cfg.CI = detectCI(meta.cfg.CI)
	baseBranch := meta.cfg.CI.BaseBranch
	if c.String(baseBranchFlag) != "" {
		baseBranch = c.String(baseBranchFlag)
	}
	currentBranch, err := git.CurrentBranch(git.RunGit)
	if err != nil {
		return fmt.Errorf("failed to get the name of current branch")
	}
	slog.Debug("Got branch information", slog.String("base", baseBranch), slog.String("current", currentBranch))
	if currentBranch == strings.Split(baseBranch, "/")[len(strings.Split(baseBranch, "/"))-1] {
		slog.Info("Running from base branch, skipping checks", slog.String("branch", currentBranch))
		return nil
	}

	var entries []discovery.Entry
	if c.Bool(devFlag) {
		finder := discovery.NewGitBranchFinder(git.RunGit, includeRe, excludeRe, baseBranch, meta.cfg.CI.MaxCommits, meta.cfg.Parser.CompileRelaxed())
		entries, err = finder.Find()
	} else {
		finder := discovery.NewGitBlameFinder(git.RunGit, includeRe, excludeRe, baseBranch, meta.cfg.CI.MaxCommits, meta.cfg.Parser.CompileRelaxed())
		entries, err = finder.Find()
	}
	if err != nil {
		return err
	}

	ctx := context.WithValue(context.Background(), config.CommandKey, config.CICommand)

	gen := config.NewPrometheusGenerator(meta.cfg, metricsRegistry)
	defer gen.Stop()

	if err = gen.GenerateStatic(); err != nil {
		return err
	}

	slog.Debug("Generated all Prometheus servers", slog.Int("count", gen.Count()))

	summary, err := checkRules(ctx, meta.workers, gen, meta.cfg, entries)
	if err != nil {
		return err
	}

	if c.Bool(requireOwnerFlag) {
		summary.Report(verifyOwners(entries, meta.cfg.Owners.CompileAllowed())...)
	}

	reps := []reporter.Reporter{
		reporter.NewConsoleReporter(os.Stderr, checks.Information),
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

		var prNum int
		if prNum, err = strconv.Atoi(prVal); err != nil {
			return fmt.Errorf("got not a valid number via GITHUB_PULL_REQUEST_NUMBER: %w", err)
		}

		timeout, _ := time.ParseDuration(meta.cfg.Repository.GitHub.Timeout)
		var gr reporter.GithubReporter
		if gr, err = reporter.NewGithubReporter(
			version,
			meta.cfg.Repository.GitHub.BaseURI,
			meta.cfg.Repository.GitHub.UploadURI,
			timeout,
			token,
			meta.cfg.Repository.GitHub.Owner,
			meta.cfg.Repository.GitHub.Repo,
			prNum,
			git.RunGit,
		); err != nil {
			return err
		}
		reps = append(reps, gr)
	}

	minSeverity, err := checks.ParseSeverity(c.String(failOnFlag))
	if err != nil {
		return fmt.Errorf("invalid --%s value: %w", failOnFlag, err)
	}

	problemsFound := false
	bySeverity := summary.CountBySeverity()
	for s := range bySeverity {
		if s >= minSeverity {
			problemsFound = true
			break
		}
	}
	if len(bySeverity) > 0 {
		slog.Info("Problems found", logSeverityCounters(bySeverity)...)
	}

	if err := submitReports(reps, summary); err != nil {
		return fmt.Errorf("submitting reports: %w", err)
	}

	if problemsFound {
		return fmt.Errorf("problems found")
	}

	return nil
}

func logSeverityCounters(src map[checks.Severity]int) (attrs []any) {
	for _, s := range []checks.Severity{checks.Fatal, checks.Bug, checks.Warning, checks.Information} {
		if c, ok := src[s]; ok {
			attrs = append(attrs, slog.Attr{Key: s.String(), Value: slog.IntValue(c)})
		}
	}
	return attrs
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
		slog.Debug("got base branch from GITHUB_BASE_REF env variable", slog.String("branch", bb))
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
			slog.Info("Setting GITHUB_PULL_REQUEST_NUMBER from GITHUB_REF env variable", slog.String("pr", parts[2]))
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
				slog.Info("Setting repository owner from GITHUB_REPOSITORY env variable", slog.String("owner", parts[0]))
				gh.Owner = parts[0]
				isDirty = true
			}
			if gh.Repo == "" {
				slog.Info("Setting repository name from GITHUB_REPOSITORY env variable", slog.String("repo", parts[1]))
				gh.Repo = parts[1]
				isDirty = true
			}
		}
	}

	if api := os.Getenv("GITHUB_API_URL"); api != "" {
		if gh.BaseURI == "" {
			slog.Info("Setting repository base URI from GITHUB_API_URL env variable", slog.String("baseuri", api))
			gh.BaseURI = api
		}
		if gh.UploadURI == "" {
			slog.Info("Setting repository upload URI from GITHUB_API_URL env variable", slog.String("uploaduri", api))
			gh.UploadURI = api
		}
	}

	if isNil && !isDirty {
		return nil
	}
	return gh
}
