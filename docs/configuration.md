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

**NOTE**: all regex patterns are anchored.

## CI

Configure continuous integration environments.

Syntax:

```js
ci {
  include    = [ "(.*)", ... ]
  maxCommits = 20
  baseBranch = "master"
}
```

- `include` - list of file patterns to check when running checks. Only files
  matching those regex rules will be checked, other modified files will be ignored.
- `maxCommits` - by default pint will try to find all commits on the current branch,
  this requires full git history to be present, if we have a shallow clone this
  might fail to find only current branch commits and give us a huge list.
  If the number of commits returned by branch discovery is more than `maxCommits`
  then pint will fail to run.
- `baseBranch` - base branch to compare `HEAD` commit with when calculating the list
  of commits to check.

## Repository

Configure supported code hosting repository, used for reporting PR checks from CI
back to the repository, to be displayed in the PR UI.
Currently it only supports [BitBucket](https://bitbucket.org/) and [GitHub](https://github.com/).

**NOTE**: BitBucket integration requires `BITBUCKET_AUTH_TOKEN` environment variable
to be set. It should contain a personal access token used to authenticate with the API.

**NOTE**: GitHub integration requires `GITHUB_AUTH_TOKEN` environment variable
to be set to a personal access key that can access your repository. Also, `GITHUB_PULL_REQUEST_NUMBER`
environment variable needs to point to the pull request number which will be used whilst
submitting comments.

Syntax:

```js
repository {
  bitbucket {
    uri        = "https://..."
    timeout    = "30s"
    project    = "..."
    repository = "..."
  }
}
```

- `bitbucket:uri` - base URI of this repository, will be used for HTTP
  requests to the BitBucket API.
- `bitbucket:timeout` - timeout to be used for API requests.
- `bitbucket:project` - name of the BitBucket project for this repository.
- `bitbucket:repository` - name of the BitBucket repository.

```js
repository {
  github {
    baseuri    = "https://..."
    uploaduri  = "https://..."
    timeout    = "30s"
    owner      = "..."
    repo       = "..."
  }
}
```

- `github:baseuri` - base URI of GitHub or GitHub enterprise, will be used for HTTP requests to the GitHub API.
- `github:uploaduri` - upload URI of GitHub or GitHub enterprise, will be used for HTTP requests to the GitHub API.

If `github:baseuri` _or_ `github:uploaduri` are not specified then [GitHub](https://github.com) will be used.

- `github:timeout` - timeout to be used for API requests;
- `github:owner` - name of the GitHub owner i.e. the first part that comes before the repository's name in the URI;
- `github:repo` - name of the GitHub repository (e.g. `monitoring`).

## Prometheus servers

Some checks work by querying a running Prometheus instance to verify if
metrics used in rules are present. If you want to use those checks then you
first need to define one or more Prometheus servers.

Syntax:

```js
prometheus "$name" {
  uri     = "https://..."
  timeout = "60s"
  paths   = ["...", ...]
}
```

- `$name` - each defined server should have a unique name that can be used in check
  definitions.
- `uri` - base URI of this Prometheus server, used for API requests and queries.
- `timeout` - timeout to be used for API requests.
- `paths` - optional path filter, if specified only paths matching one of listed regex
  patterns will use this Prometheus server for checks.

Example:

```js
prometheus "prod" {
  uri     = "https://prometheus-prod.example.com"
  timeout = "60s"
}

prometheus "dev" {
  uri     = "https://prometheus-dev.example.com"
  timeout = "30s"
  paths   = [ "alerts/test/.*" ]
}
```

## Matching rules to checks

Most checks, except basic syntax verification, requires some configuration to decide
which checks to run against which files and rules.

Syntax:

```js
rule {
  match {
    path = "(.+)"
    name = "(.+)"
    kind = "alerting|recording"
    command = "ci|lint|watch"
    annotation "(.*)" {
      value = "(.*)"
    }
    label "(.*)" {
      value = "(.*)"
    }
  }
  match { ... }
  match { ... }
  ignore {
    path = "(.+)"
    name = "(.+)"
    kind = "alerting|recording"
    command = "ci|lint|watch"
    annotation "(.*)" {
      value = "(.*)"
    }
    label "(.*)" {
      value = "(.*)"
    }
  }
  ignore { ... }
  ignore { ... }

  [ check definition ]
  ...
  [ check definition ]
}
```

- `match:path` - only files matching this pattern will be checked by this rule
- `match:name` - only rules with names (`record` for recording rules and `alert` for alerting
  rules) matching this pattern will be checked rule
- `match:kind` - optional rule type filter, only rule of this type will be checked
- `match:command` - optional command type filter, this allows to include or ignore rules
  based on the command pint is run with `pint ci`, `pint lint` or `pint watch`.
- `match:annotation` - optional annotation filter, only alert rules with at least one
  annotation matching this pattern will be checked by this rule.
- `match:label` - optional annotation filter, only rules with at least one label
   matching this pattern will be checked by this rule. For recording rules only static
   labels set on the recording rule are considered.
- `ignore` - works exactly like `match` but does the opposite - any alerting or recording rule
  matching all conditions defined on `ignore` will not be checked by this `rule` block.

Note: both `match` and `ignore` require all defined filters to be satisfied to work.
If multiple `match` and/or `ignore` rules are present any of them needs to match for the rule to
be matched / ignored.

Examples:

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
  [ check applied only to severity="critical" and severity="warning" alerts in "ci" or "lint" command is run ]
}
```

```js
rule {
  ignore {
    command = "watch"
  }
  ignore {
    command = "lint"
  }
  [ check applied unless "watch" or "lint" command is run ]
}
```
