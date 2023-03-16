---
layout: default
parent: Checks
grand_parent: Documentation
---

# promql/syntax

This is the most basic check that will report any syntax errors in a PromQL
query on any rule.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["promql/syntax"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable promql/syntax
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable promql/syntax
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP promql/syntax
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `promql/syntax` *until* `$TIMESTAMP`, after that
check will be re-enabled.
