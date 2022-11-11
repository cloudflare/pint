---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/vector_matching

This check will try to find queries that try to
[match vectors](https://prometheus.io/docs/prometheus/latest/querying/operators/#vector-matching)
but have different sets of labels on both side of the query.

Consider these two time series:

```js
http_errors{job="node-exporter", cluster="prod", instance="server1"}
```

and

```js
cluster:http_errors{job="node-exporter", cluster="prod"}
```

One of them tracks specific instance and one aggregates series for the whole cluster.
Because they have different set of labels if we want to calculate some value using both
of them, for example:

```js
http_errors / cluster:http_errors
```

we wouldn't get any results. To fix that we need ignore extra labels:

```js
http_errors / ignoring(instance) cluster:http_errors
```

This check aims to find all queries that using vector matching where both sides
of the query have different sets of labels causing no results to be returned.

**NOTE**: it's impossible for this check to inspect all time series in Prometheus
against all other series as it would be too expensive.
It will first check if given query returns anything, and
only if it doesn't it will run extra checks. When running extra checks it will sample
only a few time series from each side of the query, so it might not find all possible
issues.

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
  disabled = ["promql/vector_matching"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

`# pint file/disable promql/vector_matching`

Or you can disable it per rule by adding a comment to it. Example:

`# pint disable promql/vector_matching`

If you want to disable only individual instances of this check
you can add a more specific comment.

`# pint disable promql/vector_matching($prometheus)`

Where `$prometheus` is the name of Prometheus server to disable.

Example:

`# pint disable promql/vector_matching(prod)`
