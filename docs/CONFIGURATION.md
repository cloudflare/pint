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
    baseuri    = "https://..."
    uploaduri  = "https://..."
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
    path = "(.+)"
    name = "(.+)"
    kind = "alerting|recording"
    command = "ci|lint|watch"
    annotation "(.*)" {
      value = "(.*)"
    }
    label "(.*)" {
      value = "(.*)"
    }
  }
  ignore {
    path = "(.+)"
    name = "(.+)"
    kind = "alerting|recording"
    command = "ci|lint|watch"
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
- `match:name` - only rules with names (`record` for recording rules and `alert` for alerting
  rules) matching this pattern will be checked rule
- `match:kind` - optional rule type filter, only rule of this type will be checked
- `match:command` - optional command type filter, this allows to include or ignore rules
  based on the command pint is run with `pint ci`, `pint lint` or `pint watch`.
- `match:annotation` - optional annotation filter, only alert rules with at least one
  annotation matching this pattern will be checked by this rule.
- `match:label` - optional annotation filter, only rules with at least one label
   matching this pattern will be checked by this rule. For recording rules only static
   labels set on the recording rule are considered.
- `ignore` - works exactly like `match` but does the opposite - any alerting or recording rule
  matching all conditions defined on `ignore` will not be checked by this `rule` block.

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

This check is enabled by default for all configured Prometheus servers.

Example:

```JS
prometheus "prod" {
  uri     = "https://prometheus-prod.example.com"
  timeout = "60s"
  paths = [
    "rules/prod/.*",
    "rules/common/.*",
  ]
}

prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  paths = [
    "rules/dev/.*",
    "rules/common/.*",
  ]
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
  }
}
```

## Comparison

This check enforces use of a comparison operator in alert queries.
Since any query result triggers an alert usual query would be something
like `error_count > 10`, so we only get `error_count` series if the value
is above 10. If we would remove `> 10` part query would always return `error_count`
and so it would always trigger an alert.

This check is enabled by default and doesn't require any configuration.

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
  cost {}
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

This check is enabled by default for all configured Prometheus servers.

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

This check will also inspect all alert rules and warn if any of them
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

This check is enabled by default and doesn't require any configuration.

## Vector Matching

This check will try to find queries that try to
[match vectors](https://prometheus.io/docs/prometheus/latest/querying/operators/#vector-matching)
but have different sets of labels on both side of the query.

Consider these two time series:

```
http_errors{job="node-exporter", cluster="prod", instance="server1"}
```

and

```
cluster:http_errors{job="node-exporter", cluster="prod"}
```

One of them tracks specific instance and one aggregates series for the whole cluster.
Because they have different set of labels if we want to calculate some value using both
of them, for example:

```
http_errors / cluster:http_errors
```

we wouldn't get any results. To fix that we need ignore extra labels:

```
http_errors / ignoring(instance) cluster:http_errors
```

This check aims to find all queries that using vector matching where both sides
of the query have different sets of labels causing no results to be returned.

This check is enabled by default for all configured Prometheus servers.

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
```

## Fragile

This check will try to find rules with queries that can be rewritten in a way
which makes them more resilient to label changes.

Example:

Let's assume we have these metrics:

```
errors{cluster="prod", instance="server1", job="node_exporter"} 5
requests{cluster="prod", instance="server1", job="node_exporter", rack="a"} 10
requests{cluster="prod", instance="server1", job="node_exporter", rack="b"} 30
requests{cluster="prod", instance="server1", job="node_exporter", rack="c"} 25
```

If we want to calculate the ratio of errors to requests we can use this query:

```
errors / sum(requests) without(rack)
```

`sum(requests) without(rack)` will produce this result:

```
requests{cluster="prod", instance="server1", job="node_exporter"} 65
```

Both sides of the query will have exact same set of labels:

```
{cluster="prod", instance="server1", job="node_exporter"}`
```

which is needed to be able to use a binary expression here, and so this query will
work correctly.

But the risk here is that if at any point we change labels on those metrics we might
end up with left and right hand sides having different set of labels.
Let's see what happens if we add an extra label to `requests` metric.

```
errors{cluster="prod", instance="server1", job="node_exporter"} 5
requests{cluster="prod", instance="server1", job="node_exporter", rack="a", path="/"} 3
requests{cluster="prod", instance="server1", job="node_exporter", rack="a", path="/users"} 7
requests{cluster="prod", instance="server1", job="node_exporter", rack="b", path="/"} 10
requests{cluster="prod", instance="server1", job="node_exporter", rack="b", path="/login"} 1
requests{cluster="prod", instance="server1", job="node_exporter", rack="b", path="/users"} 19
requests{cluster="prod", instance="server1", job="node_exporter", rack="c", path="/"} 25
```

Our left hand side (`errors` metric) still has the same set of labels:

```
{cluster="prod", instance="server1", job="node_exporter"}
```

But `sum(requests) without(rack)` will now return a different result:

```
requests{cluster="prod", instance="server1", job="node_exporter", path="/"} 38
requests{cluster="prod", instance="server1", job="node_exporter", path="/users"} 26
requests{cluster="prod", instance="server1", job="node_exporter", path="/login"} 1
```

We no longer get a single result because we only aggregate by removing `rack` label.
Newly added `path` label is not being aggregated away so it splits our results into
multiple series. Since our left hand side doesn't have any `path` label it won't
match any of the right hand side result and this query won't produce anything.

One solution here is to add `path` to `without()` to remove this label when aggregating,
but this requires updating all queries that use this metric every time labels change.

Another solution is to rewrite this query with `by()` instead of `without()` which
will ensure that extra labels will be aggregated away automatically:

```
errors / sum(requests) by(cluster, instance, job)
```

The list of labels we aggregate by doesn't have to match exactly with the list
of labels on the left hand side, we can use `on()` to instruct Prometheus
which labels should be used to match both sides.
For example if we would remove `job` label during aggregation we would once
again have different sets of labels on both side, but we can fix that
by adding labels we use in `by()` to `on()`:

```
errors / on(cluster, instance) sum(requests) by(cluster, instance)
```

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

{% raw %}
```YAML
{% set some_jinja_var1 = "bar" %} # pint ignore/line
groups:
  - name: example
    rules:
    - record: job:http_inprogress_requests:sum
      expr: sum by (job) (http_inprogress_requests)

# pint ignore/next-line
{% set some_jinja_var2 = "foo" %}
```
{% endraw %}

## Ignoring a range of lines

To ignore a part of a file wrap it with `# pint ignore/begin` and
`# pint ignore/end` comments.

Example:

{% raw %}
```YAML
# pint ignore/begin
{% set some_jinja_var1 = "bar" %}
{% set some_jinja_var2 = "foo" %}
# pint ignore/end

groups:
  - name: example
    rules:
    - record: job:http_inprogress_requests:sum
      expr: sum by (job) (http_inprogress_requests)
```
{% endraw %}

## Disabling checks globally

To disable given check globally, for all files and rules, add it to pint configuration
file. Syntax:

```JS
checks {
  disabled = [ "...", ... ]
}
```

Example:

```JS
checks {
  disabled = ["alerts/template"]
}
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

Some checks allow to specify extra parameters.

### query/series

You can disable `query/series` for specific metric using `# pint disable query/series(selector)`
comment.
Just like with PromQL if a selector doesn't have any labels then it will match all instances,
if you pass any labels it will only pass time series with those labels.

Disable warnings about missing `my_metric_name`:

```YAML
# pint disable query/series(my_metric_name)
```

Disable it only for `my_metric_name{cluster="dev"}` but still warn about `my_metric_name{cluster="prod"}`:

```YAML
# pint disable query/series(my_metric_name{cluster="dev"})
```
