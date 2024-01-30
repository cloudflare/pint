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

- `$pattern` - regexp pattern to match rule name on, this can be templated
  to reference checked rule fields, see [Configuration](../../configuration.md)
  for details
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

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/aggregate
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/aggregate
```

If you want to disable only individual instances of this check
you can add a more specific comment.

### If `keep` is set

```yaml
# pint disable promql/aggregate($label:true)
```

Example:

```yaml
# pint disable promql/aggregate(job:true)
```

### If `strip` is set

```yaml
# pint disable promql/aggregate($label:false)
```

Example:

```yaml
# pint disable promql/aggregate(instance:false)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/aggregate
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `promql/aggregate` *until* `$TIMESTAMP`, after that
check will be re-enabled.
