---
layout: default
title: Configuration
parent: Documentation
nav_order: 2
---

# Configuration syntax

## Table of contents

{: .no_toc .text-delta }

1. TOC
{:toc}

## Environment variables

Environment variables can be expanded inside the pint configuration file as `ENV_*` HCL
variables. To use a variable named `FOO`, reference it as `${ENV_FOO}`.

Examples:

If you have `AUTH_KEY` environment variable set that you want to use a header
for Prometheus requests then use this:

```js
prometheus "..." {
  uri = "https://..."
  headers = {
    "X-Auth": "${ENV_AUTH_KEY}"
  }
}
```

## Regexp matchers

All regexp patterns use [Go regexp](https://pkg.go.dev/regexp) module and are fully anchored.
This means that when you pass `.*` regexp expression internally it will be represented as
`^.*$`, where `^` indicates beginning of a string and `$` is the end of string.
This follows [PromQL behaviour](https://prometheus.io/docs/prometheus/latest/querying/basics/)
for consistency with Prometheus.
If you have a string `alice bob john` and you want to match a sub-string `bob`, then be sure to use
`.*bob.*`.

When using regexp matcher in checks configuration you can reference alerting and recording rule
fields in the regexp using [Go text/template](https://pkg.go.dev/text/template) syntax.
Rule fields are exposed as:

- `$alert` - rule `alert` field
- `$record` - rule `record` field
- `$expr` - rule `expr` field
- `$for` - rule `for` field
- `$labels` - rule `labels` map, individual labels can be accessed as `$labels.foo`
- `$annotations` - rule `annotations` map, individual annotations can be accessed as `$annotations.foo`

Accessing a field that's not present in the rule will return an empty string.

## Parser

Configure how pint parses Prometheus rule files.

Syntax:

```js
parser {
  include = [ "(.*)", ... ]
  exclude = [ "(.*)", ... ]
  relaxed = [ "(.*)", ... ]
}
```

- `include` - list of file patterns to check when running checks. Only files
  matching those regexp rules will be checked, other modified files will be ignored.
- `exclude` - list of file patterns to ignore when running checks.
  This option takes precedence over `include`, so if a file path matches both
  `include` & `exclude` patterns, it will be excluded.
- `relaxed` - by default, pint will parse all files in strict mode, where
  all rule files must have the exact syntax Prometheus expects:

  ```yaml
  groups:
    - name: example
      rules:
        - record: ...
          expr: ...
  ```

  If you're using pint to lint rules that are embedded inside a different structure
  you can set this option to allow fuzzy parsing, which will try to find rule
  definitions anywhere in the file, without requiring `groups -> rules -> rule`
  structure to be present.
  This option takes a list of file patterns, all files matching those regexp rules
  will be parsed in relaxed mode.

## Owners

When `pint ci` or `pint lint` is run with `--require-owner` flag it will require
all Prometheus rules to have an owner assigned via comment.
See [rule/owner](checks/rule/owner.md) for details.

Those checks can be further customised by setting a list of allowed owner names.

Syntax:

```js
owners {
  allowed = [ "(.*)", ... ]
}
```

- `allowed` - list of allowed owner names, this option accepts regexp rules.
  When set, all owners set via comments must match at least one entry on this list.

If there's no `owners:allowed` configuration block, or if it's empty, then any
owner name is accepted.

## CI

Configure continuous integration environments.

Syntax:

```js
ci {
  maxCommits = 20
  baseBranch = "master"
}
```

- `maxCommits` - by default, pint will try to find all commits on the current branch,
  this requires full git history to be present, if we have a shallow clone this
  might fail to find only current branch commits and give us a huge list.
  If the number of commits returned by branch discovery is more than `maxCommits`,
  then pint will fail to run.
- `baseBranch` - base branch to compare `HEAD` commit with when calculating the list
  of commits to check.

## Repository

Configure supported code hosting repository, used for reporting PR checks from CI
back to the repository, to be displayed in the PR UI.
Currently supported platforms are:

- [BitBucket](https://bitbucket.org)
- [GitHub](https://github.com)
- [GitLab](https://gitlab.com)

**NOTE**: BitBucket integration requires `BITBUCKET_AUTH_TOKEN` environment variable
to be set. It should contain a personal access token used to authenticate with the API.

**NOTE**: GitHub integration requires `GITHUB_AUTH_TOKEN` environment variable
to be set to a personal access key that can access your repository.

**NOTE**: GitLab integration requires `GITLAB_AUTH_TOKEN` environment variable
to be set to a personal access key that can access your repository.

**NOTE** The pull request number must be known to pint so it can add comments if it detects any problems.
If pint is run as part of GitHub actions workflow, then this number will be detected from `GITHUB_REF`
environment variable. For other use cases, the `GITHUB_PULL_REQUEST_NUMBER` environment variable must be set
with the pull request number.

Syntax:

```js
repository {
  bitbucket { ... }
  github { ... }
  gitlab { ... }
}
```

### BitBucket options

Syntax:

```js
repository {
  bitbucket {
    uri         = "https://..."
    timeout     = "1m"
    project     = "..."
    repository  = "..."
    maxComments = 50
  }
}
```

- `bitbucket:uri` - base URI of this repository, will be used for HTTP
  requests to the BitBucket API.
- `bitbucket:timeout` - timeout to be used for API requests, defaults to 1 minute.
- `bitbucket:project` - name of the BitBucket project for this repository.
- `bitbucket:repository` - name of the BitBucket repository.
- `bitbucket:maxComments` - the maximum number of comments pint can create on a single
  pull request. Default is 50.

### GitHub options

```js
repository {
  github {
    baseuri     = "https://..."
    uploaduri   = "https://..."
    timeout     = "1m"
    owner       = "..."
    repo        = "..."
    maxComments = 50
  }
}
```

- `github:baseuri` - base URI of GitHub or GitHub enterprise, will be used for HTTP requests to the GitHub API.
  If not set, `pint` will try to use the `GITHUB_API_URL` environment variable instead (if set).
- `github:uploaduri` - upload URI of GitHub or GitHub enterprise, will be used for HTTP requests to the GitHub API.
  If not set, `pint` will try to use the `GITHUB_API_URL` environment variable instead (if set).

If `github:baseuri` _or_ `github:uploaduri` are not specified, then [GitHub](https://github.com) will be used.

- `github:timeout` - timeout to be used for API requests, defaults to 1 minute.
- `github:owner` - name of the GitHub owner i.e. the first part that comes before the repository's name in the URI.
  If not set, `pint` will try to use the `GITHUB_REPOSITORY` environment variable instead (if set).
- `github:repo` - name of the GitHub repository (e.g. `monitoring`).
  If not set, `pint` will try to use the `GITHUB_REPOSITORY` environment variable instead (if set).
- `github:maxComments` - the maximum number of comments pint can create on a single pull request. Default is 50.

Most GitHub settings can be detected from environment variables that are set inside GitHub Actions
environment. The only exception is `GITHUB_AUTH_TOKEN` environment variable that must be set
manually.

### GitLab options

```js
repository {
  gitlab {
    uri         = "https://..."
    timeout     = "1m"
    project     = "..."
    maxComments = 50
  }
}
```

- `gitlab:uri` - optional base URI for GitLab API calls when using self hosted setup.
  You don't need to set it if you use repositories hosted on [gitlab.com](https://gitlab.com/).
- `gitlab:timeout` - timeout to be used for API requests, defaults to 1 minute.
- `gitlab:project` - ID of the GitLab repository.
- `gitlab:maxComments` - the maximum number of comments pint can create on a single pull request. Default is 50.

## Prometheus servers

Some checks work by querying a running Prometheus instance to verify if
metrics used in rules are present. If you want to use those checks, then you
first need to define one or more Prometheus servers.

Syntax:

```js
prometheus "$name" {
  uri         = "https://..."
  publicURI   = "https://..."
  failover    = ["https://...", ...]
  tags        = ["...", ...]
  headers     = { "...": "..." }
  timeout     = "2m"
  concurrency = 16
  rateLimit   = 100
  required    = true|false
  include     = ["...", ...]
  exclude     = ["...", ...]
  tls {
    serverName = "..."
    caCert     = "..."
    clientCert = "..."
    clientKey  = "..."
    skipVerify = true|false
  }
}
```

- `$name` - each defined server should have a unique name that can be used in check
  definitions.
- `uri` - base URI of this Prometheus server, used for API requests and queries.
- `publicURI` - optional URI to use instead of `uri` in problems reported to users.
  Set it if Prometheus links used by pint in comments submitted to BitBucket or GitHub
  should use different URIs than the one used by pint when querying Prometheus.
  If not set, `uri` will be used instead.
- `failover` - list of URIs to try (in order they are specified) if `uri` doesn't respond
  to requests or returns an error. This allows one to configure fail-over Prometheus servers
  to avoid CI failures in case the main Prometheus server is unreachable.
  Fail over URIs are not used if Prometheus returns an error caused by the query, like
  `many-to-many matching not allowed`.
  It's highly recommended that all URIs point to Prometheus servers with identical
  configuration, otherwise pint checks might return unreliable results and potential
  false positives.
- `tags` - a list of strings that can be used to group Prometheus servers together.
  Tags cannot contain spaces.
  Tags can be later used when disabling checks via comments, see [ignoring](ignoring.md).
- `headers` - a list of HTTP headers that will be set on all requests for this Prometheus
  server.
- `timeout` - timeout to be used for API requests. Defaults to 2 minutes.
- `concurrency` - how many concurrent requests pint can send to this Prometheus server.
  Optional, defaults to 16.
- `rateLimit` - per second rate limit for all API requests sent to this Prometheus server.
  Setting it to `1000` would allow for up to 1000 requests per each wall clock second.
  Optional, default to 100 requests per second.
- `uptime` - metric selector used to detect gaps in Prometheus uptime.
  Since some checks are sending queries to validate if given metric always present in Prometheus
  they might find gaps when Prometheus itself was down. Pint tries to detect that by querying
  metrics that are always guaranteed to be present when Prometheus is running.
  By default metric used for this is `up`, which is generated by Prometheus itself, see
  [Prometheus docs](https://prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series)
  for details.
  Uptime gap detection works by running a range query `count(up)` and checking for any gaps
  in the response.
  Since the `up` metric can have a lot of time series, `count(up)` might be slow and expensive.
  An alternative is to use one of metrics exposed by Prometheus itself, like `prometheus_build_info`, but
  those metrics are only present if Prometheus is configured to scrape itself, so `up` is used by default
  since it's guaranteed to work in every setup.
  If your Prometheus has a lot of time series and it's configured to scrape itself, then
  it is recommended to set the `uptime` field to `prometheus_build_info`.
- `required` - decides how pint will report errors if it's unable to get a valid response
  from this Prometheus server. If `required` is `true` and all API calls to this Prometheus
  fail, pint will report those as `bug` level problems. If it's set to `false`, pint will
  report those with the `warning` level.
  Default value for `required` is `false`. Set it to `true` if you want to hard fail
  in case of remote Prometheus issues. Note that setting it to `true` might block
  PRs when running `pint ci` until pint is able to talk to Prometheus again.
- `include` - optional path filter, if specified only paths matching one of listed regexp
  patterns will use this Prometheus server for checks.
- `exclude` - optional path filter, if specified any path matching one of listed regexp
  patterns will never use this Prometheus server for checks.
  `exclude` takes precedence over `include.
- `tls` - optional TLS configuration for HTTP requests sent to this Prometheus server.
- `tls:serverName` - server name (SNI) for TLS handshakes. Optional, default is unset.
- `tls:caCert` - path for CA certificate to use. Optional, default is unset.
- `tls:clientCert` - path for client certificate to use. Optional, default is unset.
  If set, `clientKey` must also be set.
- `tls:clientKey` - path for client key file to use. Optional, default is unset.
  If set, `clientCert` must also be set.
- `tls:skipVerify` - if `true` all TLS certificate checks will be skipped.
  Enabling this option can be a security risk; use only for testing.
  Optional, default is false.

Example:

```js
prometheus "prod" {
  uri         = "https://prometheus-prod.example.com"
  tags        = ["prod"]
  headers     = {
    "X-Auth": "secret"
  }
  concurrency = 40
}

prometheus "prod-tls" {
  uri         = "https://prometheus-tls.example.com"
  tags        = ["prod"]
  tls {
    serverName = "prometheus.example.com"
    clientCert = "/ssl/ca.pem"
    clientCert = "/ssl/client.pem"
    clientKey  = "/ssl/client.key"
  }
}

prometheus "staging" {
  uri    = "https://prometheus-staging.example.com"
  uptime = "prometheus_build_info"
}

prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  include = [ "alerts/test/.*" ]
  exclude = [ "alerts/test/docs/.*" ]
}
```

## Prometheus discovery

Sometimes specifying a static list of Prometheus server definitions in pint
configuration is not possible and a dynamic discovery of running Prometheus
instances is needed. This can be configured using `discovery` config blocks.

### File path discovery

File path discovery allows to generate Prometheus server definitions used by pint
based on path patterns on disk.
Syntax:

```js
filepath {
  directory = "..."
  match = "(.*)"
  ignore = [ "(.*)", ... ]
  template { ... }
  template { ... }
}
```

- `directory` - the base directory to scan for paths.
- `match` - a regexp expression to match. Any named capture group defined here
  will be accessible when rendering Prometheus the template.
- `ignore` - a list of regexp rules, any path matching any of these rules will
  be ignored.
- `template` - a template for generating Prometheus server definitions.
  See below.

### Prometheus query discovery

```js
prometheusQuery {
  uri = "https://..."
  headers     = { "...": "..." }
  timeout     = "2m"
  tls {
    serverName = "..."
    caCert     = "..."
    clientCert = "..."
    clientKey  = "..."
    skipVerify = true|false
  }
  template { ... }
  template { ... }
}
```

- `uri` - Prometheus server base URI. This is where the discovery query will be sent.
- `headers` - optional list of headers to set on Prometheus query requests.
- `timeout` - Prometheus request timeout. Defaults to 2 minutes.
- `tls` - optional TLS configuration for Prometheus requests, see `prometheus` block
  documentation for details.
- `query` - the PromQL query to use for discovery of Prometheus servers.
  Every returned time series will generate a new Prometheus server definition using attached
  `template` definition. You can set multiple `template` blocks for each discovery block, each
  returned time series will generate a single Prometheus server for each `template` block.
  You can use labels on returned time series as [Go text/template](https://pkg.go.dev/text/template)
  variables named `$name`. Example: `instance` label will be available as the `$instance` variable.

### Prometheus template

The `template` block is nearly identical to the `prometheus` configuration block, except that
`name` is an explicit field inside the block.

You can use [Go text/template](https://pkg.go.dev/text/template) to render some of the
fields using variables from either regexp capture groups (when using `filepath` discovery)
or metric labels (when using `prometheusQuery` discovery).

Fields that are allowed to be templated are:

- `name`
- `uri`
- `failover`
- `headers`
- `tags`
- `include`
- `exclude`

```js
template {
  name        = "..."
  uri         = "https://..."
  uri         = "https://..."
  failover    = ["https://...", ...]
  tags        = ["...", ...]
  headers     = { "...": "..." }
  timeout     = "2m"
  concurrency = 16
  rateLimit   = 100
  required    = true|false
  include     = ["...", ...]
  exclude     = ["...", ...]
  tls {
    serverName = "..."
    caCert     = "..."
    clientCert = "..."
    clientKey  = "..."
    skipVerify = true|false
  }
}
```

Generated Prometheus servers will be deduplicated. If there are multiple servers with the same
name but different URIs then extra URIs will be added to `failover` list.
Servers must have identical `name`, `tags`, `include` and `exclude` fields to be deduplicated.

### Examples

Each Prometheus server has a sub-directory inside `/etc/prometheus/clusters`
folder. This directory is named after the Prometheus cluster it's part of.
All rules are stored in `/etc/prometheus/clusters/<cluster>/.*.yaml` or
`/etc/prometheus/clusters/<cluster>/.*.yml` files.
Each Prometheus cluster is accessible under `https://<cluster>.prometheus.example.com` URI.

```js
filepath {
  directory = "/etc/prometheus/clusters"
  match     = "(?P<cluster>[a-z]+[0-9]{2})"
  ignore    = [ "staging[0-9]+" ]
  template {
    name     = "cluster-{{ $cluster }}"
    uri      = "https://{{ $cluster }}.prometheus.example.com"
    uptime   = "prometheus_ready"
    include  = [
      "/etc/prometheus/clusters/{{ $cluster }}/.*.ya?ml",
    ]
  }
}
```

## Matching rules to checks

Most checks, except basic syntax verification, requires some configuration to decide
which checks to run against which files and rules.

Syntax:

```js
rule {
  match {
    path    = "(.+)"
    state   = [ "any|added|modified|renamed|unmodified", ... ]
    name    = "(.+)"
    kind    = "alerting|recording"
    command = "ci|lint|watch"
    annotation "(.*)" {
      value = "(.*)"
    }
    label "(.*)" {
      value = "(.*)"
    }
    for = "..."
  }
  match { ... }
  match { ... }
  ignore {
    path    = "(.+)"
    state   = [ "any|added|modified|renamed|unmodified", ... ]
    name    = "(.+)"
    kind    = "alerting|recording"
    command = "ci|lint|watch"
    annotation "(.*)" {
      value = "(.*)"
    }
    label "(.*)" {
      value = "(.*)"
    }
    for = "..."
  }
  ignore { ... }
  ignore { ... }

  [ check definition ]
  ...
  [ check definition ]
}
```

- `match:path` - only files matching this pattern will be checked by this rule.
- `match:state` - only match rules based on their state. Default value for `state` depends on the
  pint command that is being run, for `pint ci` the default value is `["added", "modified", "renamed"]`,
  for any other command the default value is `["any"]`.
  Possible values:
  - `any` - match rule in any state
  - `added` - a rule is being added in a git commit, a rule can only be in this state when running `pint ci` on a pull request.
  - `modified` - a rule is being modified in a git commit, a rule can only be in this state when running `pint ci` on a pull request.
  - `renamed` - a rule is being modified in a git commit, a rule can only be in this state when running `pint ci` on a pull request.
  - `unmodified` - a rule is not being modified in a git commit when running `pint ci` or other pint command was run that doesn't try to detect which rules were modified.
- `match:name` - only rules with names (`record` for recording rules and `alert` for alerting
  rules) matching this pattern will be checked rule.
- `match:kind` - optional rule type filter, only rule of this type will be checked.
- `match:command` - optional command type filter, this allows to include or ignore rules
  based on the command pint is run with `pint ci`, `pint lint` or `pint watch`.
- `match:annotation` - optional annotation filter, only alert rules with at least one
  annotation matching this pattern will be checked by this rule.
- `match:label` - optional annotation filter, only rules with at least one label
  matching this pattern will be checked by this rule. For recording rules only static
  labels set on the recording rule are considered.
- `match:for` - optional alerting rule `for` filter. If set, only alerting rules with the `for`
  field present and matching the provided value will be checked by this rule. Recording rules
  will never match it as they don't have the `for` field.
  Syntax is `OP DURATION` where `OP` can be any of `=`, `!=`, `>`, `>=`, `<`, `<=`.
- `match:keep_firing_for` - optional alerting rule `keep_firing_for` filter. Works the same
  way as `for` match filter.
- `ignore` - works exactly like `match` but does the opposite - any alerting or recording rule
  matching all conditions defined on `ignore` will not be checked by this `rule` block.

Note: both `match` and `ignore` require all defined filters to be satisfied to work.
If multiple `match` and/or `ignore` rules are present any of them needs to match for the rule to
be matched / ignored.

Examples:

Check applied only to severity="critical" and severity="warning" alerts in "ci" or "lint" command is run:

```js
rule {
  match {
    path = "rules/.*"
    kind = "alerting"
    label "severity" {
      value = "(warning|critical)"
    }
  }
  ignore {
    command = "watch"
  }
  check { ... }
}
```

Check applied unless "watch" or "lint" command is run:

```js
rule {
  ignore {
    command = "watch"
  }
  ignore {
    command = "lint"
  }
  check { ... }
}
```

Check applied only to alerting rules with "for" field value that is >= 5m:

```js
rule {
  match {
    for = ">= 5m"
  }
  check { ... }
}
```

Check applied only to alerting rules with "keep_firing_for" field value that is > 15m:

```js
rule {
  match {
    keep_firing_for = "> 15m"
  }
  check { ... }
}
```

Check that is run on all rules, including unmodified files, when running `pint ci`:

```js
rule {
  match {
    state = ["any"]
  }
  check { ... }
}
```
