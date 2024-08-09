---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/series

This check will query Prometheus servers, it is used to warn about queries
that are using metrics not currently present in Prometheus.
It parses `expr` query from every rule, finds individual metric selectors and
runs a series of checks for each of them.

Let's say we have a rule this query: `sum(my_metric{foo="bar"}) > 10`.
This checks would first try to determine if `my_metric{foo="bar"}`
returns anything via instant query and if it doesn't it will try
to determine why, by checking if:

- `my_metric` metric was ever present in Prometheus
- `my_metric` was present but disappeared
- `my_metric` has any series with `foo` label
- `my_metric` has any series matching `foo="bar"`

Metrics that are wrapped in `... or vector(0)` won't be checked, since
the intention of adding `or vector(0)` is to provide a fallback value
when there are no matching time series.
Example:

```yaml
- alert: Foo
  expr: sum(my_metric or vector(0)) > 1
```

## Common problems

If you see this check complaining about some metric it's might due to a number
of different issues. Here are some usual cases.

### Your query is using ALERTS or ALERTS_FOR_STATE metrics

Prometheus itself exposes [metrics about active alerts](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/#inspecting-alerts-during-runtime).

And it's possible to use those metrics in recording or alerting rules.

If pint finds a query using either `ALERTS{alertname="..."}` or
`ALERTS_FOR_STATE{alertname="..."}` selector it will check if there's
alerting rule with matching name defined. For queries that don't pass any
`alertname` label filters it will skip any further checks.

### Your query is using recording rules

If a metric isn't present in Prometheus but pint finds a recording rule
with matching name then it will emit a warning and skip further checks.

Example with alert rule that depends on two recording rules:

```yaml
# count the number of targets per job
- record: job:up:count
  expr: count(up) by(job)

# total number of targets that are healthy per job
- record: job:up:sum
  expr: sum(up) by(job)

# alert if <50% of targets are down
- alert: Too Many Targets Are Down
  expr: (job:up:sum / job:up:count) < 0.5
```

If all three rules where added in a single PR and pint didn't try to match
metrics to recording rule then `pint ci` would block such PR because metrics
this alert is using are not present in Prometheus.

To avoid this pint will only emit a warning, to make it obvious that it was
unable to run a full set of checks, but won't report any problems.

For best results you should split your PR and first add all recording rules
before adding the alert that depends on it. Otherwise pint might miss some
problems like label mismatch.

### Your query cannot return anything

- You are trying to use a metric that is not present in Prometheus at all.
- Service exporting your metric is not working or no longer being scraped.
- You are querying wrong Prometheus server.
- You are trying to filter a metric that exists using a label key that is
  never present on that metric.
- You are using label value as a filter, but that value is never present.

If that's the case you need to fix you query. Make sure your metric is present
and it has all the labels you expect to see.

### Metrics you are using have unstable labelling scheme

Some time series for the same metric will have label `foo` and some won't.
Although there's nothing technically wrong with this and Prometheus allows
you to do so, this makes querying metrics difficult as results containing
label `foo` will be mixed with other results not having that label.

All queries would effectively need a `{foo!=""}` or `{foo=""}` filter to
select only one variant of this metric.

Best solution here is to fix labelling scheme.

### Metric labels are generated dynamically in response to some activity

Some label values will appear only temporarily, for example if metrics
are generated for serviced HTTP request and they include some details of
those requests that cannot be known ahead of time, like request path or
method.

When possible this can be addressed by initialising metrics with all known
label values to zero on startup:

```go
func main() {
  myMetric = prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
      Name: "http_requests_total",
      Help: "Total number of HTTP requests",
    },
    []string{"code"},
  )
  myMetric.WithLabelValues("2xx").Set(0)
  myMetric.WithLabelValues("3xx").Set(0)
  myMetric.WithLabelValues("4xx").Set(0)
  myMetric.WithLabelValues("5xx").Set(0)
}
```

If that's not doable you can let pint know that it's not possible to validate
those queries by disabling this check. See below for instructions on how to do
that.

## Configuration

This check supports setting extra configuration option to fine tune its behaviour.

Syntax:

```js
check "promql/series" {
  ignoreMetrics     = [ "(.*)", ... ]
  ignoreLabelsValue = { "...": [ "...", ... ] }
}
```

- `lookbackRange` - how far back to query when checking if given metric was ever
  present in Prometheus.
  Default is `7d`, meaning that if a metric is missing pint will query last 7 days
  of metrics to tell you if this metric was ever present and if so, when was it last
  seen.
- `lookbackStep` - look-back query resolution.
  Default is `5m` which matches Prometheus default
  [staleness](https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness)
  checks.
  If you have a custom `--query.lookback-delta` flag passed to Prometheus you might want
  to set this option to the same value.
- `ignoreMetrics` - list of regexp matchers, if a metric is missing from Prometheus
  but the name matches any of provided regexp matchers then pint will only report a
  warning, instead of a bug level report.
- `ignoreLabelsValue` - allows to configure a global list of label **names** for which pint
  should **NOT** report problems if there's a query that uses a **value** that does not exist.
  This can be also set per rule using `# pint rule/set promql/series ignore/label-value $labelName`
  comments, see below.
  The value of this option is a map where the key is a metric selector to match on and the value
  is the list of label names.

Example:

```js
check "promql/series" {
  lookbackRange = "5d"
  lookbackStep = "1m"
  ignoreMetrics = [
    ".*_error",
    ".*_error_.*",
    ".*_errors",
    ".*_errors_.*",
  ]
}
```

If you might have a query with `http_requests_total{code="401"}` selector but `http_requests_total`
will only have time series with `code="401"` label if there were requests that resulted in responses
with HTTP code `401`, which would result in a pint reports. This example would disable all such pint
reports for all Prometheus rules:

```js
check "promql/series" {
  ignoreLabelsValue = {
    "http_requests_total" = [ "code" ]
  }
}
```

You can use any metric selectors as keys in `ignoreLabelsValue` if you want apply it only
to metric selectors in queries that match the selector in `ignoreLabelsValue`.
For example if you have a rule that uses the same metric with two different selectors:

```yaml
- alerts: ...
  expr: |
    rate(http_requests_total{env="prod", code="401"}[5m]) > 0
    or
    rate(http_requests_total{env="dev", code="401"}[5m]) > 0
```

And you want to disable pint warnings only for the second selector (`http_requests_total{env="dev", code="401"}`)
but not the first one (`http_requests_total{env="prod", code="401"}`) you can do that by adding any label matcher
used in the query:

```js
check "promql/series" {
  ignoreLabelsValue = {
    "http_requests_total{env=\"dev\"}" = [ "code" ]
  }
}
```

You can only use label matchers that would match the selector from the query itself, not from the time series
the query would return. This whole logic applies only to the query, not to the results of it.

### min-age

But default this check will report a problem if a metric was present
in Prometheus but disappeared at least two hours ago.
You can change this duration per Prometheus rule by adding a comment around it.
Syntax:

To set `min-age` for all metrics in a query:

```yaml
# pint rule/set promql/series min-age $duration
```

Duration must follow syntax documented [here](https://prometheus.io/docs/prometheus/latest/querying/basics/#time-durations).

To set `min-age` for specific metric:

```yaml
# pint rule/set promql/series($selector)) min-age $duration
```

Example:

```yaml
- record: ...
  # Report problems if any metric in this query is missing for at least 3 days
  # pint rule/set promql/series min-age 3d
  expr: sum(foo) / sum(bar)

- record: ...
  # Report problems if:
  # - metric "foo" is missing for at least 1 hour (defaults)
  # - metric "bar{instance=xxx}" is missing for at least 4 hours
  # pint rule/set promql/series(bar{instance="xxx"}) min-age 4h
  expr: sum(foo) / sum(bar{instance="xxx"})
```

### ignore/label-value

By default pint will report a problem if a rule uses query with a label filter
and the value of that filter query doesn't match anything.

For example `rate(http_errors_total{code="500"}[2m])` will report a problem
if there are no `http_errors_total` series with `code="500"`.

The goal here is to catch typos in label filters or labels with values that
got renamed, but in some cases this will report false positive problems,
especially if label values are exported dynamically, for example after
HTTP status code is observed.

In the `http_errors_total{code="500"}` example if `code` label is generated
based on HTTP responses then there won't be any series with `code="500"` until
there's at least one HTTP response that generated this code.

You can relax pint checks so it doesn't validate if label values for specific
labels are present on any time series.

This can be also set globally for all rules using `ignoreLabelsValue` config option,
see above.

Syntax:

```yaml
# pint rule/set promql/series ignore/label-value $labelName
```

Example:

```yaml
- alert: ...
  # disable code label checks for all metrics used in this rule
  # pint rule/set promql/series ignore/label-value code
  expr: rate(http_errors_total{code="500"}[2m]) > 0.1

- alert: ...
  # disable code label checks for http_errors_total metric
  # pint rule/set promql/series(http_errors_total) ignore/label-value code
  expr: rate(http_errors_total{code="500"}[2m]) > 0.1

- alert: ...
  # disable code label checks only for http_errors_total{code="500"} queries
  # pint rule/set promql/series(http_errors_total{code="500"}) ignore/label-value code
  expr: rate(http_errors_total{code="500"}[2m]) > 0.1
```

## How to enable it

This check is enabled by default for all configured Prometheus servers.

Example:

```js
prometheus "prod" {
  uri     = "https://prometheus-prod.example.com"
  timeout = "60s"
  include = [
    "rules/prod/.*",
    "rules/common/.*",
  ]
}

prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  include = [
    "rules/dev/.*",
    "rules/common/.*",
  ]
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/series"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/series
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/series`
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable promql/series($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable promql/series(prod)
```

You can also disable `promql/series` for specific metric using
`# pint disable promql/series($selector)` comment.

Just like with PromQL if a selector doesn't have any matchers then it will match all instances,

Example:

```yaml
- alert: foo
  # Disable promql/series for any instance of my_metric_name metric selector
  # pint disable promql/series(my_metric_name)
  expr: my_metric_name{instance="a"} / my_metric_name{instance="b"}
```

To disable individual selectors you can pass matchers.

Example:

```yaml
- alert: foo
  # Disable promql/series only for my_metric_name{instance="a"} metric selector
  # pint disable promql/series(my_metric_name{instance="a"})
  expr: my_metric_name{instance="a"} / my_metric_name{instance="b"}
```

Matching is done the same way PromQL matchers work - if the selector from
the query has more matchers than the comment the it will be still matched.

Example:

```yaml
- alert: foo
  # Disable promql/series for any selector at least partially matching {job="dev"}
  # pint disable promql/series({job="dev"})
  expr: my_metric_name{job="dev", instance="a"} / other_metric_name{job="dev", instance="b"}
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/series
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted or `YYYY-MM-DD`.
Adding this comment will disable `promql/series` _until_ `$TIMESTAMP`, after that
check will be re-enabled.
