package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/config"
	"github.com/cloudflare/pint/internal/discovery"
	"github.com/cloudflare/pint/internal/git"
	"github.com/cloudflare/pint/internal/reporter"

	"github.com/urfave/cli/v3"
)

var (
	baseBranchFlag = "base-branch"
	failOnFlag     = "fail-on"
	teamCityFlag   = "teamcity"
	checkStyleFlag = "checkstyle"
	jsonFlag       = "json"
)

var ciCmd = &cli.Command{
	Name:   "ci",
	Usage:  "Run checks on all git changes.",
	Action: actionCI,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    requireOwnerFlag,
			Aliases: []string{"r"},
			Value:   false,
			Usage:   "Require all rules to have an owner set via comment.",
		},
		&cli.StringFlag{
			Name:    baseBranchFlag,
			Aliases: []string{"b"},
			Value:   "",
			Usage:   "Set base branch to use for PR checks (main, master, ...).",
		},
		&cli.StringFlag{
			Name:    failOnFlag,
			Aliases: []string{"w"},
			Value:   "bug",
			Usage:   "Exit with non-zero code if there are problems with given severity (or higher) detected.",
		},
		&cli.BoolFlag{
			Name:    teamCityFlag,
			Aliases: []string{"t"},
			Value:   false,
			Usage:   "Print found problems using TeamCity Service Messages format.",
		},
		&cli.StringFlag{
			Name:    checkStyleFlag,
			Aliases: []string{"c"},
			Value:   "",
			Usage:   "Write a checkstyle xml formatted report of all problems to this path.",
		},
		&cli.StringFlag{
			Name:    jsonFlag,
			Aliases: []string{"j"},
			Value:   "",
			Usage:   "Write a JSON formatted report of all problems to this path.",
		},
	},
}

func actionCI(ctx context.Context, c *cli.Command) error {
	meta, err := actionSetup(c)
	if err != nil {
		return err
	}

	baseBranch := detectBaseBranch(meta.cfg.CI.BaseBranch)
	if c.String(baseBranchFlag) != "" {
		baseBranch = c.String(baseBranchFlag)
	}
	gitInfo, err := git.Describe(git.RunGit)
	if err != nil {
		return fmt.Errorf("failed to get git info: %w", err)
	}
	currentBranch := detectCurrentBranch(gitInfo.CurrentBranch)
	slog.LogAttrs(ctx, slog.LevelDebug, "Got branch information", slog.String("base", baseBranch), slog.String("current", currentBranch))
	if currentBranch == strings.Split(baseBranch, "/")[len(strings.Split(baseBranch, "/"))-1] {
		slog.LogAttrs(ctx, slog.LevelInfo, "Running from base branch, skipping checks", slog.String("branch", currentBranch))
		return nil
	}

	slog.LogAttrs(ctx, slog.LevelInfo, "Finding all rules to check on current git branch", slog.String("base", baseBranch))

	filter := git.NewPathFilter(
		config.MustCompileRegexes(meta.cfg.Parser.Include...),
		config.MustCompileRegexes(meta.cfg.Parser.Exclude...),
		config.MustCompileRegexes(meta.cfg.Parser.Relaxed...),
	)

	allowedOwners := meta.cfg.Owners.CompileAllowed()
	var entries []*discovery.Entry
	entries, err = discovery.NewGlobFinder([]string{"*"}, filter, meta.cfg.Parser.Options(), allowedOwners).Find()
	if err != nil {
		return err
	}

	entries, err = discovery.NewGitBranchFinder(git.RunGit, filter, baseBranch, meta.cfg.CI.MaxCommits, meta.cfg.Parser.Options(), allowedOwners).Find(entries)
	if err != nil {
		return err
	}

	ctx = context.WithValue(ctx, config.CommandKey, config.CICommand)

	gen := config.NewPrometheusGenerator(meta.cfg, metricsRegistry)
	defer gen.Stop()

	gen.GenerateStatic()
	slog.LogAttrs(ctx, slog.LevelDebug, "Generated all Prometheus servers", slog.Int("count", gen.Count()))

	summary, err := checkRules(ctx, meta.workers, meta.isOffline, gen, meta.cfg, entries)
	if err != nil {
		return err
	}

	if c.Bool(requireOwnerFlag) {
		summary.Report(verifyOwners(entries, allowedOwners)...)
	}

	reps := []reporter.Reporter{}
	if c.Bool(teamCityFlag) {
		reps = append(reps, reporter.NewTeamCityReporter(os.Stderr))
	} else {
		reps = append(
			reps,
			reporter.NewConsoleReporter(os.Stderr, checks.Information, c.Bool(noColorFlag), c.Bool(showDupsFlag)),
		)
	}
	if c.String(checkStyleFlag) != "" {
		var f *os.File
		f, err = os.Create(c.String(checkStyleFlag))
		if err != nil {
			return err
		}
		defer f.Close()
		reps = append(reps, reporter.NewCheckStyleReporter(f))
	}
	if c.String(jsonFlag) != "" {
		var j *os.File
		j, err = os.Create(c.String(jsonFlag))
		if err != nil {
			return err
		}
		defer j.Close()
		reps = append(reps, reporter.NewJSONReporter(j))
	}

	if meta.cfg.Repository != nil && meta.cfg.Repository.BitBucket != nil {
		token, ok := os.LookupEnv("BITBUCKET_AUTH_TOKEN")
		if !ok {
			return errors.New("BITBUCKET_AUTH_TOKEN env variable is required when reporting to BitBucket")
		}

		timeout, _ := time.ParseDuration(meta.cfg.Repository.BitBucket.Timeout)
		br := reporter.NewBitBucketReporter(
			meta.cfg.Repository.BitBucket.URI,
			timeout,
			token,
			meta.cfg.Repository.BitBucket.Project,
			meta.cfg.Repository.BitBucket.Repository,
			gitInfo.CurrentBranch,
			gitInfo.HeadCommit,
			meta.cfg.Repository.BitBucket.MaxComments,
		)
		reps = append(reps, reporter.NewCommentReporter(br, c.Bool(showDupsFlag)))
	}

	if meta.cfg.Repository != nil && meta.cfg.Repository.GitLab != nil {
		token, ok := os.LookupEnv("GITLAB_AUTH_TOKEN")
		if !ok {
			return errors.New("GITLAB_AUTH_TOKEN env variable is required when reporting to GitLab")
		}

		timeout, _ := time.ParseDuration(meta.cfg.Repository.GitLab.Timeout)
		var gl reporter.GitLabReporter
		if gl, err = reporter.NewGitLabReporter(
			version,
			currentBranch,
			meta.cfg.Repository.GitLab.URI,
			timeout,
			token,
			meta.cfg.Repository.GitLab.Project,
			meta.cfg.Repository.GitLab.MaxComments,
		); err != nil {
			return err
		}
		reps = append(reps, reporter.NewCommentReporter(gl, c.Bool(showDupsFlag)))
	}

	meta.cfg.Repository = detectRepository(ctx, meta.cfg.Repository)
	if meta.cfg.Repository != nil && meta.cfg.Repository.GitHub != nil {
		token, ok := os.LookupEnv("GITHUB_AUTH_TOKEN")
		if !ok {
			return errors.New("GITHUB_AUTH_TOKEN env variable is required when reporting to GitHub")
		}

		prVal, ok := os.LookupEnv("GITHUB_PULL_REQUEST_NUMBER")
		if !ok {
			return errors.New("GITHUB_PULL_REQUEST_NUMBER env variable is required when reporting to GitHub")
		}

		var prNum int
		if prNum, err = strconv.Atoi(prVal); err != nil {
			return fmt.Errorf("got not a valid number via GITHUB_PULL_REQUEST_NUMBER: %w", err)
		}

		timeout, _ := time.ParseDuration(meta.cfg.Repository.GitHub.Timeout)
		var gr reporter.GithubReporter
		if gr, err = reporter.NewGithubReporter(
			ctx,
			version,
			meta.cfg.Repository.GitHub.BaseURI,
			meta.cfg.Repository.GitHub.UploadURI,
			timeout,
			token,
			meta.cfg.Repository.GitHub.Owner,
			meta.cfg.Repository.GitHub.Repo,
			prNum,
			meta.cfg.Repository.GitHub.MaxComments,
			gitInfo.HeadCommit,
			c.Bool(showDupsFlag),
		); err != nil {
			return err
		}
		reps = append(reps, reporter.NewCommentReporter(gr, c.Bool(showDupsFlag)))
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
		slog.LogAttrs(ctx, slog.LevelInfo, "Problems found", logSeverityCounters(bySeverity)...)
	}

	summary.SortReports()
	summary.Dedup()
	for _, rep := range reps {
		err = rep.Submit(ctx, summary)
		if err != nil {
			return fmt.Errorf("submitting reports: %w", err)
		}
	}

	if problemsFound {
		return errors.New("problems found")
	}

	return nil
}

func logSeverityCounters(src map[checks.Severity]int) (attrs []slog.Attr) {
	for _, s := range []checks.Severity{checks.Fatal, checks.Bug, checks.Warning, checks.Information} {
		if c, ok := src[s]; ok {
			attrs = append(attrs, slog.Attr{Key: s.String(), Value: slog.IntValue(c)})
		}
	}
	return attrs
}

// Normally we get the branch name from the config, but when running in CI
// we might have a git checkout that lacks branch information, so we need
// to get that name from ENV variables.
func detectCurrentBranch(branch string) string {
	if branch != "HEAD" {
		return branch
	}
	for _, key := range []string{
		"GITHUB_HEAD_REF",                     // GitHub
		"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME", // GitLab
		"CI_COMMIT_BRANCH",                    // GitLab
	} {
		if val := os.Getenv(key); val != "" {
			slog.LogAttrs(
				context.Background(), slog.LevelDebug,
				"Got current branch from environment variable",
				slog.String("key", key),
				slog.String("branch", val),
			)
			return val
		}
	}
	return branch
}

// Normally PRs would target the base branch (main) and that's what we assume,
// but we might have a pull request that is between two feature branches where the target
// is not the repos base branch (main), we can detect that from the ENV and set the
// base (target) branch accordingly.
func detectBaseBranch(branch string) string {
	for _, key := range []string{
		"GITHUB_BASE_REF",                     // GitHub
		"CI_MERGE_REQUEST_TARGET_BRANCH_NAME", // GitLab
	} {
		if val := os.Getenv(key); val != "" {
			slog.LogAttrs(
				context.Background(), slog.LevelDebug,
				"Got base branch from environment variable",
				slog.String("key", key),
				slog.String("branch", val),
			)
			return val
		}
	}
	return branch
}

func detectRepository(ctx context.Context, cfg *config.Repository) *config.Repository {
	if os.Getenv("GITHUB_ACTION") != "" {
		cfg.GitHub = detectGithubActions(ctx, cfg.GitHub)
	}
	if cfg != nil && cfg.GitHub != nil && cfg.GitHub.MaxComments == 0 {
		cfg.GitHub.MaxComments = 50
	}
	return cfg
}

func detectGithubActions(ctx context.Context, gh *config.GitHub) *config.GitHub {
	if os.Getenv("GITHUB_PULL_REQUEST_NUMBER") == "" &&
		os.Getenv("GITHUB_EVENT_NAME") == "pull_request" &&
		os.Getenv("GITHUB_REF") != "" {
		parts := strings.Split(os.Getenv("GITHUB_REF"), "/")
		if len(parts) >= 4 {
			slog.LogAttrs(ctx, slog.LevelInfo, "Setting GITHUB_PULL_REQUEST_NUMBER from GITHUB_REF env variable", slog.String("pr", parts[2]))
			os.Setenv("GITHUB_PULL_REQUEST_NUMBER", parts[2])
		}
	}

	var isDirty, isNil bool

	if gh == nil {
		isNil = true
		gh = &config.GitHub{Timeout: time.Minute.String()} // nolint: exhaustruct
	}

	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) == 2 {
			if gh.Owner == "" {
				slog.LogAttrs(ctx, slog.LevelInfo, "Setting repository owner from GITHUB_REPOSITORY env variable", slog.String("owner", parts[0]))
				gh.Owner = parts[0]
				isDirty = true
			}
			if gh.Repo == "" {
				slog.LogAttrs(ctx, slog.LevelInfo, "Setting repository name from GITHUB_REPOSITORY env variable", slog.String("repo", parts[1]))
				gh.Repo = parts[1]
				isDirty = true
			}
		}
	}

	if api := os.Getenv("GITHUB_API_URL"); api != "" {
		if gh.BaseURI == "" {
			slog.LogAttrs(ctx, slog.LevelInfo, "Setting repository base URI from GITHUB_API_URL env variable", slog.String("baseuri", api))
			gh.BaseURI = api
		}
		if gh.UploadURI == "" {
			slog.LogAttrs(ctx, slog.LevelInfo, "Setting repository upload URI from GITHUB_API_URL env variable", slog.String("uploaduri", api))
			gh.UploadURI = api
		}
	}

	if isNil && !isDirty {
		return nil
	}
	return gh
}
