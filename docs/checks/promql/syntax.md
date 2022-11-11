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

`# pint file/disable promql/syntax`

Or you can disable it per rule by adding a comment to it. Example:

`# pint disable promql/syntax`
