# Configuration syntax

**NOTE** all regex pattern are anchored.

## CI

Configure continuous integration environments.

Syntax:

```JS
ci {
  include    = [ "(.*)", ... ]
  maxCommits = 20
  baseBranch = "master"
}
```

- `include` - list of file patters to checks when running checks. Only files
  matching those regex rules will be checked, other modified files will be ignored.
- `maxCommits` - by default pint will try to find all commits on current branch,
  this requires full git history to be present, if we have a shallow clone this
  might fail to find only current branch commits and give us a huge list.
  If the number of commits returned by branch discovery is more than `maxCommits`
  then pint will fail to run.
- `baseBranch` - base branch to compare `HEAD` commit with when calculating the list
  of commits to check.

## Repository

Configure supported code hosting repository, used for reporting PR checks from CI
back to the repository, to be displayed in the PR UI.
Currently only supports [BitBucket](https://bitbucket.org/) and [GitHub](https://github.com/).

**NOTE**: BitBucket integration requires `BITBUCKET_AUTH_TOKEN` environment variable
to be set. It should contain a personal access token used to authenticate with the API.

**NOTE**: GitHub integration requires `GITHUB_AUTH_TOKEN` environment variable
to be set to a personal access key that can access your repository. Also, `GITHUB_PULL_REQUEST_NUMBER`
environment variable needs to point to the pull request number which will be used whilst
submitting comments.

Syntax:

```JS
repository {
  bitbucket {
    uri        = "https://..."
    timeout    = "30s"
    project    = "..."
    repository = "..."
  }
}
```

- `bitbucket:uri` - base URI of this repository, will be used for HTTP
  requests to the BitBucket API.
- `bitbucket:timeout` - timeout to be used for API requests.
- `bitbucket:project` - name of the BitBucket project for this repository.
- `bitbucket:repository` - name of the BitBucket repository.

```JS
repository {
  github {
    uri        = "https://..."
    timeout    = "30s"
    owner      = "..."
    repo       = "..."
  }
}
```

- `github:baseuri` - base URI of GitHub or GitHub enterprise, will be used for HTTP requests to the GitHub API.
- `github:uploaduri` - upload URI of GitHub or GitHub enterprise, will be used for HTTP requests to the GitHub API.

If `github:baseuri` _or_ `github:uploaduri` are not specified then [GitHub](https://github.com) will be used.

- `github:timeout` - timeout to be used for API requests;
- `github:owner` - name of the GitHub owner i.e. the first part that comes before the repository's name in the URI;
- `github:repo` - name of the GitHub repository (e.g. `monitoring`).

## Prometheus servers

Some checks work by querying a running Prometheus instance to verify if
metrics used in rules are present. If you want to use those checks then you
first need to define one or more Prometheus servers.

Syntax:

```JS
prometheus "$name" {
  uri     = "https://..."
  timeout = "60s"
  paths   = ["...", ...]
}
```

- `$name` - each defined server should have a unique name that can be used in check
  definitions.
- `uri` - base URI of this Prometheus server, used for API requests and queries.
- `timeout` - timeout to be used for API requests.
- `paths` - optional path filter, if specified only paths matching one of listed regex
  patterns will use this Prometheus server for checks.

Example:

```JS
prometheus "prod" {
  uri     = "https://prometheus-prod.example.com"
  timeout = "60s"
}

prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  paths   = [ "alerts/test/.*" ]
}
```

## Matching rules to checks

Most checks, except basic syntax verification, requires some configuration to decide
which checks to run against which files and rules.

Syntax:

```JS
rule {
  match {
    path = "..."
    kind = "alerting|recording"
    annotation "(.*)" {
      value = "(.*)"
    }
    label "(.*)" {
      value = "(.*)"
    }
  }

  [ check definition ]
  ...
  [ check definition ]
}
```

- `match:path` - only files matching this pattern will be checked by this rule
- `match:kind` - optional rule type filter, only rule of this type will be checked
- `match:annotation` - optional annotation filter, only alert rules with at least one
  annotation matching this pattern will be checked by this rule.
- `match:label` - optional annotation filter, only rules with at least one label
   matching this pattern will be checked by this rule. For recording rules only static
   labels set on the recording rule are considered.

Example:

```JS
rule {
  match {
    path = "rules/.*"
    kind = "alerting"
    label "severity" {
      value = "(warning|critical)"
    }
    [ check applied only to severity="critical" and severity="warning" alerts ]
  }
}
```

# Check definitions

## Aggregation

This check is used to inspect promql expressions and ensure that specific labels
are kept or stripped away when aggregating results. It's mostly useful in recording
rules.

Syntax:

```JS
aggregate "(.*)" {
  severity = "bug|warning|info"
  keep = [ "...", ... ]
  strip = [ "...", ... ]
}
```

- `severity` - set custom severity for reported issues, defaults to a warning
- `keep` - list of label names that must be preserved
- `strip` - list of label names that must be stripped

Examples:

Ensure that all series generated from recording rules have `job` labels preserved:

```JS
rule {
  match {
    kind = "recording"
  }
  aggregate ".+" {
    keep = ["job"]
  }
}
```

In some cases you might want to ensure that specific labels are removed in aggregations.
For example in recording rules that are producing series consumed by federation, where
only aggregated results (not per instance) are allowed:

```JS
rule {
  match {
    kind = "recording"
  }
  aggregate "cluster:.+" {
    strip = ["instance"]
  }
}
```

By default all issues found by this check will be reported as warnings. To adjust
severity set a custom `severity` key:

```JS
aggregate ".+" {
  ...
  severity = "bug"
}
```

## Annotations

This check is used to ensure that all required annotations are set on alerts and that
they have correct values.

Syntax:

```JS
annotation "(.*)" {
  severity = "bug|warning|info"
  value    = "(.*)"
  required = true|false
}
```

- `severity` - set custom severity for reported issues, defaults to a warning
- `value` - optional value pattern to enforce
- `required` - if `true` pint will require every alert to have this annotation set,
  if `false` it will only check values where annotation is set

Examples:

This set of rules will:
- require `summary` annotation to be present, if missing it will be reported as a warning
- if a `dashboard` annotation is provided it must match `https://grafana\.example\.com/.+`
  pattern, if it doesn't match that pattern it will be reported as a bug

```JS
rule {
  match {
    kind = "alerting"
  }

  annotation "summary" {
    required = true
  }

  annotation "dashboard" {
    severity = "bug"
    value    = "https://grafana\.example\.com/.+"
  }
}
```

## Labels

This check works the same way as `annotation` check, but it operates on
labels instead.
It uses static labels set on alerting or recording rule. It doesn't use
labels on time series used in those rules.

Syntax:

```JS
label "(.*)" {
  severity = "bug|warning|info"
  value    = "..."
  required = true|false
}
```

Example:

Require `severity` label to be set on alert rules with two all possible values:

```JS
rule {
  match {
    kind = "alerting"
  }

  label "severity" {
    value    = "(warning|critical)"
    required = true
  }
}
```

## Rate

This check inspects `rate()` and `irate()` functions and warns if used duration
is too low. It does so by first getting global `scrape_interval` value for selected
Prometheus servers and comparing duration to it.
Reported issue depends on a few factors:

For `rate()` function:
- If duration is less than 2x `scrape_interval` it will report a bug.
- If duration is between 2x and 4x `scrape_interval` it will report a warning.

For `irate()` function:
- If duration is less than 2x `scrape_interval` it will report a bug.
- If duration is between 2x and 3x `scrape_interval` it will report a warning.

Syntax:

```JS
rate {
  prometheus = ["...", ...]
}
```

Example:

```JS
prometheus "prod" {
  uri     = "https://prometheus-prod.example.com"
  timeout = "60s"
}

prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
}

rule {
  match {
    kind = "recording"
  }

  rate {
    prometheus = ["prod", "dev"]
  }
}
```

## Alerts

This check is used to estimate how many times given alert would fire.
It will run `expr` query from every alert rule against selected Prometheus
servers and report how many unique alerts it would generate.
If `for` is set on alerts it will be used to adjust results.

Syntax:

```JS
alerts {
  range      = "1h"
  step       = "1m"
  resolve    = "5m"
  prometheus = [ "...", ... ]
}
```

- `range` - query range, how far to look back, `1h` would mean that pint will
  query last 1h of metrics. If a query results in a timeout pint will retry it
  with 50% smaller range until it succeeds.
  Defaults to `1d`.
- `step` - query resolution, for most accurate result use step equal
  to `scrape_interval`, try to reduce it if that would load too many samples.
  Defaults to `1m`.
- `resolve` - duration after which stale alerts are resolved. Defaults to `5m`.
- `prometheus` - list of Prometheus servers to query. All servers must be first
  defined as `prometheus` blocks in global pint config.

Example:

```JS
prometheus "prod" {
  uri     = "https://prometheus-prod.example.com"
  timeout = "60s"
}

rule {
  match {
    kind = "recording"
  }
  alerts {
    range      = "1d"
    step       = "1m"
    resolve    = "5m"
    prometheus = [ "prod" ]
  }
}
```

## Comparison

This check enforces use of a comparison operator in alert queries.
Since any query result triggers an alert usual query would be something
like `error_count > 10`, so we only get `error_count` series if the value
is above 10. If we would remove `> 10` part query would always return `error_count`
and so it would always trigger an alert.

Syntax:

```JS
comparison {
  severity = "bug|warning|info"
}
```

- `severity` - set custom severity for reported issues, defaults to a bug.

Example:

```
rule {
  match {
    kind = "alerting"
  }
  comparison {}
}
```


## Cost

This check is used to calculate cost of a query and optionally report an issue
if that cost is too high. It will run `expr` query from every rule against
selected Prometheus servers and report results.
This check can be used for both recording and alerting rules, but is most
useful for recording rules.

Syntax:

```JS
cost {
  severity       = "bug|warning|info"
  bytesPerSample = 1024
  maxSeries      = 5000
  prometheus     = ["...", ...]
}
```

- `severity` - set custom severity for reported issues, defaults to a warning.
  This is only used when query result series exceed `maxSeries` value (if set).
  If `maxSeries` is not set or when results count is below it pint will still
  report it as information.
- `bytesPerSample` - if set results will use this to calculate estimated memory
  required to store returned series in Prometheus.
- `maxSeries` - if set and number of results for given query exceeds this value
  it will be reported as a bug (or custom severity if `severity` is set).
- `prometheus` - list of Prometheus servers to query. All servers must be first
  defined as `prometheus` blocks in global pint config.

Examples:

All rules from files matching `rules/dev/.+` pattern will be tested against
`dev` server. Results will be reported as information regardless of results.

```JS
prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  paths   = ["rules/dev/.+"]
}

rule {
  cost {
    prometheus     = ["dev"]
  }
}
```

To add memory usage estimate we first need to get average bytes per sample.
This can be be estimated using two different queries:

- for RSS usage: `process_resident_memory_bytes / prometheus_tsdb_head_series`
- for Go allocations: `go_memstats_alloc_bytes / prometheus_tsdb_head_series`

Since Go uses garbage collector RSS memory will be more than the sum of all
memory allocations. RSS usage will be "worst case" while "Go alloc" best case,
while real memory usage will be somewhere in between, depending on many factors
like memory pressure, Go version, GOGC settings etc.

```JS
...
  cost {
    bytesPerSample = 4096
    prometheus     = ["dev"]
  }
}
```

## Series

This check will also query Prometheus servers, it is used to warn about queries
that are using metrics not currently present in Prometheus.
It parses `expr` query from every rule, finds individual metric selectors and
checks if they return any values.

Let's say we have a rule this query: `sum(my_metric{foo="bar"}) > 10`.
This checks would query all configured server for the existence of
`my_metric{foo="bar"}` series and report a warning if it's missing.

Syntax:

```JS
series {
  severity       = "bug|warning|info"
  prometheus     = ["...", ...]
}
```

- `severity` - set custom severity for reported issues, defaults to a warning.
- `prometheus` - list of Prometheus servers to query. All servers must be first
  defined as `prometheus` blocks in global pint config.

Example:

```JS
prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
}

prometheus "prod" {
  uri     = "https://prometheus-prod.example.com"
  timeout = "30s"
}

rule {
  match {
    kind = "recording"
  }

  cost {
    prometheus = ["dev", "prod"]
  }
}
```

## Values

This check will inspect all alert rules and warn if any of them
uses query return values inside alert labels.
Two alerts are identical if they have identical labels, so using
query value will generate a new unique alert every time it changes.
If alerting rule is using `for` it might prevent it from ever firing
if the value keeps changing before `for` is satisfied, because
Prometheus will consider it to be a new alert and start `for` tracking
from zero.

If you want to include query value in the alert then use annotations
for that. Annotations are not used to compare alerts identity and so
the value of any annotation can change between alert evaluations.

See [this blog post](https://www.robustperception.io/dont-put-the-value-in-alert-labels)
for more details.

Syntax:

```JS
series {
  severity       = "bug|warning|info"
}
```

- `severity` - set custom severity for reported issues, defaults to a bug.

Example:

```JS
rule {
  match {
    kind = "alerting"
  }

  value {
    severity = "fatal"
  }
}
```

## Reject

This check allows rejecting label or annotations keys and values
using regexp rules.

Syntax:

```JS
reject "(.*)" {
  severity          = "bug|warning|info"
  label_keys        = true|false
  label_values      = true|false
  annotation_keys   = true|false
  annotation_values = true|false
}
```

- `severity` - set custom severity for reported issues, defaults to a bug.
- `label_keys` - if true label keys for recording and alerting rules will
  be checked.
- `label_values` - if true label values for recording and alerting rules will
  be checked.
- `annotation_keys` - if true annotation keys for alerting rules will be checked.
- `annotation_values` - if true label values for alerting rules will be checked.

Example:

Disallow using URLs as label keys or values:

```JS
rule {
  match {
    kind = "alerting"
  }

  reject "https?://.+" {
    label_keys = true
    label_values = true
  }
}
```

Disallow spaces in label and annotation keys:

```JS
rule {
  reject ".* +.*" {
    annotation_keys = true
    label_keys = true
  }
}
```

## Template

This check validates templating used in annotations and labels for alerting rules.
See [Prometheus docs](https://prometheus.io/docs/prometheus/latest/configuration/template_reference/)
for details of supported templating syntax.

Syntax:

```JS
template {
  severity          = "bug|warning|info"
}
```

- `severity` - set custom severity for reported issues, defaults to a bug.

Example:

```JS
rule {
  match {
    kind = "alerting"
  }

  template {
    severity = "fatal"
  }
}

# Ignoring selected lines or files

While parsing files pint will look for special comment blocks and use them to
exclude some parts all whole files from checks.

## Ignoring whole files

Add a `# pint ignore/file` comment on top of the file, everything below that line
will be ignored.

Example:

```YAML
# pint ignore/file

groups:
  - name: example
    rules:
    - record: job:http_inprogress_requests:sum
      expr: sum by (job) (http_inprogress_requests)
```

## Ignoring individual lines

To ignore just one line use `# pint ignore/line` at the end of that line or
`# ignore/next-line` on the line before.
This is useful if you're linting templates used to generate Prometheus
configuration and it contains some extra lines that are not valid YAML.

Example:

```YAML
{% set some_jinja_var1 = "bar" } # pint ignore/line
groups:
  - name: example
    rules:
    - record: job:http_inprogress_requests:sum
      expr: sum by (job) (http_inprogress_requests)

# pint ignore/next-line
{% set some_jinja_var2 = "foo" }
```

## Ignoring a range of lines

To ignore a part of a file wrap it with `# pint ignore/begin` and
`# pint ignore/end` comments.

Example:

```YAML
# pint ignore/begin
{% set some_jinja_var1 = "bar" }
{% set some_jinja_var2 = "foo" }
# pint ignore/end

groups:
  - name: example
    rules:
    - record: job:http_inprogress_requests:sum
      expr: sum by (job) (http_inprogress_requests)
```

## Disabling individual checks for specific rules

To disable individual check for a specific rule use `# pint disable ...` comments.
A single comment can only disable one check, so repeat it for every check you wish
to disable.

To disable `query/cost` check add `# pint disable query/cost` comment anywhere in
the rule.

Example:

```YAML
groups:
  - name: example
    rules:
    - record: instance:http_requests_total:avg_over_time:1w
      # pint disable query/cost
      expr: avg_over_time(http_requests_total[1w]) by (instance)
```

```YAML
groups:
  - name: example
    rules:
    - record: instance:http_requests_total:avg_over_time:1w
      expr: avg_over_time(http_requests_total[1w]) by (instance) # pint disable query/cost
```
