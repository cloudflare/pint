---
layout: default
title: Documentation
nav_order: 1
has_children: true
---

# pint

pint is a Prometheus rule linter/validator.

## Requirements

pint will run checks on Prometheus alerting & recording rules to detect potential problems
with those rules.
Some checks rely only on the rule itself and can be run "offline" - without talking to any
Prometheus server.
You can run pint in "offline" if you:

- Don't pass any configuration file to pint.
- You pass configuration file to pint that **doesn't** contain any `prometheus` definition.
- You pass `--offline` flag to `pint` command.

Most checks included in pint will require sending queries to a running Prometheus server where
those rules are, or would be, deployed.
Those checks are enabled if you pass a configuration file to pint that includes at least one
`prometheus` block.
Checks might use various Prometheus
[HTTP API endpoints](https://prometheus.io/docs/prometheus/latest/querying/api/) to retrieve
extra information, for example Prometheus configuration or metrics metadata.
If you run pint against a different service, like [Thanos](https://thanos.io/) some checks
might return problems due to API call errors, since not all Prometheus HTTP APIs are supported by it.
In that case, you might want to disable failing checks in the pint configuration file.

## Usage

There are three modes it works in:

- CI PR linting
- Ad-hoc linting of a selected files or directories
- A daemon that continuously checks selected files or directories and expose metrics describing
  all discovered problems.

### Pull Requests

Run it with `pint ci`. Git is currently the only supported VCS.

When `pint ci` runs it will find all files in the current working directory and try to parse
them as Prometheus rules. Then it will look for all commits on the current branch that are not
present in the parent branch and to decide which rules were modified.
Checks are run only on modified rules but they require the full list of all rules to find any
cross-rule dependencies.

Running `pint ci` doesn't require any configuration but it's recommended to add a pint config file
with `ci` section containing at least the `include` option. This will ensure that pint validates
only Prometheus rules and ignores other files.

Results can optionally be reported using
[BitBucket API](https://developer.atlassian.com/server/bitbucket/rest/)
or [GitHub API](https://docs.github.com/en/rest) to generate a report with any found issues.

Exit code will be one (1) if any issues were detected with severity `Bug` or higher. This permits running
`pint` in your CI system whilst at the same you will get detailed reports on your source control system.

If any commit on the PR contains `[skip ci]` or `[no ci]` somewhere in the commit message then pint will
skip running all checks.

#### GitHub Actions

The easiest way of using `pint` with GitHub Actions is by using
[prymitive/pint-action](https://github.com/prymitive/pint-action).
Here's an example workflow:

{% raw %}

```yaml
name: pint

on:
  push:
    branches:
      - main
  pull_request:
    branches:
      - main

jobs:
  pint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run pint
        uses: prymitive/pint-action@v1
        with:
          token: ${{ github.token }}
          # directory containing Prometheus rules
          workdir: 'rules'
```

{% endraw %}

To customise pint checks create a `.pint.hcl` file in the root of your repository.
See [Configuration](configuration.md) for a description of all options.

If your repository contains other files, not only Prometheus rules, then tell pint
to only check selected paths when running checks on a pull request:

```js
ci {
  include    = [ "rules/dev/.*.yml", "rules/prod/.*" ]
}
```

When pint runs checks after a push to a branch (for example after a merge), then
it will pass `workdir` option to `pint lint`, which means that all files inside
`rules` directory will be checked.

### Ad-hoc

Check specified files and report any found issue.
You can pass directory paths and use [glob](https://pkg.go.dev/path/filepath#Match)
patterns as arguments to select files for checking.

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

Using glob patterns:

```shell
pint lint path/*.yml path/*.yaml
```

### Watch mode

Run pint as a daemon in watch mode where it continuously checks
all rules found in selected files and exposes metrics about
found problems.

#### Manually selecting files and directories

You can tell it to continuously test specific files or directories:

```shell
pint watch glob $GLOB_1 $GLOB_2 ... $GLOB_N
```

Example:

```shell
pint watch glob /etc/prometheus/rules-*.yml /etc/prometheus/rules.d
```

If provide a config file for pint with some Prometheus server definitions
then pint will also run "online" checks for it to, for example, ensure all
time series used inside your alerting rules are still present.
Example config:

```js
prometheus "local" {
  uri = "http://localhost:9090"
}
```

#### Getting list of files to check from Prometheus

You can also point pint directly at a Prometheus server from the config file.
On every iteration, before starting any checks, pint will query Prometheus API
to get the current value of `rule_files` Prometheus config option and then run
checks on all matching files.
This way if you test your rules against a running Prometheus instance then you don't
need to manually specify any paths or directories.

Usage:

```shell
pint watch rule_files $prometheus
```

Where `$prometheus` is the name of `prometheus` configuration block from pint
config file.

Example:

```shell
pint watch rule_files local
```

#### Accessing watch mode metrics

By default it will start a HTTP server on port `8080` and run all checks every
10 minutes. This can be customised by passing extra flags to the `watch` command.
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
  When this flag is set, pint will expose only up to `--max-problems` value number of
  `pint_problem` metrics.
- `pint_problems` - this metric is the total number of all problems detected by pint,
  including those not exported due to the `--max-problems` flag.

The `pint problem` metric can include the `owner` label for each rule. This is useful
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

## YAML parser

By default pint will expect all Prometheus rule files to be following the exact
syntax Prometheus expects for YAML files containing [recording](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)
and [alerting](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)
rules.
If you have Prometheus rules stored in YAML files with different YAML tree, but still
retain the same set of fields, for example:

```yaml
# Flat rule list
- alert: AlertName
  expr: up == 0
- record: sum:up
  expr: count(up == 1)
```

```yaml
# Rules nested under custom tree
service:
  prometheus:
    rules:
      - alert: AlertName
        expr: up == 0
      - record: sum:up
        expr: count(up == 1)
```

You can still check these rules using pint, but you need to switch pint YAML
parser into "relaxed" mode by adding this section to pint config file:

```js
parser {
  relaxed = [ "my/files/*.yml" ]
}
```

See [parser](configuration.md#parser) documentation for more details.
"Relaxed" parser mode will load anything that can be parsed as Prometheus rule,
while "strict" parser mode will fail if it reads a file that wouldn't load
cleanly as Prometheus config file.

## Control comments

There is a number of comments you can add to your rule files in order to change
pint behaviour, some of them allow you to exclude selected files or line, see
[docs here](./ignoring.md) for details.

There are a few requirements for any comment to be recognized by pint:

- All comments must have a `pint` prefix.
- All comments must have at least one space between `#` symbol and `pint` prefix.

Good comment examples:

```yaml
# pint file/owner bob
#   pint file/owner bob
```

Bad comment examples:

```yaml
# file/owner bob
#pint file/owner bob
```

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

3. Configuration file is optional, but without it, pint will only run very basic
   syntax checks. See [configuration](configuration.md) for details on
   config syntax.
   By default pint will try to load configuration from `.pint.hcl`, you can
   specify a different path using `--config` flag:

   ```shell
   ./pint --config /etc/pint.hcl lint /etc/prometheus/rules/*.yml
   ```

There are docker images available on [GitHub](https://github.com/cloudflare/pint/pkgs/container/pint).
Example usage:

```shell
docker run --mount=type=bind,source="$(pwd)",target=/rules,readonly ghcr.io/cloudflare/pint pint lint /rules
```

## License

```text
Copyright (c) 2021-2023 Cloudflare, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    
    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```
