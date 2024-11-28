---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/report

This check will always report a problem for **every matching rule**.

## Configuration

Syntax:

```js
report {
  comment  = "..."
  severity = "bug|warning|info"
}
```

- `comment` - set a custom comment that will be added to reported problems.
- `severity` - set custom severity for reported issues.

## How to enable it

This check is not enabled by default as it requires explicit configuration
to work.
To enable it add one or more `rule {...}` blocks that matches some rules and
then add a `report` block there.

Example where we block all recording rule:

```js
rule {
  match {
    kind = "recording"
  }

  report {
    comment  = "You cannot add any recording rules to this Prometheus server."
    severity = "bug"
  }
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/report"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable rule/report
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable rule/report
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP rule/report
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted or `YYYY-MM-DD`.
Adding this comment will disable `rule/report` _until_ `$TIMESTAMP`, after that
check will be re-enabled.
