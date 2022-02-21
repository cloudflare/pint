---
layout: default
parent: Checks
grand_parent: Documentation
---

# yaml/parse

You will only ever see this check reporting problems if a file containing
Prometheus rules to check doesn't parse as valid [YAML](https://yaml.org/),
meaning that pint is unable to read any rules from that file.

This includes basic YAML parser checks but will also fail if a rule
block contains duplicated keys, example:

```yaml
- record: foo
  expr: sum(my_metric)
  expr: sum(my_metric) without(instance)
```

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You cannot disable this check.
