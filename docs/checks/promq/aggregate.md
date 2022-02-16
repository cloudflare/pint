---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/aggregate

This check is used to inspect promql expressions and ensure that specific labels
are kept or stripped away when aggregating results. It's mostly useful in recording
rules.

## Configuration

Syntax:

```js
aggregate "$pattern" {
  severity = "bug|warning|info"
  keep = [ "...", ... ]
  strip = [ "...", ... ]
}
```

- `$pattern` - regexp pattern to match rule name on
- `severity` - set custom severity for reported issues, defaults to a warning
- `keep` - list of label names that must be preserved
- `strip` - list of label names that must be stripped

## How to enable it

This check is not enabled by default as it requires explicit configuration
to work.
To enable it add one or more `rule {...}` blocks and specify all required
rules there.

Examples:

Ensure that all series generated from recording rules have `job` labels preserved:

```js
rule {
  match {
    kind = "recording"
  }
  aggregate ".+" {
    keep = ["job"]
  }
}
```

In some cases you might want to ensure that specific labels are removed in aggregations.
For example in recording rules that are producing series consumed by federation, where
only aggregated results (not per instance) are allowed:

```js
rule {
  match {
    kind = "recording"
  }
  aggregate "cluster:.+" {
    strip = ["instance"]
  }
}
```

By default all issues found by this check will be reported as warnings. To adjust
severity set a custom `severity` key:

```js
aggregate ".+" {
  ...
  severity = "bug"
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/aggregate"]
}
```

Or you can disable it per rule by adding a comment to it.

### If `keep` is set

`# pint disable promql/aggregate($label:true)`

Example:

`# pint disable promql/aggregate(job:true)`

### If `strip` is set

`# pint disable promql/aggregate($label:false)`

Example:

`# pint disable promql/aggregate(instance:true)`
