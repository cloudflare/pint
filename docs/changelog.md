# Changelog

## v0.65.0

### Added

- [promql/regexp](checks/promql/regexp.md) check will now look for smelly regexp selectors.
  See check docs for details.
- [promql/range_query](checks/promql/range_query.md) now allows to configure a custom maximum
  duration for range queries - #1064.
- Added `--enabled` flag to the pint command. Passing this flag will only run selected check(s).
- Added `state` option to the rule `match` block. See [configuration](configuration.md) docs for details.

### Fixed

- Don't try to report problem on unmodified files when using GitHub reporter.
- If there is a pint config file present then pint will now always add it to the `parser` block `exclude` list.
  This is to avoid trying to parse it as a rule file if it's included in the same folder as rules.

## v0.64.1

### Fixed

- Validate `ignoreLabelsValue` option values in the pint config.

## v0.64.0

### Added

- Added `ignoreLabelsValue` to [promql/series](checks/promql/series.md) check settings.
- Added `include` & `exclude` options were moved from the `ci` to the `parser` configuration block.
  They now apply to all pint commands, not just `pint ci` - #631.

## v0.63.0

### Added

- Added [alerts/absent](checks/alerts/absent.md) check.
- Added [rule/name](checks/rule/name.md) check - #1020.

### Changed

- [promql/vector_matching](checks/promql/vector_matching.md) will now report more details, including which Prometheus server reports problems and which part of the query is the issue.
- GitHub report code was refactored, it should behave as before.
- When running `pint ci` pint will now run all checks on unmodified rules when a `file/disable` comment was removed.

### Fixed

- Fixed false positive warnings from [alerts/comparison](checks/alerts/comparison.md) when using `absent_over_time()`.
- GitHub reporter will now wait before making more requests if it's rate limited - #699.

## v0.62.2

### Fixed

- When using BitBucket reporter `pint ci` might create a comment longer than the limit allowed by BitBucket.
  To avoid this pint will now truncate long comments.

## v0.62.1

### Fixed

- Fixed false positive warnings from [rule/dependency](checks/rule/dependency.md) check.

## v0.62.0

### Added

- [promql/series](checks/promql/series.md) check will now generate warnings if there are `# pint disable`
  or `# pint rule/set` comments that are not matching any valid query selector or Prometheus server.

### Fixed

- [promql/series](checks/promql/series.md) will now parse `rule/set` comments that target specific
  time series selectors in PromQL the same way as `# pint disable` comments do.

## v0.61.2

### Fixed

- Fixed false positive reports about removed rules when running `pint ci` on a branch that
  contains YAML syntax errors.

## v0.61.1

### Fixed

- Fixed release workflow on GitHub.

## v0.61.0

### Fixed

- Don't suggest using `humanize` when alert template is already using printf on format the `$value`.
- Fixed git history parsing when running `pint ci` on a branch that include merge commits.

### Changed

- Refactored YAML syntax checks to avoid using [rulefmt.Parse](https://pkg.go.dev/github.com/prometheus/prometheus@v0.53.0/model/rulefmt#Parse) and effectively parsing rules twice. Some error messages will have different formatting.

## v0.60.0

### Fixed

- [alerts/annotations](checks/alerts/annotation.md) and [rule/label](checks/rule/label.md) will
  now report when a label or annotation is required but set to an empty value.
- Fixed handling of `# pint ignore/line` comments on line including multiple `#` characters.
- When reporting problems all messages will now use `publicURI` from each `prometheus` definition.

## v0.59.0

### Added

- Added GitLab support when running `pint ci`.

### Changed

- Reduced severity of problems reported by [promql/counter](checks/promql/counter.md) due
  to high number of false positives. It will now report warnings instead of errors.

### Fixed

- Fixed false positive reports from [rule/duplicate](checks/rule/duplicate.md) when a rule file
  is being deleted and the rules are moved to a different file.

## v0.58.1

### Fixed

- Correctly handle default Prometheus retention value in [promql/range_query](checks/promql/range_query.md)
  check - #958.

## v0.58.0

### Fixed

- Fixed false positives from [rule/duplicate](checks/rule/duplicate.md) check
  when running `pint ci` on files that are both edited and renamed in the same PR.
- [rule/dependency](checks/rule/dependency.md) will no longer report removed symlinks.

## v0.57.3

### Fixed

- Fixed compatibility with older git releases.
- Fixed `maxComments` defaulting to zero when using GitHub reporter.
- Fixed `maxComments` comment creation on GitHub - #935.

## v0.57.2

### Fixed

- Fixed release workflow on GitHub actions.

## v0.57.0

### Added

- GitHub and BitBucket reporters now supports `maxComments` option to limit the number
  of comments pint can create on a single pull request.
  Default value of `maxComments` is `50`.

## v0.56.2

### Fixed

- [query/cost](checks/query/cost.md) will now only create reports if a query is more
  expensive than any of the configured limits.
- Fixed duplicated reports when using BitBucket reporter.

## v0.56.1

### Fixed

- [promql/counter](checks/promql/counter.md) check will now consider a metric
  to be a counter only if all metadata entries for it use `TYPE counter`.
  Previously it would check for at least one metadata entry with `TYPE counter`.

## v0.56.0

### Added

- Added [promql/counter](checks/promql/counter.md) check - #655.
- Check using custom rules now accept an optional `comment` option for adding
  a text comment to all reported problems.
  Checks supporting `comment` option:
  - [alerts/annotation](checks/alerts/annotation.md)
  - [alerts/count](checks/alerts/count.md)
  - [promql/aggregate](checks/promql/aggregate.md)
  - [query/cost](checks/query/cost.md)
  - [rule/for](checks/rule/for.md)
  - [rule/label](checks/rule/label.md)
  - [rule/link](checks/rule/link.md)
  - [rule/reject](checks/rule/reject.md)
  Example:

  ```js
  for {
    comment  = "All alert rules must have `for: 5m` (or more) to avoid flaky alerts."
    severity = "bug"
    min      = "5m"
  }
  ```

## v0.55.0

### Added

- [labels/conflict](checks/labels/conflict.md) check will now warn if alerting
  rule is setting labels that are already configured as external labels.

### Changed

- `pint watch` command now has two sub-commands: `pint watch glob` and `pint watch rule_files`.
  See [watch mode](index.md#watch-mode) docs for details.
- [promql/series](checks/promql/series.md) check will now ignore missing metrics if the
  query uses a fallback with `or vector(1)`. This will reduce the number of reported
  missing metrics for queries where the intention is to accept and ignore missing
  metrics.
  Example of a rule that will no longer report problems:

  ```yaml
  - alert: Foo
    expr: sum(my_metric or vector(0)) > 1
  ```

### Fixed

- Fixed false positive reports from [rule/duplicate](checks/rule/duplicate.md) when
  using symlinks.
- Fixed support for multi-document YAML files when using relaxed parsed mode - #746.

## v0.54.0

### Added

- [rule/dependency](checks/rule/dependency.md) check will now warn if an alerting rule
  that's being removed in a pull request is being used inside `ALERTS{alertname="..."}`
  or  `ALERTS_FOR_STATE{alertname="..."}` queries.
- pint will now perform extra validation of YAML files to ensure that all values
  are mappings and strings.
  This will error on rules where, for example, label value is unquoted number:

  Bad rule:

  ```yaml
  - alert: DeadMansSwitch
    expr: vector(1)
    labels:
      priority: 5
  ```

  Good rule:

  ```yaml
  - alert: DeadMansSwitch
    expr: vector(1)
    labels:
      priority: "5"
  ```

### Changed

- A large part of rule parsing code was refactored and more problems will now be deduplicated.
- Improved validation of labels and annotations.

## v0.53.0

### Added

- pint will now report any comment that looks like a [pint control comment](./ignoring.md)
  but cannot be parsed correctly.

### Fixed

- Fixed a crash when running `pint ci` and using `ignore/file` comments.

## v0.52.0

### Added

- Both [alerts/annotation](checks/alerts/annotation.md) and [rule/label](checks/rule/label.md)
  now support more advance validation of label and annotation values with extra `token` option.
  In addition to the `value` regexp matching you can also validate values against a static
  list of allowed values using new `values` option.
  See both checks documentation for detail.

### Changed

- More reports will now be merged into a single comments when using BitBucket.

### Fixed

- Fixed YAML anchor parsing.
- Fixed regexp matching for label names in [rule/label](checks/rule/label.md).

## v0.51.1

### Fixed

- Fixed a few bugs in control comment parsing code.
- BitBucket tasks were not marked as resolved correctly.

## v0.51.0

### Added

- Added [rule/dependency](checks/rule/dependency.md) check.

### Changed

- When running `pint ci` pint will now first try to parse all files in current
  working directory, before checking for files modified on current branch.
  This is to have a full list of all rules, which is needed for checks like
  newly added [rule/dependency](checks/rule/dependency.md).
  This can slow `pint` runs if there's a lot of files in your repository.
  If there are non-rule files these may fail to parse and result in check errors.
  To avoid any errors or slowdowns from scanning unrelated files you might need
  to add `ci` section to `.pint.hcl` with `include` and/or `exclude` options set.
  See [examples/ci.hcl](examples/ci.hcl) for an example config.
- `Bug` and `Fatal` severity problems are now reported as tasks when using
  BitBucket.
- [promql/regexp](checks/promql/regexp.md) will now check for more problems
  with vector selectors.

### Fixed

- `pint` will no longer run dynamic Prometheus discovery when `--offline` flag
  is passed.

## v0.50.1

### Fixed

- Fixed alert preview link on `alerts/count` reports.

## v0.50.0

### Changed

- `pint ci` now uses a new logic for deciding which rules have changed when validating
  pull requests. Changes that would previously be invisible to pint, like modifying
  comments or moving the entire rules file, will now trigger checks for affected
  rules.
- pint will now try to create fewer BitBucket comments by merging multiple
  problem reports into a single comment.
- Control comment handling code was refactored, there are some additional rules
  that comment must follow. See `Control comments` section in [pint docs](./index.md).

## v0.49.2

### Fixed

- Fixed false positive reports from `promql/regexp` - #782.
- `Information` level reports using BitBucket were using wrong comment icon.

## v0.49.1

### Fixed

- `alerts/count` check wasn't using `uptime` field from `prometheus` config blocks
  for metric gap detection.

## v0.49.0

### Added

- Added [alerts/external_labels](checks/alerts/external_labels.md) check.
- Added support for reporting problems to TeamCity using [Service Messages](https://www.jetbrains.com/help/teamcity/service-messages.html).
  To enable run it run `pint --teamcity lint` or `pint --teamcity ci`.
- Problems reported to BitBucket and GitHub will now include more details.
- Added `publicURI` field to `prometheus` configuration blocks.

### Changed

- When [promql/series](checks/promql/series.md) finds that a time series used
  by a rule is missing it will now also check other defined Prometheus servers
  and add that information to the report.
  This allows pint to flag rules that are most likely deployed to the wrong servers,
  using missing scrape jobs.
- Reporting problems to BitBucket will now use comments instead of annotations.
  This is only if there is an  open pull request for tested branch, if there is no
  open pull request problems will be reported using code insight annotations.

## v0.48.2

### Fixed

- Fixed `pint watch` to correctly work with `discovery`.

## v0.48.1

- Prometheus `template` under `discovery` block can now template
  `include` and `exclude` fields.

## v0.48.0

### Added

- Prometheus servers can now be dynamically configured using one of
  supported discovery mechanism: file paths or Prometheus metrics query.

## v0.47.0

### Added

- `alerts/template` check can now report problems with alerting rules
  when trying to use templates on a query that doesn't produce any labels
  at all.
  For example when using [`vector(...)`](https://prometheus.io/docs/prometheus/latest/querying/functions/#vector) function:

  {% raw %}

  ```yaml
  - alert: DeadMansSwitch
    expr: vector(1)
    annotations:
      summary: Deadman's switch on {{ $labels.instance }} is firing
  ```

  {% endraw %}

- `alerts/comparison` check can now warn if alerting rules use a query
  with `foo > 0 OR vector(1)`, which would always fire.
- `alerts/template` check will now look check `on(...)` clause on binary
  expressions. When `on(...)` is set only labels listed there will appear
  on result metrics.
  For example `app_type` here cannot appear on query results, even if it's
  present on `foo` time series.

  ```yaml
  - alert: ...
    expr: foo / on (instance, app_name) bar
    annotations:
      summary: ... {{ $labels.app_type }} ...
  ```

## v0.46.0

### Added

- Added support for `keep_firing_for` in alerting rules - #713.
- Added `rule/keep_firing_for` check - #713.
- Added `alerts/count` check will now estimate alerts using
  `keep_firing_for` field if set - #713.
- Configuration rule `match` block supports a new filter `keep_firing_for`.

## v0.45.0

### Added

- The `query/cost` check can now use Prometheus query stats to verify query
  evaluation time and the number of samples used by a query. See
  [query/cost](checks/query/cost.md) docs for details.

## v0.44.2

### Fixed

- Fixed a crash in `promql/series` check when Prometheus instance becomes
  unavailable - #682.
- Fixed false positive reports in `alerts/template` check - #681.

## v0.44.1

### Fixed

- Rule names were not checked for correctly, allowing for rules with empty names
  to pass checks.
- Fixed GitHub annotations being added to unmodified lines - #645.

## v0.44.0

### Added

- Added `exclude` option to `ci` config block - #609.
- Added `minCount` & `severity` to `alerts/count` check - #612.
  This allows to only show estimated alerts count only if there would be high
  enough (`>= minCount`) number of alerts.
  Setting `severity` as well allows to block rules that would create too many alerts.
- GitHub reporter will now included folded list of all problems in the summary
  comment - #608.

### Fixed

- Fixed `alerts/annotation` check regexp matching - #613.
- When running `pint ci` using GitHub integration annotation comments are now
  reported only on modified lines - #640.

## v0.43.1

### Fixed

- Fixed `--base-branch` flag handling when branch name contains `/`.

## v0.43.0

### Added

- Added `--fail-on` flag to `pint lint` command - #570.
- Added `tls` section to `prometheus` configuration block - #540.

### Changed

- If a query run by pint fails because it was too expensive too run it will
  now be reported as a warning instead of an error.

### Fixed

- When validating queries using `{__name__=~"...", foo="bar"}` selectors pint could end up
  running queries matching a single label, like `count({foo="bar"})`, which could return too many
  results. This version ensures that queries always include name matcher to avoid that.

## v0.42.2

### Fixed

- `alerts/template` check didn't correctly handle `label_replace()` calls
  in queries - #568.

## v0.42.1

### Fixed

- Fixed `--base-branch` flag handling.
  Value of this flag wasn't being used correctly - #559.

## v0.42.0

### Fixed

- Fixed incorrect results in `promql/series` check for time series with only a single
  data point.

## v0.41.1

### Fixed

- Fixed parsing of alert `for` field values with long durations (`for: 1d`).

## v0.41.0

### Added

- Added `--fail-on` flag to `pint ci` command - #525.
- [promql/rate](checks/promql/rate.md) will now look for queries that call
  `rate()` on results of `sum(counter)` via recording rules.
  Example:

  ```yaml
  - record: my:sum
    expr: sum(http_requests_total)

  - alert: my alert
    expr: rate(my:sum[5m])
  ```

- Added [rule/for](checks/rule/for.md) check.
- Added `owners` configuration block for setting the list of allowed rule owner values.
  See [configuration](configuration.md) for details.

### Changed

- `pint lint` output will now include severity level as a text label - #524.

## v0.40.1

### Fixed

- Fixed a bug in `pint ci` that would cause a failure if a directory was renamed.
- Fixed false positive reports from `promql/series` check when a time series
  disappears from Prometheus.
- Fixed Prometheus flags parsing in `promql/range_query` check.

## v0.40.0

### Added

- Allow snoozing checks for entire file using `# pint file/snooze ...` comments.
- Added `lookbackRange` and `lookbackStep` configuration option to the
  [promql/series](checks/promql/series.md) check - #493.

### Changed

- Reverted GitHub integration to use [Pull Request Review](https://docs.github.com/en/rest/pulls/reviews)
  API - #490.

## v0.39.0

### Changed

- GitHub integration now uses [Check Runs](https://docs.github.com/en/rest/checks/runs) API - #478.

## v0.38.1

### Fixed

- `# pint file/disable` comments didn't properly handle Prometheus tags, this is fixed now.

## v0.38.0

### Added

- `prometheus` configuration blocks now accepts `tags` field with a list of tags.
  Tags can be used to disable or snooze specific checks on all Prometheus instances
  with that tag.
  See [ignoring](ignoring.md) for details.

## v0.37.0

### Added

- Added `pint_rule_file_owner` metric.

## v0.36.0

### Added

- Added ability to expand environment variables in pint configuration file.
  See [configuration](configuration.md) for details.

## v0.35.0

### Added

- Use [uber-go/automaxprocs](https://github.com/uber-go/automaxprocs) to
  automatically set GOMAXPROCS to match Linux container CPU quota.
- Added [labels/conflict](checks/labels/conflict.md) check.
- If you want to disable individual checks just for some time then you can now
  snooze them instead of disabling forever.

  The difference between `# pint disable ...` and `# pint snooze ...` comments is that
  the snooze comment must include a timestamp. Selected check will be disabled _until_
  that timestamp.
  Timestamp must either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339) syntax
  or `YYYY-MM-DD` (if you don't care about time and want to snooze until given date).
  Examples:

  ```yaml
  # pint snooze 2023-01-12T10:00:00Z promql/series
  # pint snooze 2023-01-12 promql/rate
  - record: ...
    expr: ...
  ```

### Changed

- Removed `cache` option from `prometheus` config blocks. Query cache will now auto-size itself
  as needed.

  If you have a config entry with `cache` option, example:

  ```javascript
  prometheus "prod" {
    uri   = "https://prometheus.example.com"
    cache = 20000
  }
  ```

  Then pint will fail to start. To fix this simply remove the `cache` option:

  ```javascript
  prometheus "prod" {
    uri = "https://prometheus.example.com"
  }
  ```

## v0.34.0

### Added

- Added [rule/duplicate](checks/rule/duplicate.md) check.

## v0.33.1

### Fixed

- Fixed a regression causing poor query cache hit rate.

## v0.33.0

### Added

- Added `uptime` field in `prometheus` configuration block.
  This field can be used to set a custom metric used for Prometheus uptime checks
  and by default uses `up` metric.
  If you have a Prometheus with a large number of scrape targets there might
  be a huge number of `up` time series making those uptime checks slow to run.
  If your Prometheus is configured to scrape itself, then you most likely want to use
  one of metrics exported by Prometheus, like `prometheus_build_info`:

  ```javascript
  prometheus "prod" {
    uri    = "https://prometheus.example.com"
    uptime = "prometheus_build_info"
  }
  ```

### Changed

- Refactored some queries used by [promql/series](checks/promql/series.md) check to
  avoid sending queries that might be very slow and/or return a huge amount of data.
- Prometheus query cache now takes into account the size of cached response.
  This makes memory usage needed for query cache more predictable.
  As a result the `cache` option for `prometheus` config block now means
  `the number of time series cached` instead of `the number of responses cached`
  and the default for this option is now `50000`.

## v0.32.1

### Fixed

- [promql/vector_matching](checks/promql/vector_matching.md) was sending expensive
  queries resulting in high memory usage, this is now fixed.

## v0.32.0

### Added

- Added `pint_prometheus_cache_evictions_total` metric tracking the number of times
  cache results were evicted from query cache.
- Allow disabling individual checks for the entire file using
  `# pint file/disable ...` comments.

### Changed

- Refactored query cache to only store queries that are requested more than once.
  This will avoid storing big responses that are never requested from the cache.

### Fixed

- Config validation will now check for duplicated `prometheus` block names.

## v0.31.1

### Fixed

- Fixed performance regression slowing down `pint watch` over time.

## v0.31.0

### Added

- `prometheus` configuration block now accepts optional `headers` field, for setting
  request headers that will be attached to any request made to given Prometheus server.
  Example:

  ```javascript
  prometheus "protected" {
  uri      = "https://prod.example.com"
  headers  = {
    "X-Auth": "secret",
    "X-User": "bob"
  }
  ```

### Changed

- Prometheus range query handling was rewritten to improve memory usage
  caused by queries returning huge number of results.
  As a result pint should use up to 5x less memory.

### Fixed

- Fixed false positive reports in [promql/vector_matching](checks/promql/vector_matching.md)
  for rules using `on(...)`. Example:

  ```yaml
  sum(foo) without(instance) * on(app_name) group_left() bar
  ```

- Don't log passwords when Prometheus URI is using basic authentication.
- Fixed false positive reports in [alerts/template](checks/alerts/template.md)
  suggesting to use `humanize` on queries that already use `round()`.
- Fixed false positive reports in [alerts/comparison](checks/alerts/comparison.md)
  when `bool` modifier is used on a condition that is guarded by another condition.
  Example:

  ```yaml
  alert: Foo
  expr: (foo > 1) > bool 1
  ```

- Fixed false positive reports in [alerts/template](checks/alerts/template.md)
  warning about labels removed in a query despite being re-added by a join.

## v0.30.2

### Fixed

- Fixed incorrect line number reporting on BitBucket annotations.

## v0.30.1

### Fixed

- Fixed handling of symlinks when running `pint lint` and `pint watch` commands.

## v0.30.0

### Added

- BitBucket only allows for annotations on modified lines, so when a high severity problem
  is reported on unmodified line pint will move that annotation to the first modified line,
  so it's still visible in BitBucket.
  Now pint will also add a note to that annotation to make it clear that the problem is really
  on a different line.
- [alerts/template](checks/alerts/template.md) will now run extra checks to validate syntax
  of queries executed from within alerting rule templates.

  Example template using `sum(xxx` query that's missing closing `)`:

  {% raw %}

  ```yaml
  - alert: ...
    expr: ...
    annotations:
      summary: |
        {{ with query "sum(xxx" }}
        {{ . | first | value | humanize }}
        {{ end }}
  ```

  {% endraw %}

- If a file is ignored pint will now note that using `Information` level annotation.
  This will make it more obvious that a CI check passed because pint didn't run any
  checks due to file being excluded.

### Changed

- Prometheus rule files can be symlinked between directories.
  If the symlink source and target files are in a different directory they can
  end up querying different Prometheus server when running ping checks.
  This means that when modifying symlink target file checks must be executed against
  both symlink source and target.
  Until now pint was ignoring symlinks but starting with this release it will try to
  follow them. This means that if you modify a file that has symlinks pointing to them
  pint will try run checks against those symlinks too.

  **NOTE**: pint can only detect and check symlinks if they are located in the current
  working directory (as seen by running pint process) or its sub-directories.

### Fixed

- Fixed a regression in [promql/vector_matching](checks/promql/vector_matching.md) that
  would cause a panic when parsing function calls with optional arguments.

## v0.29.4

### Fixed

- [promql/vector_matching](checks/promql/vector_matching.md) was incorrectly handling
  queries containing function calls with multiple arguments.

## v0.29.3

### Fixed

- Revert 'Use smaller buffers when decoding Prometheus API responses' change.

## v0.29.2

### Fixed

- Use smaller buffers when decoding Prometheus API responses.

## v0.29.1

### Fixed

- Fixed wrong request formatting for Prometheus metric metadata queries.

## v0.29.0

### Changed

- Switched from using [prometheus/client_golang](https://github.com/prometheus/client_golang)
  API client to streaming JSON library [prymitive/current](https://github.com/prymitive/current)

### Fixed

- Avoid reporting same issue multiple times in `promql/rate` and `promql/regexp` checks.

## v0.28.7

### Changed

- Updated Prometheus modules to [v2.38.0](https://github.com/prometheus/prometheus/releases/tag/v2.38.0).
  This adds support for `toTime` template function.

## v0.28.6

### Fixed

- Fixed symlink handling when running `pint lint`.

## v0.28.5

### Fixed

- Remove noisy debug logs.

## v0.28.4

### Added

- Added `pint_prometheus_cache_miss_total` metric.

### Changed

- Reduce log level for `File parsed` messages.

### Fixed

- Purge expired cache entries faster to reduce memory usage.

## v0.28.3

### Fixed

- Fix `absent()` handling in [alerts/comparison](checks/alerts/comparison.md) #330.

## v0.28.2

### Added

- Added `--min-severity` flag to the `pint lint` command. Default value is set to `warning`.

### Fixed

- Fix a regression in [promql/vector_matching](checks/promql/vector_matching.md) introduced
  in previous release.
- Fix [promql/series](checks/promql/series.md) disable comments not working when there
  are multiple comments on a rule.
- [promql/series](checks/promql/series.md) no longer emits an information message
  `metric is generated by alerts ...`.

## v0.28.1

### Fixed

- Don't use `topk` in [promql/vector_matching](checks/promql/vector_matching.md) check to
  avoid false positives.

## v0.28.0

### Added

- [promql/rate](checks/promql/rate.md) check will now also validate `deriv` function usage.
- [alerts/annotation](checks/alerts/annotation.md) check will now recommend using one of
  humanize functions if alert query is returning results based on `rate()` and the value
  is used in annotations.

### Changed

- [promql/series](checks/promql/series.md) check now supports more flexible
  `# pint disable promql/series(...)` comments.
  Adding a comment `# pint disable promql/series({cluster="dev"})` will disable this check
  for any metric selector with `cluster="dev"` matcher.
- [query/cost](checks/query/cost.md) check will now calculate how much Prometheus memory
  will be needed for storing results of given query.
  `bytesPerSample` option that was previously used to calculate this was removed.
- `prometheus {}` config block now allows to pass a list of paths to explicitly ignore
  by setting `exclude` option. Existing `paths` option was renamed to `include` for
  consistency. Example migration:

  ```javascript
  prometheus "foo" {
    [...]
    paths = [ "rules/.*" ]
  }
  ```

  becomes

  ```javascript
  prometheus "foo" {
    [...]
    include = [ "rules/.*" ]
  }
  ```

### Fixed

- `pint_last_run_checks` and `pint_last_run_checks_done` were not updated properly.

## v0.27.0

### Added

- Deduplicate reports where possible to avoid showing same issue twice.
- [rule/link](checks/rule/link.md) check for validating URIs found in alerting rule annotations.

### Changed

- Add more details to BitBucket CI reports.
- More compact console output when running `pint lint`.

## v0.26.0

### Added

- [promql/range_query](checks/promql/range_query.md) check.

### Fixed

- Strict parsing mode shouldn't fail on template errors, those will be later
  reported by `alerts/template` check.

## v0.25.0

### Changed

- All timeout options are now optional. This includes following config blocks:
  - `prometheus { timeout = ... }`
  - `repository { bitbucket { timeout = ... } }`
  - `repository { github { timeout = ... } }`
- `pint` will now try to discover all repository settings from environment variables
  when run as part of GitHub Actions workflow and so it doesn't need any
  `repository { github { ... } }` configuration block for that anymore.
  Setting `GITHUB_AUTH_TOKEN` is the only requirement for GitHub Actions now.

## v0.24.1

### Fixed

- Fixed line reporting on some strict parser errors.

### Added

- Added `--base-branch` flag to `pint ci` command.

## v0.24.0

### Added

- Added rate limit for Prometheus API requests with a default value of 100
  requests per second. To customise it set `rateLimit` field inside selected
  `prometheus` server definition.
- Added `pint_last_run_checks` and `pint_last_run_checks_done` metrics to track
  progress when running `pint watch`.

## v0.23.0

### Fixed

- Improved range query cache efficiency.

### Added

- Added extra global configuration for `promql/series` check.
  See check [documentation](checks/promql/series.md) for details.
- `prometheus` server definition in `pint` config file can now accept optional
  `cache` field (defaults to 10000) to allow fine tuning of built-in Prometheus
  API query caching.
- Added `pint_prometheus_cache_size` metric that exposes the number of entries
  currently in the query cache.

## v0.22.2

### Fixed

- Improved error reporting when strict mode is enabled.

## v0.22.1

### Fixed

- Fixed high memory usage when running range queries against Prometheus servers.

## v0.22.0

### Changed

- The way `pint` sends API requests to Prometheus was changed to improve performance.

  First change is that each `prometheus` server definition in `pint` config file can
  now accept optional `concurrency` field (defaults to 16) that sets a limit on how
  many concurrent requests can that server receive. There is a new metric that
  tracks how many queries are currently being run for each Prometheus server -
  `pint_prometheus_queries_running`.

  Second change is that range queries will now be split into smaller queries, so
  if `pint` needs to run a range query on one week of metrics, then it will break
  this down into multiple queries each for a two hour slot, and then merge all
  the results. Previously it would try to run a single query for a whole week
  and if that failed it would reduce time range until a query would succeed.

### Fixed

- Strict parsing mode didn't fully validate rule group files, this is now fixed
  and pint runs the same set of checks as Prometheus.
- Fixed `promql/series` handling of rules with `{__name__=~"foo|bar"}` queries.
- If Prometheus was stopped or restarted `promql/series` would occasionally
  report metrics as "sometimes present". This check will now try to find time
  ranges with no metrics in Prometheus and ignore these when checking if
  metrics are present.

## v0.21.1

### Fixed

- `pint_prometheus_queries_total` and `pint_prometheus_cache_hits_total` metric wasn't
  always correctly updated.
- Ignore `unknown` metric types in `promql/rate`.

## v0.21.0

### Added

- `promql/rate` check will now report if `rate()` or `irate()` function is being
  passed a non-counter metric.

## v0.20.0

### Fixed

- pint will now correctly handle YAML anchors.

## v0.19.0

### Added

- Parsing files in relaxed mode will now try to find rules inside multi-line strings #252.
  This allows direct linting of k8s manifests like the one below:

  ```yaml
  ---
  kind: ConfigMap
  apiVersion: v1
  metadata:
    name: example-app-alerts
    labels:
    app: example-app
  data:
    alerts: |
      groups:
        - name: example-app-alerts
          rules:
            - alert: Example_Is_Down
              expr: kube_deployment_status_replicas_available{namespace="example-app"} < 1
              for: 5m
              labels:
                priority: "2"
                environment: production
              annotations:
                summary: "No replicas for Example have been running for 5 minutes"
  ```

## v0.18.1

### Fixed

- Fixed incorrect line reported when pint fails to unmarshal YAML file.

## v0.18.0

### Added

- Allow fine tuning `promql/series` check with extra control comments
  `# pint rule/set promql/series min-age ...` and
  `# pint rule/set promql/series ignore/label-value ...`
  See [promql/series](checks/promql/series.md) for details.
- `promql/regexp` will report redundant use of regex anchors.

### Changed

- `promql/series` will now report missing metrics only if they were last seen
  over 2 hours ago by default. This can be customised per rule with comments.

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
  To enable old behaviour add `parser { relaxed = ["(.+)", ...]}` option in
  the config file. See [Configuration](configuration.md) for details.
  To enable old (relaxed) behaviour for all files add:

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

- Don't use fail-over Prometheus servers in case of errors caused by the query
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

- `prometheus` config block now allows to specify fail-over URIs using `failover` field.
  If fail-over URIs are set and main URI fails to respond pint will attempt to use them
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

- Added `pint_last_run_time_seconds` and `pint_rules_parsed_total` metrics when running `pint watch`.

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
- Added `pint_prometheus_queries_total` and `pint_prometheus_query_errors_total`
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

- Cache each Prometheus server responses to minimise the number of API calls.
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
  for all defined `prometheus {}` blocks unless there was at least one `rule {}` block.
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
  - `# pint disable promql/by` becomes `# pint disable promql/aggregate`
  - `# pint disable promql/without` becomes `# pint disable promql/aggregate`
  - `# pint ignore promql/by` becomes `# pint ignore promql/aggregate`
  - `# pint ignore promql/without` becomes `# pint ignore promql/aggregate`

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
      summary: "foo metric is missing for job {{ $labels.job }}"
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
      summary: "foo is down on {{ $labels.instance }}"
  ```

  {% endraw %}

  Would generate a warning since `instance` label is being stripped by `without(instance)`.

## v0.1.5

### Fixed

- Fixed file descriptor leak due to missing file `Close()` #69.

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
