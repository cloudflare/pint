# Changelog

## v0.2.0

### Added

- `--offline` flag for `pint lint` command. When passed only checks that don't send
  any live queries to Prometheus server will be run.
- `alerts/template` check will now warn if template if referencing a label that is being
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
