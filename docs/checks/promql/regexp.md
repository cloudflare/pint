---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/regexp

This checks will inspect all query metric selectors that use regexp matchers
to look for common problems.

## Unnecessary regexp selectors

This check will warn if metric selector uses a regexp match but the regexp query
doesn't have any patterns and so a simple string equality match can be used.
Since regexp checks are more expensive using a simple equality check if
preferable.

Example of a query that would trigger this warning:

```js
foo{job=~"bar"}
```

`job=~"bar"` uses a regexp match but since it matches `job` value to a static string
there's no need to use a regexp here and `job="bar"` should be used instead.

Example of a query that wouldn't trigger this warning:

```js
foo{job=~"bar|baz"}
```

## Redundant anchors

As noted on [Querying Prometheus](https://prometheus.io/docs/prometheus/latest/querying/basics/)
page Prometheus fully anchors all regex matchers.
So a query match using `foo=~"bar.*"` will be parsed as `foo=~"^bar.*$"` and
so any anchors used in the query will be redundant.
This means that passing `foo=~"^bar.*$"` to the query will be parsed as
`foo=~"^^bar.*$$"`, so both `^` and `$` should be skipped to avoid it.
This check will report selectors with redundant anchors.

## Smelly selectors

Metric labels are suppose to be opaque strings that you use for filtering, but sometimes
you might end up with a single label value that is really a few different strings concatenated together,
rather than a few different labels. Which then forces you to use regexp label selectors that target
individual parts of that one very long label.

An example of that is a `job` label that includes too much information and instead of just being a name.
Since the `job` labels value is (by default) equal to the `job_name` field on a scrape configuration block
it's easy to end up with one very long string if you need to create a few similar scrape configs:

```yaml
- job_name: myservice_cluster1_production
  [...]

- job_name: myservice_cluster2_production
  [...]

- job_name: myservice_cluster1_staging
  [...]
```

In the configs above we end up with `job` label holding extra information:

- the environment in which the service is deployed (`production` or `staging`)
- the name of the cluster (`cluster1` or `cluster2`)

And the time series we will scrape using this config will look like this:

```js
{job="myservice_cluster1_production", instance="..."}
{job="myservice_cluster2_production", instance="..."}
{job="myservice_cluster1_staging", instance="..."}
```

If we then wanted to query metrics only for the `production` environment we have to use a regexp:

```yaml
- alert: Scrape failed
  expr: up{job=~"myservice_.+_production"} == 0
```

This isn't ideal for a number of reasons:

- It's a lot less obvious what's happening here and why.
- It's much easier to have a typo in a long regexp expression.
- It's a lot harder to aggregate by cluster or environment this way.

This is why regexp selector like this are a code smell that should be avoided.
To avoid using them one can simply set more labels on these scrape jobs:

```yaml
- job_name: myservice_cluster1_production
  [...]
  relabel_configs:
    - target_label: job
      replacement: myservice
    - target_label: cluster
      replacement: cluster1
    - target_label: env
      replacement: production

- job_name: myservice_cluster2_production
  [...]
  relabel_configs:
    - target_label: job
      replacement: myservice
    - target_label: cluster
      replacement: cluster2
    - target_label: env
      replacement: production

- job_name: myservice_cluster1_staging
  [...]
  relabel_configs:
    - target_label: job
      replacement: myservice
    - target_label: cluster
      replacement: cluster1
    - target_label: env
      replacement: staging
```

Which will result in time series with explicit labels:

```js
{job="myservice", cluster="cluster1", env="production", instance="..."}
{job="myservice", cluster="cluster2", env="production", instance="..."}
{job="myservice", cluster="cluster1", env="staging", instance="..."}
```

And simple explicit queries:

```yaml
- alert: Scrape failed
  expr: up{job="myservice", env="production"} == 0
```

## Configuration

This check supports setting extra configuration option to fine tune its behaviour.

Syntax:

```js
check "promql/regexp" {
  smelly = true|false
}
```

- `smelly` - enable or disable reports about smelly selectors. This is enabled by default.

Example:

```js
check "promql/regexp" {
  smelly = false
}
```

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/regexp"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/regexp
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/regexp
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/regexp
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `promql/regexp` *until* `$TIMESTAMP`, after that
check will be re-enabled.
