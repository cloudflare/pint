---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/for

This check allows to enforce the presence of `for` or `keep_firing_for` field
on alerting rules.
You can configure it to enforce some minimal and/or maximum duration
set on alerts via `for` and/or `keep_firing_for` fields.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check uses either `for` or `keep_firing_for` configuration
blocks, depending on which alerting rule field you want to enforce.

Syntax:

```js
for|keep_firing_for {
  severity = "bug|warning|info"
  min      = "5m"
  max      = "10m"
}
```

- `severity` - set custom severity for reported issues, defaults to a bug.
- `min` - minimum required `for` value for matching alerting rules.
  If not set minimum `for` duration won't be enforced.
- `max` - maximum allowed `for` value for matching alerting rules.
- If not set maximum `for` duration won't be enforced.

Example:

Enforce that all alerts have `for` fields of `5m` or more:

```js
for {
  severity = "bug"
  min      = "5m"
  max      = "10m"
}
```

Enforce that all alerts have `keep_firing_for` fields with no more than `1h`:

```js
keep_firing_for {
  severity = "bug"
  max      = "1h"
}
```

To enforce both at the same time:

```js
for {
  severity = "bug"
  min      = "5m"
  max      = "10m"
}

keep_firing_for {
  severity = "bug"
  max      = "1h"
}
```

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/for"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

```yaml
# pint file/disable rule/for
```

Or you can disable it per rule by adding a comment to it. Example:

```yaml
# pint disable rule/for
```

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

```yaml
# pint snooze $TIMESTAMP rule/for
```

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `rule/duplicate` *until* `$TIMESTAMP`, after that
check will be re-enabled.
