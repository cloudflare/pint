---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/dependency

This check only works when running `pint ci` and will validate that any
removed recording rule isn't still being used by other rules.

Removing any recording rule that is a dependency of other rules is likely
to make them stop working, unless there's some other source of the metric
that was produced by the removed rule.

Example: consider this two rules, one generates `down:count` metric
that is then used by the alert rule:

```yaml
groups:
- name: ...
  rules:
  - record: down:count
    expr: count(up == 0) by(job)
  - alert: Job is down
    expr: down:count > 0
```

If we were to edit this file and delete the recording rule:

```yaml
groups:
- name: ...
  rules:
  - alert: Job is down
    expr: down:count > 0
```

This would leave our alert rule broken because Prometheus would
no longer have `down:count` metric.

This check tries to detect scenarios like this but works across all
files.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/dependency"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable rule/dependency
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable rule/dependency
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP rule/dependency
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `rule/dependency` *until* `$TIMESTAMP`, after that
check will be re-enabled.
