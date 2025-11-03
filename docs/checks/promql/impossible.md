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
