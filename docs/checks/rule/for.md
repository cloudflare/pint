---
layout: default
parent: Checks
grand_parent: Documentation
---

# rule/for

This check allows to enforce the presence of `for` field on alerting
rules.
You can configure it to enforce some minimal and/or maximum duration
set on alerts via `for` field.

## Configuration

This check doesn't have any configuration options.

## How to enable it

Syntax:

```js
for {
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

## How to disable it

You can disable this check globally by adding this config block:

```js
checks {
  disabled = ["rule/for"]
}
```

You can also disable it for all rules inside given file by adding
a comment anywhere in that file. Example:

`# pint file/disable rule/for`

Or you can disable it per rule by adding a comment to it. Example:

`# pint disable rule/for`

## How to snooze it

You can disable this check until given time by adding a comment to it. Example:

`# pint snooze $TIMESTAMP rule/for`

Where `$TIMESTAMP` is either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339)
formatted  or `YYYY-MM-DD`.
Adding this comment will disable `rule/duplicate` *until* `$TIMESTAMP`, after that
check will be re-enabled.
