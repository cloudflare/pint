# Changelog

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

  ```
  - alert: Foo
    expr: absent(foo{env="prod"})
    annotations:
      summary: 'foo metric is missing for job {{ $labels.job }}'
  ```

  Would generate a warning since `absent()` can only return labels that are explicitly
  passed to it and the above call only passes `env` label.
  This can be fixed by updating the query to `absent(foo{env="prod", job="bar"})`.

## v0.4.0

### Added

- `comparison` check will now warn when alert query uses
  [bool](https://prometheus.io/docs/prometheus/latest/querying/operators/#comparison-binary-operators)
  modifier after condition, which can cause alert to always fire.
  Example:

  ```
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

  ```
  - alert: Foo
    expr: count(up) without(instance) == 0
    annotations:
      summary: 'foo is down on {{ $labels.instance }}'
  ```

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
