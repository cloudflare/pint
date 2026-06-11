parser {
  include    = [".github/pint/rules/.*"]
}

rule {
  match {
    kind = "alerting"
  }

  annotation "summary" {
    severity = "warning"
    required = true
  }

  label "severity" {
    severity = "warning"
    value    = "(critical|warning|info)"
    required = true
  }

  for {
    min      = "1m"
    max      = "1h"
    severity = "warning"
  }
}

rule {
  match {
    kind = "recording"
  }

  aggregate ".+" {
    severity = "warning"
    keep     = ["job"]
  }

  name "((.+):)+.+" {
    severity = "warning"
    comment  = "Recording rules must use the `level:metric:operations` naming convention."
  }
}

rule {
  reject ".* +.*" {
    severity        = "warning"
    label_keys      = true
    annotation_keys = true
  }

  reject "https?://.+" {
    severity     = "warning"
    label_values = true
  }
}

rule {
  range_query {
    max      = "6h"
    severity = "warning"
  }
}

rule {
  match {
    kind = "alerting"
  }

  selector "up" {
    requiredLabels = ["job"]
    severity       = "warning"
  }
}

rule {
  match {
    path = ".*/reject\\.yml"
  }

  report {
    comment  = "Rules in this file are intentionally broken for testing."
    severity = "info"
  }
}
