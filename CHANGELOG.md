# Changelog

## [v0.1.3]

### Added

- `vector_matching` check for finding queries with incorrect `on()` or `ignoring()`
  keywords.

## [v0.1.2]

### Fixed

- `# pint skip/line` place between `# pint skip/begin` and `# pint skip/end` lines would
  reset ignore rules causing lines that should be ignored to be parsed. 

## [v0.1.1]

### Changed

- `value` check was replaced by `template`, which covers the same functionality and more.
  See [docs](/docs/CONFIGURATION.md#template) for details.
