---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/impossible

This check will report PromQL queries that can never return anything.

Example query:

```js
foo{job="bar"} unless sum(foo)
```

The right hand side (`unless sum(foo)`) cannot ever match any time series returned by the left hand side
(`foo{job="bar"}`) because `sum()` strips away all labels. This means that we end up with left hand side
having at least the `job` label, while the right hand side has no labels, so we end up with:

```js
foo{job="bar"} unless {}
```

Both sides can only match if they have the same label set, see [Prometheus docs](https://prometheus.io/docs/prometheus/latest/querying/operators/#vector-matching). A result with `{job="bar"}` will never be matched with empty label set `{}`.

This check will also detect aggregations on labels that are already removed. Example:

```js
group by(cluster) (
  sum(errors)
)
```

In the above `sum(errors)` removes all labels from the results, so there will be no `cluster` label to aggregate
in the `group by(cluster)` outer query.

Similarly this check will warn about joins using labels that are already removed, example:

```js
sum(foo) / on(cluster) count(bar)
```

Since both `sum(...)` calls remove all labels there won't be any `cluster` label to join on.

Another problem detected by this check is binary aggregations that block labels from propagating to the results.
Consider this query:

```js
service_ready * on (cluster) group_left(env, location) cluster_info
```

What happens above is that we are joining our results for the `service_ready` series with an extra time series
`cluster_info` and add two labels (`env` and `location`) to query results.
But what happens if someone wanted to further filter down the results of `service_ready` with a filter like:

```js
service_ready and on (instance) (service_errors == 0)
```

We can simply add it to the query:

```js
service_ready and on (instance) (service_errors == 0) * on (cluster) group_left(env, location) cluster_info
```

But doing it this way splits our query in two parts:

- A: `service_ready`
- B: `(service_errors == 0) * on (cluster) group_left(env, location) cluster_info`

When we the evaluate `A and on(instance) B` we no longer propagate `env` and `location` to the results, because
these labels are only added to the results on the `B` side.
One way to fix this is to ensure that we join extra labels with the entire
`service_ready and on (instance) (service_errors == 0)` expression by wrapping it in parentheses.

```js
(service_ready and on (instance) (service_errors == 0)) * on (cluster) group_left(env, location) cluster_info
```

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/impossible"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/impossible
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/impossible
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/impossible
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `promql/impossible` *until* `$TIMESTAMP`, after that
check will be re-enabled.
