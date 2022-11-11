---
layout: default
title: Ignoring problems
parent: Documentation
nav_order: 2
---

# Ignoring selected lines or files

While parsing files pint will look for special comment blocks and use them to
exclude some parts or whole files from checks.

## Ignoring whole files

Add a `# pint ignore/file` comment on top of the file, everything below that line
will be ignored.

Example:

```yaml
# pint ignore/file

groups:
  - name: example
    rules:
    - record: job:http_inprogress_requests:sum
      expr: sum by (job) (http_inprogress_requests)
```

## Ignoring individual lines

To ignore just one line use `# pint ignore/line` at the end of that line or
`# ignore/next-line` on the line before.
This is useful if you're linting templates used to generate Prometheus
configuration and it contains some extra lines that are not valid YAML.

Example:

{% raw %}
```yaml
{% set some_jinja_var1 = "bar" %} # pint ignore/line
groups:
  - name: example
    rules:
    - record: job:http_inprogress_requests:sum
      expr: sum by (job) (http_inprogress_requests)

# pint ignore/next-line
{% set some_jinja_var2 = "foo" %}
```
{% endraw %}

## Ignoring a range of lines

To ignore a part of a file wrap it with `# pint ignore/begin` and
`# pint ignore/end` comments.

Example:

{% raw %}
```yaml
# pint ignore/begin
{% set some_jinja_var1 = "bar" %}
{% set some_jinja_var2 = "foo" %}
# pint ignore/end

groups:
  - name: example
    rules:
    - record: job:http_inprogress_requests:sum
      expr: sum by (job) (http_inprogress_requests)
```
{% endraw %}

## Disabling checks globally

To disable specific check globally, for all files and rules, add it to pint configuration
file. Syntax:

```js
checks {
  disabled = [ "...", ... ]
}
```

Example:

```js
checks {
  disabled = ["alerts/template"]
}
```

## Disabling invididual checks for specific files

To disable individual check for a specific rule use `# pint file/disable ...` comments
anywhere in the file. This will disable given check for all rules in that file.

See each individual [check](checks/index.md) documentation for details.

## Disabling individual checks for specific rules

To disable individual check for a specific rule use `# pint disable ...` comments.
A single comment can only disable one check, so repeat it for every check you wish
to disable.

See each individual [check](checks/index.md) documentation for details.
