---
layout: default
parent: Checks
grand_parent: Documentation
---

# query/cost

This check is used to calculate cost of a query and optionally report an issue
if that cost is too high. It will run `expr` query from every rule against
selected Prometheus servers and report results.
This check can be used for both recording and alerting rules, but is most
useful for recording rules.

`pint` will try to estimate the number of bytes needed per single time series
and use that to estimate the amount of memory needed for all time series
returned by given query.
The `bytes per time series` number is calculated using this query:

```
avg(avg_over_time(go_memstats_alloc_bytes[2h]) / avg_over_time(prometheus_tsdb_head_series[2h]))
```

Since Go uses garbage collector total Prometheus process memory will be more than the
sum of all memory allocations, depending on many factors like memory pressure,
Go version, GOGC settings etc. The estimate `pint` gives you should be considered
`best case` scenario.

## Configuration

Syntax:

```js
cost {
  severity       = "bug|warning|info"
  maxSeries      = 5000
}
```

- `severity` - set custom severity for reported issues, defaults to a warning.
  This is only used when query result series exceed `maxSeries` value (if set).
  If `maxSeries` is not set or when results count is below it pint will still
  report it as information.
- `maxSeries` - if set and number of results for given query exceeds this value
  it will be reported as a bug (or custom severity if `severity` is set).

## How to enable it

This check is not enabled by default as it requires explicit configuration
to work.
To enable it add one or more `prometheus {...}` blocks and a `rule {...}` block
with this checks config.

Examples:

All rules from files matching `rules/dev/.+` pattern will be tested against
`dev` server. Results will be reported as information regardless of results.

```js
prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  include = ["rules/dev/.+"]
}

rule {
  cost {}
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["query/cost"]
}
```

Or you can disable it per rule by adding a comment to it:

`# pint disable query/cost`

If you want to disable only individual instances of this check
you can add a more specific comment.

### If `maxSeries` is set

`# pint disable query/cost($prometheus:$maxSeries)`

Where `$prometheus` is the name of Prometheus server to disable.

Example:

`# pint disable query/cost(dev:5000)`

### If `maxSeries` is NOT set

`# pint disable query/cost($prometheus)`

Where `$prometheus` is the name of Prometheus server to disable.

Example:

`# pint disable query/cost(dev)`
