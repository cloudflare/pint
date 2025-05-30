---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/fragile

This check will try to find alerting rules that might produce flapping alerts.

## Sampling functions

If you use sampling functions like `topk()` in alerting rules you might end up with flapping alerts.
Consider this rule:

```yaml
- alert: oops
  expr: topk(10, mymetric > 0)
```

The `topk(10, mymetric > 0)` query used here might return a different set of time series on each rule evaluation.
Different time series will have different labels, and because labels from returned time series are added to the
alert labels it means that a different set of alerts will fire each time.
This will cause flapping alerts from this rule.

## Arithmetic operations between aggregations

If you have an alerting rule that uses two aggregations via metrics from different targets then
when Prometheus restarts it might cause a false positive alerts.
Consider this rule:

```yaml
- alert: oops
  expr: sum(foo{job="a"}) / sum(bar{job="b}) > 0.1
```

It calculates a ratio using a sum of `foo` that comes from scrape job `a` and a sum of `bar` that comes from a scrape job `b`.
This will work fine but if the Prometheus server where this rule runs is restarted then you might receive a false positive.
This is because when Prometheus is started it doesn't scrape all targets at once, it spreads it over the first scrape interval. Until it finishes scraping all target queries that use aggregation will return results calculated from only a subset of targets. When Prometheus evaluates this query for the first time after startup it might see results from more targets
on one side of the query. For example:

```js
sum(foo{job="a"}) <--- This query will have results from all job="a" targets ready.
/
sum(bar{job="b})  <--- This query will have results only from one job="b" target ready.
> 0.1
```

In such a situation the result might be artificially high value and so be above the configured threshold, causing an alert.
To make it worse, when you run this query yourself to debug the alert, all targets would have been scraped and you won't
be able to replicate the data that caused this alert to fire.

The easiest way to avoid this situation is to add `for` option to your alerting rule with the value equal to at least one
scrape interval, for example:

```yaml
- alert: oops
  expr: sum(foo{job="a"}) / sum(bar{job="b}) > 0.1
  for: 2m
```

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/fragile"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/fragile
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/fragile
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/fragile
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `promql/fragile` *until* `$TIMESTAMP`, after that
check will be re-enabled.
