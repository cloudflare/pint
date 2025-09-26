---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/selector

This check is used to enforce rules on how vector selectors must be structured.

## Configuration

Syntax:

```js
selector "$pattern" {
  comment        = "..."
  severity       = "bug|warning|info"
  requiredLabels = [ "...", ... ]
}
```

- `$pattern` - regexp pattern to match time series name used in the vector selector,
  this can be templated to reference checked rule fields, see [Configuration](../../configuration.md)
  for details.
- `comment` - set a custom comment that will be added to reported problems.
- `severity` - set custom severity for reported issues, defaults to a warning.
- `requiredLabels` - list of label names that must be present on all matching vector selectors.

The `selector` block can be optionally wrapped inside a `call` block to restrict this check
only to selectors used inside specific function, syntax:

```js
call "$pattern" {
  selector { ... }
}
```

- `$pattern` - regexp pattern to match the name of the function,
  this can be templated to reference checked rule fields, see [Configuration](../../configuration.md)
  for details.

## How to enable it

This check is not enabled by default as it requires explicit configuration to work.
To enable it add one or more `rule {...}` blocks and specify all required rules there.

Examples:

Ensure that all alerting rules are scoped to specific scrape job we can enforce that they
all have a `job` label filter:

```js
rule {
  match {
    kind = "alerting"
  }
  selector ".+" {
    requiredLabels = ["job"]
    comment        = "All alerts must be scoped to a specific scrape job via the `job` label."
  }
}
```

The above rule would flag this rule:

```yaml
- alert: TargetDown
  expr: up == 0
```

but this one would pass:

```yaml
- alert: TargetDown
  expr: up{job="myjob"}
```

We can also restrict this check only to some metrics, for example only to the `up` metric, by
changing the regexp pattern in the `selector` block from `.+` to `up`:

```js
rule {
  selector "up" { ... }
}
```

The `selector` block can be nested under the `call` block if we want to apply it only to selectors
used inside some specific function. To ensure that all alerts using `absent()` and `absent_over_time()`
call have the `job` label:

```js
rule {
  match {
    kind = "alerting"
  }
  call 
  selector ".+" {
    requiredLabels = ["job"]
    comment        = "All alerts using absent() must be specify the `job` label."
  }
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/selector"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/selector
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/selector
```

If you want to disable only individual instances of this check
you can add a more specific comment.

### If you only have the `selector` config block

```yaml
# pint disable promql/selector($pattern:$label)
```

Example:

```js
selector "up" {
  requiredLabels = ["job"]
}
```

```yaml
# pint disable promql/selector(^up$:job)
```

### If you have the `call` config block

```yaml
# pint disable promql/selector($callPattern:$selectorPattern:$label)
```

Example:

```js
call "absent" {
  selector "up" {
    requiredLabels = ["job"]
  }
}
```

```yaml
# pint disable promql/selector(^absent$:^up$:job)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/selector
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `promql/selector` *until* `$TIMESTAMP`, after that
check will be re-enabled.
