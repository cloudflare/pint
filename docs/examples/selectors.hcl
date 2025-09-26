rule {
  # Only match alerting rules.
  match {
    kind = "alerting"
  }

  # All alerts using the up metric must specify the "job" label.
  #
  # Good:
  #
  # - alert: TargetDown
  #   expr: up{job="foo"} == 0
  #
  # Bad:
  #
  # - alert: TargetDown
  #   expr: up
  #
  selector "up" {
    requiredLabels = ["job"]
  }

  # All alerts using the absent() or absent_over_time() call must specify
  # the "team" label:
  #
  # Good:
  #
  # - alert: MetricAbsent
  #   expr: absent(my_metric{team="foo"})
  #
  # Bad:
  #
  # - alert: MetricAbsent
  #   expr: absent(my_metric{})
  #
  call "absent|absent_over_time" {
    selector ".+" {
      requiredLabels = ["team"]
    }
  }
}
