---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/name

This check allows to match rule names:

- `alert` for alerting rules
- `record` for recording rules

## Configuration

Syntax:

```js
name "$pattern" {
  comment  = "..."
  severity = "bug|warning|info"
}
```

- `$pattern` - regexp pattern to match the name on, this can be templated
  to reference checked rule fields, see [Configuration](../../configuration.md)
  for details.
- `comment` - set a custom comment that will be added to reported problems.
- `severity` - set custom severity for reported issues, defaults to a information.

## How to enable it

This check is not enabled by default as it requires explicit configuration
to work.
To enable it add one or more `rule {...}` blocks and specify all required
labels there.

Example that will require all recording rules to have `rec:` prefix:

```js
rule {
  match {
    kind = "recording"
  }

  name "rec:.+" {
    comment  = "ALl recording rules must use the `rec:` prefix."
    severity = "bug"
  }
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/name"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable rule/name
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable rule/name
```

If you want to disable only individual instances of this check
you can add a more specific comment.

```yaml
# pint disable rule/name($pattern)
```

Example pint rule:

```js
name "rec:.+" {
  comment  = "ALl recording rules must use the `rec:` prefix."
  severity = "bug"
}
```

Example comment disabling that rule:

```yaml
# pint disable rule/name($rec:.+$)
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP rule/name
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted or `YYYY-MM-DD`.
Adding this comment will disable `rule/name` _until_ `$TIMESTAMP`, after that
check will be re-enabled.
