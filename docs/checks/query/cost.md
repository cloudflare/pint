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

## Configuration

Syntax:

```js
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

```js
...
  cost {
    bytesPerSample = 4096
  }
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["query/cost"]
}
```

Or you can disable it per rule by adding a comment to it.

### If `maxSeries` is set

`# pint disable promql/aggregate($prometheus:$maxSeries)`

Where `$prometheus` is the name of Prometheus server to disable.

Example:

`# pint disable promql/aggregate(dev:5000)`

### If `maxSeries` is NOT set

`# pint disable promql/aggregate($prometheus)`

Where `$prometheus` is the name of Prometheus server to disable.

Example:

`# pint disable promql/aggregate(dev)`
