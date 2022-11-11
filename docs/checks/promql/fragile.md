---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/fragile

This check will try to find rules with queries that can be rewritten in a way
which makes them more resilient to label changes.

Example:

Let's assume we have these metrics:

```js
errors{cluster="prod", instance="server1", job="node_exporter"} 5
requests{cluster="prod", instance="server1", job="node_exporter", rack="a"} 10
requests{cluster="prod", instance="server1", job="node_exporter", rack="b"} 30
requests{cluster="prod", instance="server1", job="node_exporter", rack="c"} 25
```

If we want to calculate the ratio of errors to requests we can use this query:

```js
errors / sum(requests) without(rack)
```

`sum(requests) without(rack)` will produce this result:

```js
requests{cluster="prod", instance="server1", job="node_exporter"} 65
```

Both sides of the query will have exact same set of labels:

```js
{cluster="prod", instance="server1", job="node_exporter"}`
```

which is needed to be able to use a binary expression here, and so this query will
work correctly.

But the risk here is that if at any point we change labels on those metrics we might
end up with left and right hand sides having different set of labels.
Let's see what happens if we add an extra label to `requests` metric.

```js
errors{cluster="prod", instance="server1", job="node_exporter"} 5
requests{cluster="prod", instance="server1", job="node_exporter", rack="a", path="/"} 3
requests{cluster="prod", instance="server1", job="node_exporter", rack="a", path="/users"} 7
requests{cluster="prod", instance="server1", job="node_exporter", rack="b", path="/"} 10
requests{cluster="prod", instance="server1", job="node_exporter", rack="b", path="/login"} 1
requests{cluster="prod", instance="server1", job="node_exporter", rack="b", path="/users"} 19
requests{cluster="prod", instance="server1", job="node_exporter", rack="c", path="/"} 25
```

Our left hand side (`errors` metric) still has the same set of labels:

```js
{cluster="prod", instance="server1", job="node_exporter"}
```

But `sum(requests) without(rack)` will now return a different result:

```js
requests{cluster="prod", instance="server1", job="node_exporter", path="/"} 38
requests{cluster="prod", instance="server1", job="node_exporter", path="/users"} 26
requests{cluster="prod", instance="server1", job="node_exporter", path="/login"} 1
```

We no longer get a single result because we only aggregate by removing `rack` label.
Newly added `path` label is not being aggregated away so it splits our results into
multiple series. Since our left hand side doesn't have any `path` label it won't
match any of the right hand side result and this query won't produce anything.

One solution here is to add `path` to `without()` to remove this label when aggregating,
but this requires updating all queries that use this metric every time labels change.

Another solution is to rewrite this query with `by()` instead of `without()` which
will ensure that extra labels will be aggregated away automatically:

```js
errors / sum(requests) by(cluster, instance, job)
```

The list of labels we aggregate by doesn't have to match exactly with the list
of labels on the left hand side, we can use `on()` to instruct Prometheus
which labels should be used to match both sides.
For example if we would remove `job` label during aggregation we would once
again have different sets of labels on both side, but we can fix that
by adding labels we use in `by()` to `on()`:

```js
errors / on(cluster, instance) sum(requests) by(cluster, instance)
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

`# pint file/disable promql/fragile`

Or you can disable it per rule by adding a comment to it. Example:

`# pint disable promql/fragile`
