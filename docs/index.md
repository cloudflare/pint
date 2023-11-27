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
In that case you might want to disable failing checks in pint configuration file.

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

## Control comments

There is a number of comments you can add to your rule files in order to change
pint behaviour, some of them allow you to exclude selected files or line, see
[docs here](./ignoring.md) for details.

There are a few requirement for any comment to be recognized by pint:

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

3. Configuration file is optional, but without it pint will only run very basic
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
