---
layout: default
title: Documentation
nav_order: 1
has_children: true
---

# pint

pint is a Prometheus rule linter.

## Usage

There are three modes it works in:

- CI PR linting
- Ad-hoc linting of a selected files or directories
- A daemon that continuously checks selected files or directories and expose metrics describing
  all discovered problems.

### Pull Requests

Run it with `pint ci`.

It currently supports git for which it will find all commits on the current branch that are not
present in the parent branch and scan all modified files included in those changes.

Results can optionally be reported using
[BitBucket API](https://docs.atlassian.com/bitbucket-server/rest/7.8.0/bitbucket-code-insights-rest.html)
or [GitHub API](https://docs.github.com/en/rest) to generate a report with any found issues.
If you are using BitBucket API then each issue will create an inline annotation in BitBucket with a description of
the issue. If you are using GitHub API then each issue will appear as a comment on your pull request.

Exit code will be one (1) if any issues were detected with severity `Bug` or higher. This permits running
`pint` in your CI system whilst at the same you will get detailed reports on your source control system.

If any commit on the PR contains `[skip ci]` or `[no ci]` somewhere in the commit message then pint will
skip running all checks.

### Ad-hoc

Lint specified files and report any found issue.

You can lint selected files:

```shell
pint lint rules.yml
```

or directories:

```shell
pint lint path/to/dir
```

or both:

```shell
pint lint path/to/dir file.yml path/file.yml path/dir
```

### Watch mode

Run pint as a daemon in watch mode:

```shell
pint watch rules.yml
```

By default it will start a HTTP server on port `8080` and run all checks every
10 minutes. This can be customized by passing extra flags to the `watch` command.
Run `pint watch -h` to see all available flags.

Query `/metrics` to see all expose metrics, example with default flags:

```shell
curl -s http://localhost:8080/metrics
```

Or setup Prometheus scrape job:

```yaml
scrape_configs:
  - job_name: pint
    static_configs:
      - targets: ['localhost:8080']
```

Available metrics:

- `pint_problem` - exported for every problem detected by pint.
  To avoid exposing too many metrics at once pass `--max-problems` flag to watch command.
  When this flag is set pint will expose only up to `--max-problems` value number of
  `pint_problem` metrics.
- `pint_problems` - this metric is the total number of all problems detected by pint,
  including those not exported due to the `--max-problems` flag.

`pint problem` metric can include `owner` label for each rule. This is useful
to route alerts based on metrics to the right team.
To set a rule owner add a `# pint file/owner $owner` comment in a file, to set
an owner for all rules in that file. You can also set an owner per rule, by adding
`# pint rule/owner $owner` comment around given rule.

Example:

```yaml
# pint file/owner bob

- alert: ...
  expr: ...

# pint rule/owner alice
- alert: ...
  expr: ...
```

Here's an example alert you can use for problems detected by pint:

{% raw %}
```yaml
- alert: Pint Problem Detected
  # pint_problem is only present if pint detects any problems
  # pint disable promql/series(pint_problem)
  expr: |
    sum without(instance, problem) (pint_problem) > 0
  for: 1h
  annotations:
    summary: |
      {{ with printf "pint_problem{filename='%s', name='%s', reporter='%s'}" .Labels.filename .Labels.name .Labels.reporter | query }}
        {{ . | first | label "problem" }}
      {{ end }}
    docs: "https://cloudflare.github.io/pint/checks/{{ $labels.reporter }}.html"
```
{% endraw %}

## Release Notes

See [changelog](changelog.md) for history of changes.

## Quick start

Requirements:

- [Git](https://git-scm.com/)
- [Go](https://golang.org/) - current stable release

Steps:

1. Download a binary from [Releases](https://github.com/cloudflare/pint/releases) page
  or build from source:

   ```shell
   git clone https://github.com/cloudflare/pint.git
   cd pint
   make
   ```

2. Run a simple syntax check on Prometheus
   [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)
   or [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)
   rules file(s).

   ```shell
   ./pint lint /etc/prometheus/*.rules.yml
   ```

3. Configuration file is optional, but without it pint will only run very basic
   syntax checks. See [configuration](configuration.md) for details on
   config syntax.
   By default pint will try to load configuration from `.pint.hcl`, you can
   specify a different path using `--config` flag:

   ```shell
   ./pint --config /etc/pint.hcl lint /etc/prometheus/rules/*.yml
   ```
