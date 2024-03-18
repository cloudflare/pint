---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/counter

This check will find rules with invalid use of counters.
[Counters](https://prometheus.io/docs/concepts/metric_types/#counter) track the number of events over time and so the value of a counter can only grow and never decrease.
This means that the absolute value of a counter doesn't matter, it will be a random number that depends on the number of events that happened since your application was started.
To use the value of a counter in PromQL you most likely want to calculate the rate of events using the [rate()](https://prometheus.io/docs/prometheus/latest/querying/functions/#rate) function, or any other function that is safe to use with counters.
Once you calculate the rate you can use that result in other functions or aggregations that are not counter safe, like [sum()](https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators).`

Here's an example of invalid alerting rules that uses a counter metric called `errors_total`.
This metric will be incremented every time there's an error.

A bad rule could look like this:

```yaml
- alert: Too many errors
  expr: errors_total > 10
```

The problem here is that a counter like `errors_total` will only go up in value until:

- the value overflows the maximum value for a float
- your service restarts and resets the value of `errors_total` to zero - so it starts counting again

Once there are 11 errors observed since your application started `Too many errors` alert will fire and will keep firing
until your application restarts.
This kind of alerts is usually unhelpful and what you really want to track is the health of your application
**right now**. This alert should be triggered if, for example, in the last 1 hour there were more than N errors.
If there's a spike of errors but then errors stop, then the alert should stop firing.

Example of a better rule:

```yaml
- alert: Too many errors
  expr: rate(errors_total[1h]) > 10
```

## Common problems

### Metadata mismatch

Metric type checks are using
[metadata API](https://prometheus.io/docs/prometheus/latest/querying/api/#querying-metric-metadata).
Metadata is aggregated from all scraped metrics.

This can cause a few potential problems:

- You might have the same metric reported with multiple different types and Prometheus or pint won't know
  which time series is which type, because all we have to match a metric to a type is its name.
  Best solution here is to never export same name as multiple metrics with different types.
- If you change the typo of some exported metric then the old type will still show up in metadata,
  plus the new one, as long as there's at least one target still exporting old metric type.
  If you accidentally exported some metric with wrong type, then fixed it, but pint is still complaining,
  then it's very likely that you didn't release your fix to all targets yet.

## Configuration

This check doesn't have any configuration options.

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
  disabled = ["promql/counter"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/counter
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/counter
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable promql/counter($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable promql/counter(prod)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/counter
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `promql/counter` *until* `$TIMESTAMP`, after that
check will be re-enabled.
