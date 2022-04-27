# Changelog

## v0.18.0

### Added

- Allow fine tuning `promql/series` check with extra control comments
  `# pint rule/set promql/series min-age ...` and
  `# pint rule/set promql/series ignore/label-value ...`
  See [promql/series](checks/promql/series.md) for details.
- `promql/regexp` will report redundant use of regex anchors.

### Changed

- `promql/series` will now report missing metrics only if they were last seen
  over 2 hours ago by default. This can be customized per rule with comments.

## v0.17.7

### Fixed

- Fix problem line reporting for `rule/owner` check.
- Add missing `rule/owner` documentation page.

## v0.17.6

### Fixed

- Fixed false positive reports from `promql/series` check when running
  `pint watch`.

## v0.17.5

### Added

- Added `pint_last_run_duration_seconds` metric.
- Added `--require-owner` flag support to `pint ci` command.

### Fixed

- Better handling of YAML unmarshal errors.

## v0.17.4

### Fixed

- Fixed false positive reports from `alerts/template` check when `absent()` is
  used inside a binary expression.

## v0.17.3

### Fixed

- File parse errors didn't report correct line numbers when running `pint ci`.

## v0.17.2

### Fixed

- File parse errors were not reported correctly when running `pint ci`.

## v0.17.1

### Fixed

- Handle `504 Gateway Timeout` HTTP responses from Prometheus same as query
  timeouts and retry with a shorter range query.

## v0.17.0

### Added

- When running `pint ci` all checks will be skipped if any commit contains
  `[skip ci]` or `[no ci]` string in the commit message.

### Changed

- By default pint will now parse all files in strict mode, where all rule files
  must have the exact syntax Prometheus expects:

  ```yaml
  groups:
  - name: example
    rules:
    - record: ...
      expr: ...
  ```

  Previous releases were only looking for individual rules so `groups` object
  wasn't required. Now pint will fail to read any file that doesn't follow
  Prometheus syntax exactly.
  To enable old behavior add `parser { relaxed = ["(.+)", ...]}` option in
  the config file. See [Configuration](configuration.md) for details.
  To enable old (relaxed) behavior for all files add:

  ```yaml
  parser {
    relaxed = ["(.*)"]
  }
  ```

### Fixed

- Improved `promql/vector_matching` checks to detect more issues.
- Fixed reporting of problems detected on unmodified lines when running `pint ci`.

## v0.16.1

### Fixed

- Fixed false positive reports from `alerts/template` check when `absent()` function
  is receiving labels from a binary expression.

## v0.16.0

### Added

- When running `pint watch` exported metric can include `owner` label for each rule.
  This is useful to route alerts based on `pint_problem` metrics to the right team.
  To set a rule owner add a `# pint file/owner $owner` comment in a file, to set
  an owner for all rules in that file. You can also set an owner per rule, by adding
  `# pint rule/owner $owner` comment around given rule.
  To enforce ownership comments in all files pass `--require-owner` flag to `pint lint`.

## v0.15.7

### Fixed

- `promql/series` check no longer runs duplicated checks on source metrics when
  a query depends on a recording rule added in the same PR.

## v0.15.6

### Fixed

- `promql/series` check was reporting that a metric stopped being exported when check
  queries would require a few retries.

## v0.15.5

### Fixed

- `promql/series` check was reporting both `Warning` and `Bug` problems for the
  same metric when it was using newly added recording rule.

## v0.15.4

### Fixed

- Fixed false positive reports from `promql/fragile` when `foo OR bar` is used inside
  aggregation.

## v0.15.3

### Fixed

- Use more efficient queries for `promql/series` check.
- Fixed YAML parsing panics detected by Go 1.18 fuzzing.

## v0.15.2

### Fixed

- Improved query cache hit rate and added `pint_prometheus_cache_hits_total` metric
  to track the number of cache hits.

## v0.15.1

### Added

- When a range query returns `query processing would load too many samples into memory`
  error and we retry it with smaller time range cache this information and start with
  that smaller time range for future calls to speed up running `pint watch`.

## v0.15.0

### Changed

- Always print the number of detected problems when running `pint lint`.
- `promql/series` check was refactored and will now detect a range of
  problems. See [promql/series](checks/promql/series.md) for details.
- `promql/regexp` severity is now `Bug` instead of a `Warning`.
- `promql/rate` check will no longer produce warnings, it will only
  report issues that cause queries to never return anything.

## v0.14.0

### Added

- Allow matching alerting rules by `for` field - #148. Example:

  ```js
  rule {
    match {
      for = ">= 10m"
    }
  }
  ```
- Regexp matchers used in check rules can now reference rule fields.
  See [Configuration](configuration.md) for details.

### Changed

- Added `filename` label to `pint_problem` metric - #170.
- Include Prometheus server URI in reported problems.

### Fixed

- Fixed `pint ci` handling when a file was added to git and then removed in the
  next commit.

## v0.13.2

### Fixed

- `yaml/parse` was using incorrect line numbers for errors caused by duplicated
  YAML keys.

## v0.13.1

### Fixed

- Don't use failover Prometheus servers in case of errors caused by the query
  itself, like `many-to-many matching not allowed`.

## v0.13.0

### Added

- `yaml/parse` error will be raised if a rule file contains duplicated keys, example:

  ```yaml
  - record: foo
    expr: sum(my_metric)
    expr: sum(my_metric) without(instance)
  ```

### Changed

- `prometheus` config block now allows to specify failover URIs using `failover` field.
  If failover URIs are set and main URI fails to respond pint will attempt to use them
  in the order specified until one of them works.
- `prometheus` config block now allows to define how upstream errors are handled using
  `required` field. If `required` is set to `true` any check that depends on remote
  Prometheus server will be reported as `bug` if it's unable to talk to it.
  If `required` is set to `false` pint will only emit `warning` level results.
  Default value for `required` is `false`. Set it to `true` if you want to hard fail
  in case of remote Prometheus issues. Note that setting it to `true` might block
  PRs when running `pint ci` until pint is able to talk to Prometheus again.
- Renamed `pint/parse` to `yaml/parse` and added missing documentation for it.

## v0.12.0

### Added

- Added  `pint_last_run_time_seconds` and `pint_rules_parsed_total` metrics when running `pint watch`.

### Changed

- `promql/comparison` only applies to alerts, so it was renamed to
  `alerts/comparison`.
- Online documentation hosted at [cloudflare.github.io/pint](https://cloudflare.github.io/pint/)
  was reworked.
- `alerts/count` check will now retry range queries with shorter time window
  on `found duplicate series for the match group ...` errors from Prometheus.

## v0.11.1

### Fixed

- `pint_prometheus_queries_total` and `pint_prometheus_query_errors_total` metrics
  were not incremented correctly.

## v0.11.0

### Added

- Added `promql/regexp` check that will warn about unnecessary regexp matchers.
- Added  `pint_prometheus_queries_total` and `pint_prometheus_query_errors_total`
  metric when running `pint watch`.

## v0.10.1

### Fixed

- Fixed a number of bug with `promql/vector_matching` check.

## v0.10.0

### Changed

- `query/series` check was renamed to `promql/series`.

### Fixed

- Improved the logic of `promql/vector_matching` check.

## v0.9.0

### Changed

- Removed `lines` label from `pint_problem` metric exported when running `pint watch`.
- Multiple `match` and `ignore` blocks can now be specified per each `rule`.

## v0.8.2

### Added

- Export `pint_version` metric when running `pint watch`.
- Added `--min-severity` flag to `pint watch` command.

## v0.8.1

### Added

- Added `--max-problems` flag to `pint watch` command.

### Changed

- Updated Prometheus modules to [v2.33.0](https://github.com/prometheus/prometheus/releases/tag/v2.33.0).
  This adds support for `stripPort` template function.

## v0.8.0

### Added

- Added new `promql/fragile` check.
- BitBucket reports will now include a link to documentation.

## v0.7.3

### Added

- `--workers` flag to control the number of worker threads for running checks.

## v0.7.2

### Changed

- More aggressive range reduction for `query processing would load too many samples into memory`
  errors when sending range queries to Prometheus servers.

## v0.7.1

### Added

- Added `command` filter to `match` / `ignore` blocks. This allows to include
  skip some checks when (for example) running `pint watch` but include them
  in `pint lint` run.

## v0.7.0

### Added

- Cache each Prometheus server responses to minimize the number of API calls.
- `pint watch` will start a daemon that will continuously check all matching rules
  and expose metrics describing all discovered problems.

### Changed

- `alerts/annotation` and `rule/label` now include `required` flag value in
  `# pint disable ...` comments.
  Rename `# pint disable alerts/annotation($name)` to
  `# pint disable alerts/annotation($name:$required)` and
  `# pint disable rule/label($name)` to `# pint disable rule/label($name:$required)`.
- `--offline` and `--disabled` flags are now global, use `pint --offline lint` instead
  of `pint lint --offline`.

### Fixed

- `promql/rate`, `query/series` and `promql/vector_matching` checks were not enabled
  for all defined `prometheus {}` blocks  unless there was at least one `rule {}` block.
- `annotation` based `match` blocks didn't work correctly.

## v0.6.6

### Fixed

- File renames were not handled correctly when running `git ci` on branches with
  multiple commits.

## v0.6.5

### Added

- Allow disabling `query/series` check for individual series using
  `# pint disable query/series(my_metric_name)` comments.

## v0.6.4

### Fixed

- Fixed docker builds.

## v0.6.3

### Fixed

- `aggregate` check didn't report stripping required labels on queries
  using aggregation with no grouping labels (`sum(foo)`).
- `aggregate` check didn't test for name and label matches on alert rules.

## v0.6.2

### Changed

- `template` check will now include alert query line numbers when reporting issues.

## v0.6.1

### Fixed

- Labels returned by `absent()` are only from equal match types (`absent(foo="bar")`,
  not `absent(foo=~"bar.+")` but `alerts/template` didn't test for match type when
  checking for labels sourced from `absent()` queries.

## v0.6.0

### Changed

- `aggregate` check was refactored and uses to run a single test for both
  `by` and `without` conditions. As a result this check might now find issues
  previously undetected.
  Check suppression comments will need to be migrated:
  * `# pint disable promql/by` becomes `# pint disable promql/aggregate`
  * `# pint disable promql/without` becomes `# pint disable promql/aggregate`
  * `# pint ignore promql/by` becomes `# pint ignore promql/aggregate`
  * `# pint ignore promql/without` becomes `# pint ignore promql/aggregate`

## v0.5.3

### Fixed

- Fixed false positive reports in `aggregate` check.

## v0.5.2

### Added

- `--no-color` flag for disabling output colouring.

### Fixed

- Fixed duplicated warnings when multiple `rule {...}` blocks where configured.

## v0.5.1

### Fixed

- Specifying multiple `# pint disable ...` comments on a single rule would only apply
  last comment. This now works correctly and all comments will be applied.

## v0.5.0

### Added

- Added `alerts/for` check that will look for invalid `for` values in alerting rules.
  This check is enabled by default.

### Changed

- `comparison` check is now enabled by default and require no configuration.
  Remove `comparison{ ... }` blocks from pint config file when upgrading.
- `template` check is now enabled by default and require no configuration.
  Remove `template{ ... }` blocks from pint config file when upgrading.
- `rate` check is now enabled by default for all configured Prometheus servers.
  Remove `rate{ ... }` blocks from pint config file when upgrading.
- `series` check is now enabled by default for all configured Prometheus servers.
  Remove `series{ ... }` blocks from pint config file when upgrading.
- `vector_matching` check is now enabled by default for all configured Prometheus servers.
  Remove `vector_matching{ ... }` blocks from pint config file when upgrading.

## v0.4.4

### Added

- Support `parseDuration` function in alert templates added in Prometheus 2.32.0

## v0.4.3

### Fixed

- Fixed `series` check handling of queries with `{__name__="foo"}` selectors.

## v0.4.2

### Fixed

- Fixed `template` check handling of `absent` calls on aggregated metrics, like
  `absent(sum(nonexistent{job="myjob"}))`.

## v0.4.1

### Added

- `template` check will now warn if any template is referencing a label that is not passed to
  `absent()`.
  Example:

  {% raw %}
  ```yaml
  - alert: Foo
    expr: absent(foo{env="prod"})
    annotations:
      summary: 'foo metric is missing for job {{ $labels.job }}'
  ```
  {% endraw %}

  Would generate a warning since `absent()` can only return labels that are explicitly
  passed to it and the above call only passes `env` label.
  This can be fixed by updating the query to `absent(foo{env="prod", job="bar"})`.

## v0.4.0

### Added

- `comparison` check will now warn when alert query uses
  [bool](https://prometheus.io/docs/prometheus/latest/querying/operators/#comparison-binary-operators)
  modifier after condition, which can cause alert to always fire.
  Example:

  ```yaml
  - alert: Foo
    expr: rate(error_count[5m]) > bool 5
  ```

  Having `bool` as part of `> 5` condition means that the query will return value `1` when condition
  is met, and `0` when it's not. Rather than returning value of `rate(error_count[5m])` only when
  that value is `> 5`. Since all results of an alerting rule `expr` are considered alerts such alert
  rule could always fire, regardless of the value returned by `rate(error_count[5m])`.

### Fixed

- `comparison` check will now ignore `absent(foo)` alert queries without any condition.

## v0.3.1

### Added

- `--offline` flag for `pint ci` command.

### Fixed

- Fixed `template` check panic when alert query had a syntax error.

## v0.3.0

### Added

- `rule` block can now specify `ignore` conditions that have the same syntax as `match`
  but will disable `rule` for matching alerting and recording rules #48.
- `match` and `ignore` blocks can now filter alerting and recording rules by name.
  `record` will be used as name for recording rules and `alert` for alerting rules.

## v0.2.0

### Added

- `--offline` flag for `pint lint` command. When passed only checks that don't send
  any live queries to Prometheus server will be run.
- `template` check will now warn if template if referencing a label that is being
  stripped by aggregation.
  Example:

  {% raw %}
  ```yaml
  - alert: Foo
    expr: count(up) without(instance) == 0
    annotations:
      summary: 'foo is down on {{ $labels.instance }}'
  ```
  {% endraw %}

  Would generate a warning since `instance` label is being stripped by `without(instance)`.

## v0.1.5

### Fixed

- Fixed file descriptor leak due to missing file `Close()`  #69.

## v0.1.4

### Changed

- Retry queries that error with `query processing would load too many samples into memory`
  using a smaller time range.

## v0.1.3

### Added

- `vector_matching` check for finding queries with incorrect `on()` or `ignoring()`
  keywords.

### Fixed

- `comparison` check would trigger false positive for rules using `unless` keyword.

## v0.1.2

### Fixed

- `# pint skip/line` place between `# pint skip/begin` and `# pint skip/end` lines would
  reset ignore rules causing lines that should be ignored to be parsed. 

## v0.1.1

### Changed

- `value` check was replaced by `template`, which covers the same functionality and more.
  See [docs](/docs/CONFIGURATION.md#template) for details.
