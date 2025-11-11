---
layout: default
parent: Checks
grand_parent: Documentation
---

# yaml/parse

You will only ever see this check reporting problems if a file containing
Prometheus rules to check doesn't parse as valid [YAML](https://yaml.org/),
meaning that pint is unable to read any rules from that file or when
some fields are using the wrong type.

This includes basic YAML parser checks but will also fail if a rule
block contains duplicate keys, example:

```yaml
- record: foo
  expr: sum(my_metric)
  expr: sum(my_metric) without(instance)
```

Syntax checks enforced by pint are more strict than what Prometheus uses,
so a rule definition that fails pint checks might still be parsed by
Prometheus. This is because pint enforces that all fields have the correct type.
For example all annotations are expected to be strings, but the YAML parser
will load any value that can be represented as a string, for example a number:

```yaml
- alert: Foo
  expr: up == 0
  annotations:
    priotity: 1
```

The above rule will work in Prometheus but, for example, if you try to parse
such file using Python to find all rules where `priority` is `"1"` it will skip it,
because Python doesn't know the schema of rule file, so it returns whatever types
it finds:

```python
import yaml

with open("rules.yaml") as f:
    for rule in yaml.safe_load(f):
      if rule["annotations"]["priority"] == "1":
        ...
```

This kind of type confusion can be even more problematic because YAML will
automatically convert certain string values to boolean, for example:

```yaml
- alert: Foo
  expr: up == 0
  annotations:
    critical: no
```

In the above YAML will parse `no` as `false`.
There are other well known gotchas that are caused by YAML complex parsing rules
and the best way to avoid these is to always use explicit types for string.

## Configuration

This check doesn't have any configuration options.

## How to enable it

This check is enabled by default.

## How to disable it

You cannot disable this check.
