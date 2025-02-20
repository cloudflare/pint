---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/fragile

This check will try to find alerting rules that might produce flapping alerts
due to how they use sampling functions like `topk()`.

Consider this alert:

```yaml
- alert: oops
  expr: topk(10, mymetric > 0)
```

The `topk(10, mymetric > 0)` query used here might return a different set of time series on each rule evaluation.
Different time series will have different labels, and because labels from returned time series are added to the
alert labels it means that a different set of alerts will fire each time.
This will cause flapping alerts from this rule.

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
