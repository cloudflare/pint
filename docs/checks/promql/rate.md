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
  For gauge metrics please use [`deriv()`](https://prometheus.io/docs/prometheus/latest/querying/functions/#deriv)
  function instead.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default for all configured Prometheus servers.

Example:

```js
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

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/rate"]
}
```

Or you can disable it per rule by adding a comment to it:

`# pint disable promql/rate`

If you want to disable only individual instances of this check
you can add a more specific comment.

`# pint disable promql/rate($prometheus)`

Where `$prometheus` is the name of Prometheus server to disable.

Example:

`# pint disable promql/rate(prod)`