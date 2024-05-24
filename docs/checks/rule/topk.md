---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/topk

This checks disallows the usage of `topk` or `bottomk` in recording rules.
It is not configurable and will trigger if `topk` or `bottomk` is found in any
part of a recording rule. It will not effect alerting rules.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/topk"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable rule/topk
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable rule/topk
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP rule/topk
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `rule/topk` *until* `$TIMESTAMP`, after that
check will be re-enabled.
