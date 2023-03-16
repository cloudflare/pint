---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/rate

This check inspects `rate()` and `irate()` function calls used in queries
to verify that:
- [Range queries](https://prometheus.io/docs/prometheus/latest/querying/basics/#range-vector-selectors)
  are using a valid time duration.
  This is done by first getting global `scrape_interval` value for selected
  Prometheus servers and comparing duration to it.
  It will report a bug if duration is less than 2x `scrape_interval` because
  Prometheus must have at least two samples to be able to calculate rate, so
  the time range used in queries must be at least 2x `scrape_interval` value.
- Metrics passed to `rate()` and `irate()` are counters.
  Both functions only work with counters and, although any metric type can be
  passed to it and will return calculated value, using a non-counter will cause
  problems. This is because counters are only allowed to increase in value and any
  value drop is interpreted as counter overflow.
  For gauge metrics use [`delta()`](https://prometheus.io/docs/prometheus/latest/querying/functions/#delta)
  or [`deriv()`](https://prometheus.io/docs/prometheus/latest/querying/functions/#deriv) 
  functions instead.
- `rate()` is never called on result of `sum(counter)` since that will always return
  invalid results.
  Chaining `rate(sum(...))` is only possible when passing a metric produced via recording rules
  to `rate()` and so pint will try to find such chains.
  See [this blog post](https://www.robustperception.io/rate-then-sum-never-sum-then-rate/)
  for details.

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
  disabled = ["promql/rate"]
}
```


You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/rate
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/rate
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable promql/rate($prometheus)
```

Where `$prometheus` is the name of Prometheus server to disable.

Example:

```yaml
# pint disable promql/rate(prod)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/rate
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `promql/rate` *until* `$TIMESTAMP`, after that
check will be re-enabled.
