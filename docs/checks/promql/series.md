---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/series

This check will also query Prometheus servers, it is used to warn about queries
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

## Common problems

If you see this check complaining about some metric it's might due to a number
of different issues. Here are some usual cases.

### Your query cannot return anything

  - You are trying to use a metric that is not present in Prometheus at all.
  - Service exporting your metric is not working or no longer being scraped.
  - You are querying wrong Prometheus server.
  - You are trying to filter a metric that exists using a label key that is
    never present on that metric.
  - You are using label value as a filter, but that value is never present.

If that's the case you need to fix you query. Make sure your metric is present
and it has all the labels you expect to see.

### Metrics you are using have unstable labeling scheme

Some time series for the same metric will have label `foo` and some won't.
Although there's nothing technically wrong with this and Prometheus allows
you to do so, this makes querying metrics difficult as results containing
label `foo` will be mixed with other results not having that label.
All queries would effectively need a `{foo!=""}` or `{foo=""}` filter to
select only one variant of this metric.

Best solution here is to fix labeling scheme.

### Metric labels are generated dynamically in response to some activity

Some label values will appear only temporarily, for example if metrics
are generated for serviced HTTP request and they include some details of
those requests that cannot be known ahead of time, like request path or
method.

When possible this can be addressed by initializing metrics with all known
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
  disabled = ["promql/series"]
}
```

Or you can disable it per rule by adding a comment to it:

`# pint disable promql/series`

If you want to disable only individual instances of this check
you can add a more specific comment.

`# pint disable promql/series($prometheus)`

Where `$prometheus` is the name of Prometheus server to disable.

Example:

`# pint disable promql/series(prod)`

You can also disable `promql/series` for specific metric using
`# pint disable promql/series($selector)` comment.

Just like with PromQL if a selector doesn't have any labels then it will match all instances,
if you pass any labels it will only pass time series with those labels.

Disable warnings about missing `my_metric_name`:

```YAML
# pint disable promql/series(my_metric_name)
```

Disable it only for `my_metric_name{cluster="dev"}` but still warn about
`my_metric_name{cluster="prod"}`:

```YAML
# pint disable promql/series(my_metric_name{cluster="dev"})
```
