---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/regexp

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

Another problem this check will report on is redundant regexp anchors.
As noted on [Querying Prometheus](https://prometheus.io/docs/prometheus/latest/querying/basics/)
page Prometheus fully anchors all regex matchers.
So a query match using `foo=~"bar.*"` will be parsed as `foo=~"^bar.*$"` and
so any anchors used in the query will be redundant.
This means that passing `foo=~"^bar.*$"` to the query will be parsed as
`foo=~"^^bar.*$$"`, so both `^` and `$` should be skipped to avoid it.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/regexp"]
}
```

Or you can disable it per rule by adding a comment to it.

`# pint disable promql/regexp`
