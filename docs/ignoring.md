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

## Disabling individual checks for specific files

To disable individual check for a specific rule use `# pint file/disable ...` comments
anywhere in the file. This will disable given check for all rules in that file.

See each individual [check](checks/index.md) documentation for details.

You can also use tags set on Prometheus configuration blocks inside comments.
Tags must use `+` prefix, so if you want to disable `promql/series` check on all
Prometheus servers with `testing` tag then add this comment:

```yaml
# pint file/disable promql/series(+testing)
```

## Disabling individual checks for specific rules

To disable individual check for a specific rule use `# pint disable ...` comments.
A single comment can only disable one check, so repeat it for every check you wish
to disable.

See each individual [check](checks/index.md) documentation for details.

Checks can also be disabled for specific Prometheus servers using
`# pint disable ...($prometheus)` comments. Replace `$prometheus` with the name
of the Prometheus configuration block in pint that you want to disable it for.
Examples:

If you have a `stating` Prometheus configuration block in pint config file:

```js
prometheus "staging" {
  uri  = "https://prometheus-staging.example.com"
  tags = ["testing"]
}
```

and have a rule where you want to disable `promql/series` checks run against that
Prometheus server then add a comment:

```yaml
# pint disable promql/series(staging)
```

You can also use tags set on Prometheus configuration blocks inside comments.
Tags must use `+` prefix, so if you want to disable `promql/series` check on all
Prometheus servers with `testing` tag then add this comment:

```yaml
# pint disable promql/series(+testing)
```

## Snoozing checks

If you want to disable some checks just for some time then you can snooze them
instead of disabling forever.

The difference between `# pint disable ...` and `# pint snooze ...` comments is that
the snooze comment must include a timestamp. Selected check will be disabled *until*
that timestamp.
Timestamp must either use [RFC3339](https://www.rfc-editor.org/rfc/rfc3339) syntax
or `YYYY-MM-DD` (if you don't care about time and want to snooze until given date).
Examples:

```yaml
# pint snooze 2023-01-12T10:00:00Z promql/series
# pint snooze 2023-01-12 promql/rate
- record: ...
  expr: ...
```

Just like with `# pint disable ...` you can also use tags with snooze comments.

```yaml
# pint snooze 2023-01-12T10:00:00Z promql/series(+tag)
# pint snooze 2023-01-12 promql/rate(+tag)
- record: ...
  expr: ...
```

If you want to snooze some checks for the entire file then you can use
`# pint file/snooze ...` comment anywhere in given file.
