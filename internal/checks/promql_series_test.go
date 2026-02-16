package checks_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/common/model"

	"github.com/cloudflare/pint/internal/checks"
	"github.com/cloudflare/pint/internal/promapi"
)

func newSeriesCheck(prom *promapi.FailoverGroup) checks.RuleChecker {
	return checks.NewSeriesCheck(prom)
}

func TestSeriesCheck(t *testing.T) {
	testCases := []checkTest{
		{
			description: "ignores rules with syntax errors",
			content:     "- record: foo\n  expr: sum(foo) without(\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "bad response",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithBadData(),
				},
			},
		},
		{
			description: "bad uri",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newSeriesCheck,
			prometheus: func(_ string) *promapi.FailoverGroup {
				return simpleProm("prom", "http://127.127.127.127:9999", time.Second*5, false)
			},
			problems: true,
		},
		{
			description: "overload",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithTooManySamples(),
				},
			},
		},
		{
			description: "expanding series: context deadline exceeded",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithTimeoutExpandingSeriesSamples(),
				},
			},
		},
		{
			description: "simple query",
			content:     "- record: foo\n  expr: sum(notfound)\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "simple query / duplicated metric",
			content:     "- record: foo\n  expr: count(notfound) / sum(notfound)\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "complex query",
			content:     "- record: foo\n  expr: sum(found_7 * on (job) sum(sum(notfound))) / found_7\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{requireQueryPath, formCond{key: "query", value: "count(\nfound_7\n)"}},
					resp:  respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "label_replace()",
			content: `
- alert: foo
  expr: |
    count(
      label_replace(
        node_filesystem_readonly{mountpoint!=""},
        "device",
        "$2",
        "device",
        "/dev/(mapper/luks-)?(sd[a-z])[0-9]"
      )
    ) by (device,instance) > 0
    and on (device, instance)
    label_replace(
      disk_info{type="sat",interface_speed!="6.0 Gb/s"},
      "device",
      "$1",
      "disk",
      "/dev/(sd[a-z])"
    )
  for: 5m
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\ndisk_info{interface_speed!=\"6.0 Gb/s\",type=\"sat\"}\n)"},
					},
					resp: respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nnode_filesystem_readonly{mountpoint!=\"\"}\n)"},
					},
					resp: respondWithSingleInstantVector(),
				},
			},
		},
		{
			description: "offset",
			content:     "- record: foo\n  expr: node_filesystem_readonly{mountpoint!=\"\"} offset 5m\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nnode_filesystem_readonly{mountpoint!=\"\"}\n)"},
					},
					resp: respondWithSingleInstantVector(),
				},
			},
		},
		{
			description: "negative offset",
			content:     "- record: foo\n  expr: node_filesystem_readonly{mountpoint!=\"\"} offset -15m\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nnode_filesystem_readonly{mountpoint!=\"\"}\n)"},
					},
					resp: respondWithSingleInstantVector(),
				},
			},
		},
		{
			description: "duplicate label matchers",
			content:     "- record: foo\n  expr: found{job=\"a\", job=\"b\"}\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithSingleInstantVector(),
				},
			},
		},
		{
			description: "#1 series present",
			content:     "- record: foo\n  expr: found > 0\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithSingleInstantVector(),
				},
			},
		},
		{
			description: "#1 query error",
			content:     "- record: foo\n  expr: found > 0\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithInternalError(),
				},
			},
		},
		{
			description: "#2 series never present",
			content:     "- record: foo\n  expr: sum(notfound)\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "#2 series never present, custom range",
			content:     "- record: foo\n  expr: sum(notfound)\n",
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					LookbackRange: "3d",
					LookbackStep:  "6m",
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "#2 series never present but recording rule provides it correctly",
			content:     "- record: foo\n  expr: sum(foo:bar{job=\"xxx\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			entries:     mustParseContent("- record: foo:bar\n  expr: sum(foo:bar)\n"),
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo:bar{job=\"xxx\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo:bar\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#2 series never present but recording rule provides it without results",
			content:     "- record: foo\n  expr: sum(foo:bar{job=\"xxx\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			entries:     mustParseContent("- record: foo:bar\n  expr: sum(foo)\n"),
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo:bar{job=\"xxx\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo:bar\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#2 {ALERTS=...} present",
			content:     "- record: foo\n  expr: count(ALERTS{alertname=\"myalert\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			entries:     mustParseContent("- alert: myalert\n  expr: sum(foo) == 0\n"),
		},
		{
			description: "#2 {ALERTS=...} missing",
			content:     "- record: foo\n  expr: count(ALERTS{alertname=\"myalert\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			entries:     mustParseContent("- alert: notmyalert\n  expr: sum(foo) == 0\n"),
			problems:    true,
		},
		{
			description: "#2 series never present but recording rule provides it, query error",
			content:     "- record: foo\n  expr: sum(foo:bar{job=\"xxx\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			entries:     mustParseContent("- record: foo:bar\n  expr: sum(foo:bar)\n"),
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo:bar{job=\"xxx\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo:bar\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#2 query error",
			content:     "- record: foo\n  expr: found > 0\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithInternalError(),
				},
			},
		},
		{
			description: "#2 series never present but metric ignored",
			content:     "- record: foo\n  expr: sum(notfound)\n",
			checker:     newSeriesCheck,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					IgnoreMetrics: []string{"foo", "bar", "not.+"},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "#3 metric present, label missing",
			content:     "- record: foo\n  expr: sum(found{job=\"foo\", notfound=\"xxx\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{job=\"foo\",notfound=\"xxx\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{notfound=~\".+\"}\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#3 metric present, label query error",
			content:     "- record: foo\n  expr: sum(found{notfound=\"xxx\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{notfound=\"xxx\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{notfound=~\".+\"}\n)"},
					},
					resp: respondWithInternalError(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#3 metric disappeared, only one sample, absent race",
			content:     "- record: foo\n  expr: sum(found{job=\"abc\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{job=\"abc\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-7).Add(time.Minute*5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#3 metric present once",
			content:     "- record: foo\n  expr: sum(found{job=\"abc\", cluster=\"dev\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{cluster=\"dev\",job=\"abc\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{job=\"abc\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{cluster=\"dev\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*-5),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{cluster=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*-5),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#3 metric present once with labels",
			content:     "- record: foo\n  expr: sum(found{job=\"abc\", cluster=\"dev\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{cluster=\"dev\",job=\"abc\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-4),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{job=\"abc\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{cluster=\"dev\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*10),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*-10),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{cluster=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*-10),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*10),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#3 metric present once with labels, failed baseline query",
			content:     "- record: foo\n  expr: sum(found{job=\"abc\", cluster=\"dev\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{cluster=\"dev\",job=\"abc\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-4),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{job=\"abc\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{cluster=\"dev\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*10),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*-10),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{cluster=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*-10),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*10),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithInternalError(),
				},
			},
		},
		{
			description: "#4 metric was present but disappeared 50m ago",
			content:     "- record: foo\n  expr: sum(found{job=\"foo\", instance=\"bar\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=\"bar\",job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Minute*-50),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"job": "foo"},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Minute*-50),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{"instance": "bar"},
								time.Now().Add(time.Minute*-50),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#4 metric was present but disappeared over 1h ago",
			content:     "- record: foo\n  expr: sum(found{job=\"foo\", instance=\"bar\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=\"bar\",job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#4 metric was present but disappeared over 1h ago / ignored",
			content:     "- record: foo\n  expr: sum(found{job=\"foo\", instance=\"bar\"})\n",
			checker:     newSeriesCheck,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					IgnoreMetrics: []string{"foo", "found", "not.+"},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=\"bar\",job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#4 metric was present but disappeared / min-age / ok",
			content: `
- record: foo
  # pint rule/set promql/series ignore/label-value instance
  # pint rule/set promql/series min-age 5d
  expr: sum(found{job="foo", instance="bar"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=\"bar\",job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#4 metric was present but disappeared / min-age / match",
			content: `
- record: foo
  # pint rule/set promql/series(found) min-age 5d
  expr: sum(found{job="foo", instance="bar"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=\"bar\",job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#4 metric was present but disappeared / min-age with selector / match",
			content: `
- record: foo
  # pint rule/set promql/series(found{instance="bar"}) min-age 5d
  expr: sum(found{job="foo", instance="bar"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=\"bar\",job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#4 metric was present but disappeared / min-age / fail",
			content: `
- record: foo
  # pint rule/set promql/series min-age 3d
  expr: sum(found{job="foo", instance="bar"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=\"bar\",job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#4 metric was present but disappeared / min-age / mismatch",
			content: `
- record: foo
  # pint rule/set promql/series(bar) min-age 5d
  expr: sum(found{job="foo", instance="bar"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=\"bar\",job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#4 metric was present but disappeared / min-age / invalid value",
			content: `
- record: foo
  # pint rule/set promql/series(found) min-age foo
  expr: sum(found{job="foo", instance="bar"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=\"bar\",job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4).Add(time.Minute*-5),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#5 metric was present but not with label value",
			content:     "- record: foo\n  expr: sum(found{notfound=\"notfound\", instance=~\".+\", not!=\"negative\", instance!~\"bad\"})\n",
			checker:     newSeriesCheck,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					IgnoreMetrics: []string{"foo", "bar", "found"},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance!~\"bad\",instance=~\".+\",not!=\"negative\",notfound=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{notfound=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=~\".+\"}\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{notfound=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#5 metric was present but not with label value / ignored metric",
			content:     "- record: foo\n  expr: sum(found{notfound=\"notfound\", instance=~\".+\", not!=\"negative\", instance!~\"bad\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{instance!~\"bad\",instance=~\".+\",not!=\"negative\",notfound=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{instance=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{notfound=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{instance=~\".+\"}\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{notfound=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#5 label query error",
			content:     "- record: foo\n  expr: sum(found{error=\"xxx\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{error=\"xxx\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{error=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{error=\"xxx\"}\n)"},
					},
					resp: respondWithInternalError(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#5 high churn labels",
			content:     "- record: foo\n  expr: sum(sometimes{churn=\"notfound\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nsometimes{churn=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nsometimes\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-7).Add(time.Hour),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*10),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-2),
								time.Now().Add(time.Hour*24*-2).Add(time.Minute*20),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nsometimes{churn=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-7).Add(time.Hour),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*10),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-2),
								time.Now().Add(time.Hour*24*-2).Add(time.Minute*20),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nsometimes{churn=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#5 ignored label value via comment",
			content: `
- record: foo
  # pint rule/set promql/series ignore/label-value error
  expr: sum(foo{error="notfound"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo{error=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfoo{error=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#5 ignored label value globally / match name",
			content: `
- record: foo
  expr: sum(foo{error="notfound"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					IgnoreLabelsValue: map[string][]string{
						"foo": {"error"},
					},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo{error=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfoo{error=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#5 ignored label value globally / match selector",
			content: `
- record: foo
  expr: sum(foo{error="notfound"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					IgnoreLabelsValue: map[string][]string{
						"foo{}": {"error"},
					},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo{error=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfoo{error=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#5 ignored label value globally / no match",
			content: `
- record: foo
  expr: sum(foo{error="notfound"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					IgnoreLabelsValue: map[string][]string{
						"foo{cluster=\"dev\"}": {"error"},
					},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			problems: true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo{error=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo{error=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfoo{error=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#5 ignored label value / selector match",
			content: `
- record: foo
  # pint rule/set promql/series(foo{}) ignore/label-value error
  expr: sum(foo{error="notfound"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo{error=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfoo{error=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#5 ignored label value / selector mismatch",
			content: `
- record: foo
  # pint rule/set promql/series(foo{job="bob"}) ignore/label-value error
  expr: sum(foo{error="notfound"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo{error=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo{error=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfoo{error=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#6 metric was always present but label disappeared",
			content:     "- record: foo\n  expr: sum({__name__=\"found\", removed=\"xxx\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n{__name__=\"found\",removed=\"xxx\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{removed=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-6).Add(time.Hour*8),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{removed=\"xxx\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-6).Add(time.Hour*8),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#6 metric was always present but label disappeared / invalid min-age",
			content: `
# pint rule/set promql/series(found) min-age 1e
- record: foo
  expr: sum({__name__="found", removed="xxx"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n{__name__=\"found\",removed=\"xxx\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{removed=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-6).Add(time.Hour*8),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{removed=\"xxx\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-6).Add(time.Hour*8),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#6 metric was always present but label disappeared / less than min-age",
			content: `
# pint rule/set promql/series(found) min-age 3h
- record: foo
  expr: sum({__name__="found", removed="xxx"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n{__name__=\"found\",removed=\"xxx\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{removed=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Minute*-150),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{removed=\"xxx\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Minute*-150),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#7 metric was always present but label only sometimes",
			content:     "- record: foo\n  expr: sum(found{sometimes=\"xxx\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{sometimes=\"xxx\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{sometimes=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-6),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5).Add(time.Hour*8),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4),
								time.Now().Add(time.Hour*24*-3),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-2),
								time.Now().Add(time.Hour*24*-1),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{sometimes=\"xxx\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-6).Add(time.Hour*8),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-4),
								time.Now().Add(time.Hour*24*-3),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-1),
								time.Now().Add(time.Hour*24*-1),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#8 metric is sometimes present",
			content:     "- record: foo\n  expr: sum(sometimes{foo!=\"bar\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nsometimes{foo!=\"bar\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nsometimes\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-7).Add(time.Hour),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-5),
								time.Now().Add(time.Hour*24*-5).Add(time.Minute*10),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-2),
								time.Now().Add(time.Hour*24*-2).Add(time.Minute*20),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "#8 metric is sometimes present due to prometheus downtime",
			content:     "- record: foo\n  expr: sum(sometimes{foo!=\"bar\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nsometimes{foo!=\"bar\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nsometimes\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-3).Add(time.Minute*-5),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-3).Add(time.Hour),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-7),
								time.Now().Add(time.Hour*24*-3).Add(time.Minute*-5),
								time.Minute*5,
							),
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Hour*24*-3).Add(time.Hour),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
			},
		},
		{
			description: "series found, label missing",
			content:     "- record: foo\n  expr: found{job=\"notfound\"}\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfound{job=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\nfound{job=~\".+\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfound{job=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "series missing, label missing",
			content:     "- record: foo\n  expr: notfound{job=\"notfound\"}\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nnotfound{job=\"notfound\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "series missing, {__name__=}",
			content: `
- record: foo
  expr: '{__name__="notfound", job="bar"}'
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n{__name__=\"notfound\",job=\"bar\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "series missing but check disabled",
			content: `
# pint disable promql/series(notfound)
- record: foo
  expr: count(notfound) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, selector disabled",
			content: `
# pint disable promql/series(notfound{job="foo"})
- record: foo
  expr: count(notfound{job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, selector snoozed",
			content: `
# pint snooze 2099-12-31 promql/series(notfound{job="foo"})
- record: foo
  expr: count(notfound{job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, selector snoozed / snooze expired",
			content: `
# pint snooze 2000-12-31 promql/series(notfound{job="foo"})
- record: foo
  expr: count(notfound{job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nnotfound{job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
			problems: true,
		},
		{
			description: "series missing, selector snoozed / snooze mismatch",
			content: `
# pint snooze 2099-12-31 promql/series(notfound{job="bob"})
- record: foo
  expr: count(notfound{job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nnotfound{job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
			problems: true,
		},
		{
			description: "series missing, selector snoozed / bad snooze selector",
			content: `
# pint snooze 2099-12-31 promql/series(notfound{job=foo})
- record: foo
  expr: count(notfound{job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nnotfound{job=\"foo\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
			problems: true,
		},
		{
			description: "series missing, multi-label selector disabled",
			content: `
# pint disable promql/series(notfound{job="foo", instance="xxx"})
- record: foo
  expr: count(notfound{job="foo", instance="xxx"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, multi-label selector disabled with different order",
			content: `
- record: foo
  # pint disable promql/series(notfound{job="foo", instance="xxx"})
  expr: count(notfound{instance="xxx",job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, multi-label selector disabled with subset of labels",
			content: `
# pint disable promql/series(notfound{job="foo"})    
- record: foo
  expr: count(notfound{instance="xxx",job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, multi-label selector snoozed with subset of labels",
			content: `
# pint snooze 2099-12-31 promql/series(notfound{job="foo"})
- record: foo
  expr: count(notfound{instance="xxx",job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, series disabled",
			content: `
# pint disable promql/series(notfound)
- record: foo
  expr: count(notfound{job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, series disabled",
			content: `
# pint disable promql/series(notfound)
- record: foo
  expr: notfound == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, labels disabled",
			content: `
# pint disable promql/series({job="foo"})
- record: foo
  expr: count(notfound{job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, multi-label selector disabled with __name__",
			content: `
# pint disable promql/series({job="foo", __name__="notfound", instance="xxx"})    
- record: foo
  expr: count(notfound{instance="xxx",cluster="dev", job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, labels disabled, regexp",
			content: `
# pint disable promql/series({job=~"foo"})
- record: foo
  expr: count(notfound{job=~"foo", instance!="bob"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, disable comment with labels, regexp selector",
			content: `
# pint disable promql/series({job="foo"})
- record: foo
  expr: count(notfound{job=~"foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "series missing, disable comment with labels, invalid selector",
			content: `
# pint disable promql/series(notfound{job=foo})
- record: foo
  expr: count(notfound{job=~"foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "series missing but check disabled, labels",
			content: `
# pint disable promql/series(notfound)
- record: foo
  expr: count(notfound{job="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing but check disabled, negative labels",
			content: `
# pint disable promql/series(notfound)
- record: foo
  expr: count(notfound{job!="foo"}) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "series missing, disabled comment for labels",
			content: `
# pint disable promql/series(notfound{job="foo"})
- record: foo
  expr: count(notfound) == 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "alert rule using 2 recording rules",
			content:     "- alert: foo\n  expr: sum(foo:count) / sum(foo:sum) > 120\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			entries:     mustParseContent("- record: foo:count\n  expr: count(foo)\n- record: foo:sum\n  expr: sum(foo)\n"),
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo:count\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo:count\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},

				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo:sum\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo:sum\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "__name__=~foo|bar",
			content:     "- alert: NameRegex\n  expr: rate({__name__=~\"(foo|bar)_panics_total\", job=\"myjob\"}[2m]) > 0",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n{__name__=~\"(foo|bar)_panics_total\",job=\"myjob\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\n{__name__=~\"(foo|bar)_panics_total\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "multiple comments",
			content: `
- alert: Purge_Queue_Size
  # coreless_purge_queue_colo_queue_size_min can be ignored
  # pint disable promql/series(coreless_purge_queue_colo_queue_size_min)
  # coreless_purge_queue_colo_queue_size_median can be ignored
  # pint disable promql/series(coreless_purge_queue_colo_queue_size_median)
  expr: coreless_purge_queue_colo_queue_size_min > 50 or coreless_purge_queue_colo_queue_size_median > 100
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "__name__=~ shouldn't run {foo=bar} queries",
			content:     "- alert: NameRegex\n  expr: rate({__name__=~\"(foo|bar)_panics_total\", job=\"myjob\"}[2m]) > 0",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n{__name__=~\"(foo|bar)_panics_total\",job=\"myjob\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\n{__name__=~\"(foo|bar)_panics_total\"}\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "absent(\n{__name__=~\"(foo|bar)_panics_total\",job=~\".+\"}\n)"},
					},
					resp: matrixResponse{
						samples: []*model.SampleStream{
							generateSampleStream(
								map[string]string{},
								time.Now().Add(time.Minute*-50),
								time.Now(),
								time.Minute*5,
							),
						},
					},
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\n{__name__=~\"(foo|bar)_panics_total\",job=\"myjob\"}\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "series missing, failed baseline query",
			content:     "- record: foo\n  expr: count(notfound) == 0\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nnotfound\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithInternalError(),
				},
			},
		},
		{
			description: "metric with fallback / 1",
			content:     "- record: foo\n  expr: sum(sometimes{foo!=\"bar\"} or vector(0))\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "metric with fallback / 2",
			content: `
- alert: foo
  expr: |
    (sum(sometimes{foo!="bar"} or vector(0)))
    or
    (bob > 10)
    or
    vector(1)
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "metric with fallback / 3",
			content: `
- alert: foo
  expr: |
    (sum(sometimes{foo!="bar"} or vector(0)))
    or
    ((bob > 10) or sum(foo) or vector(1))
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "metric with fallback / 4",
			content: `
- alert: foo
  expr: |
    (
      sum(sometimes{foo!="bar"})
      or
      vector(1)
    ) and (
      ((bob > 10) or sum(bar))
      or
      notfound > 0
    )
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "disable comment for prometheus server",
			content: `
# pint disable promql/series(prom)
- record: my_series
  expr: vector(1)
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "disable comment for prometheus server",
			content: `
# pint disable promql/series(+mytag)
- record: my_series
  expr: vector(1)
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "disable comment for a valid series",
			content: `
# pint disable promql/series(foo)
- record: my_series
  expr: count(foo)
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "disable comment with partial selector",
			content: `
# pint disable promql/series(foo{job="foo"})
- record: my_series
  expr: count(foo{env="prod", job="foo"})
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
		},
		{
			description: "disable comment with vector() fallback",
			content: `
- alert: Foo
  # pint disable promql/series(metric1)
  # pint disable promql/series(metric2)
  # pint disable promql/series(metric3)
  expr: |
    (rate(metric2[5m]) or vector(0)) +
    (rate(metric1[5m]) or vector(0)) +
    (rate(metric3{log_name="samplerd"}[5m]) or vector(0))
    > 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "unused rule/set comment or vector",
			content: `
- alert : Foo
  # pint rule/set promql/series(mymetric) ignore/label-value action
  # pint rule/set promql/series(mymetric) ignore/label-value type
  expr: (rate(mymetric{action="failure"}[2m]) or vector(0)) > 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "unused rule/set comment",
			content: `
- alert : Foo
  # pint rule/set promql/series(mymetric) ignore/label-value action
  # pint rule/set promql/series(mymetric) ignore/label-value type
  expr: (rate(mymetric{action="failure"}[2m])) > 0
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithSingleInstantVector(),
				},
			},
		},
		{
			description: "disable comment for a mismatched series",
			content: `
# pint disable promql/series(bar)
# pint disable promql/series(foo)
- record: my_series
  expr: count(bar)
`,
			checker:    newSeriesCheck,
			prometheus: newSimpleProm,
			problems:   true,
		},
		{
			description: "series not present on other servers",
			content:     "- record: foo\n  expr: notfound\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			otherProms: func(uri string) []*promapi.FailoverGroup {
				proms := make([]*promapi.FailoverGroup, 0, 5)
				for i := range 5 {
					proms = append(proms, simpleProm(fmt.Sprintf("prom%d", i), uri+"/other", time.Second, false))
				}
				return proms
			},
			problems: true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requestPathCond{path: "/other" + promapi.APIPathQuery}},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "series present on other servers",
			content:     "- record: foo\n  expr: notfound\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			otherProms: func(uri string) []*promapi.FailoverGroup {
				proms := make([]*promapi.FailoverGroup, 0, 5)
				for i := range 5 {
					proms = append(proms, simpleProm(fmt.Sprintf("prom%d", i), uri+"/other", time.Second, false))
				}
				return proms
			},
			problems: true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requestPathCond{path: "/other" + promapi.APIPathQuery}},
					resp:  respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "series present on other servers / 15",
			content:     "- record: foo\n  expr: notfound\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			otherProms: func(uri string) []*promapi.FailoverGroup {
				proms := make([]*promapi.FailoverGroup, 0, 15)
				for i := range 15 {
					proms = append(proms, simpleProm(fmt.Sprintf("prom%d", i), uri+"/other", time.Second, false))
				}
				return proms
			},
			problems: true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requestPathCond{path: "/other" + promapi.APIPathQuery}},
					resp:  respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "series present on other servers / timeout",
			content:     "- record: foo\n  expr: notfound\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					FallbackTimeout: "50ms",
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			otherProms: func(uri string) []*promapi.FailoverGroup {
				proms := make([]*promapi.FailoverGroup, 0, 15)
				for i := range 15 {
					proms = append(proms, simpleProm(fmt.Sprintf("prom%d", i), fmt.Sprintf("%s/other/%d", uri, i), time.Second, false))
				}
				return proms
			},
			problems: true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requestPathCond{path: "/other/0" + promapi.APIPathQuery}},
					resp:  respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{requestPathCond{path: "/other/1" + promapi.APIPathQuery}},
					resp:  respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{requestPathCond{path: "/other/2" + promapi.APIPathQuery}},
					resp: sleepResponse{
						sleep: time.Millisecond * 100,
						resp:  respondWithSingleInstantVector(),
					},
				},
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "series present on other servers / timeout 2",
			content:     "- record: foo\n  expr: notfound\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					FallbackTimeout: "5s",
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			otherProms: func(uri string) []*promapi.FailoverGroup {
				proms := make([]*promapi.FailoverGroup, 0, 30)
				for i := range 30 {
					proms = append(proms, simpleProm(fmt.Sprintf("prom%d", i), uri+"/other", time.Second, false))
				}
				return proms
			},
			problems: true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requestPathCond{path: "/other" + promapi.APIPathQuery}},
					resp: sleepResponse{
						sleep: time.Millisecond * 230,
						resp:  respondWithSingleInstantVector(),
					},
				},
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "unsupported query",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		{
			description: "unsupported range query",
			content:     "- record: foo\n  expr: sum(foo)\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			problems:    true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{requireQueryPath},
					resp:  respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{requireRangeQueryPath},
					resp:  httpResponse{code: http.StatusNotFound, body: "Not Found"},
				},
			},
		},
		{
			description: "foo unless bar",
			content:     "- record: foo\n  expr: foo unless bar\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleInstantVector(),
				},
			},
		},
		{
			description: "foo unless bar > 5",
			content:     "- record: foo\n  expr: foo unless bar > 5\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nbar\n)"},
					},
					resp: respondWithSingleInstantVector(),
				},
			},
		},
		{
			description: "metric missing but found on others / ignoreMatchingElsewhere match",
			content: `
- record: foo
  expr: sum(foo)
`,
			checker: newSeriesCheck,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					IgnoreMatchingElsewhere: []string{`{job="bar"}`},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			otherProms: func(uri string) []*promapi.FailoverGroup {
				proms := make([]*promapi.FailoverGroup, 0, 30)
				for i := range 30 {
					proms = append(proms, simpleProm(fmt.Sprintf("prom%d", i), uri+"/other", time.Second, false))
				}
				return proms
			},
			prometheus: newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requestPathCond{path: "/other" + promapi.APIPathQuery},
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{
						requestPathCond{path: "/other" + promapi.APIPathQuery},
						formCond{key: "query", value: `foo`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "instance1",
								"job":      "foo",
							}),
							generateSample(map[string]string{
								"instance": "instance2",
								"job":      "foo",
							}),
							generateSample(map[string]string{
								"instance": "instance3",
								"job":      "bar",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "metric missing but found on others / ignoreMatchingElsewhere mismatch",
			content: `
- record: foo
  expr: sum(foo)
`,
			checker: newSeriesCheck,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					IgnoreMatchingElsewhere: []string{`{job="bar"}`},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			otherProms: func(uri string) []*promapi.FailoverGroup {
				proms := make([]*promapi.FailoverGroup, 0, 30)
				for i := range 30 {
					proms = append(proms, simpleProm(fmt.Sprintf("prom%d", i), uri+"/other", time.Second, false))
				}
				return proms
			},
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requestPathCond{path: "/other" + promapi.APIPathQuery},
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{
						requestPathCond{path: "/other" + promapi.APIPathQuery},
						formCond{key: "query", value: `foo`},
					},
					resp: vectorResponse{
						samples: []*model.Sample{
							generateSample(map[string]string{
								"instance": "instance1",
								"job":      "foo",
							}),
							generateSample(map[string]string{
								"instance": "instance2",
								"job":      "foo",
							}),
							generateSample(map[string]string{
								"instance": "instance3",
								"job":      "bar1",
							}),
						},
					},
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "metric missing but found on others / ignoreMatchingElsewhere error",
			content: `
- record: foo
  expr: sum(foo)
`,
			checker: newSeriesCheck,
			ctx: func(ctx context.Context, _ string) context.Context {
				s := checks.PromqlSeriesSettings{
					IgnoreMatchingElsewhere: []string{`{job="bar"}`},
				}
				if err := s.Validate(); err != nil {
					t.Error(err)
					t.FailNow()
				}
				return context.WithValue(ctx, checks.SettingsKey(checks.SeriesCheckName), &s)
			},
			otherProms: func(uri string) []*promapi.FailoverGroup {
				proms := make([]*promapi.FailoverGroup, 30)
				for i := range proms {
					proms[i] = simpleProm(fmt.Sprintf("prom%d", i), uri+"/other", time.Second, false)
				}
				return proms
			},
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requestPathCond{path: "/other" + promapi.APIPathQuery},
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithSingleInstantVector(),
				},
				{
					conds: []requestCondition{
						requestPathCond{path: "/other" + promapi.APIPathQuery},
						formCond{key: "query", value: `foo`},
					},
					resp: respondWithInternalError(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: "metric missing / other server query error",
			content: `
- record: foo
  expr: sum(foo)
`,
			checker: newSeriesCheck,
			otherProms: func(uri string) []*promapi.FailoverGroup {
				return []*promapi.FailoverGroup{
					simpleProm("prom1", uri+"/other", time.Second, false),
				}
			},
			prometheus: newSimpleProm,
			problems:   true,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requestPathCond{path: "/other" + promapi.APIPathQuery},
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithInternalError(),
				},
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nfoo\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithSingleRangeVector1W(),
				},
			},
		},
		{
			description: `absent{job="myjob"}`,
			content:     "- alert: Service Is Missing\n  expr: absent({job=\"myjob\"})",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
			mocks: []*prometheusMock{
				{
					conds: []requestCondition{
						requireQueryPath,
						formCond{key: "query", value: "count(\n{job=\"myjob\"}\n)"},
					},
					resp: respondWithEmptyVector(),
				},
				{
					conds: []requestCondition{
						requireRangeQueryPath,
						formCond{key: "query", value: "count(\nup\n)"},
					},
					resp: respondWithEmptyMatrix(),
				},
			},
		},
		{
			description: "ALERTS without alertname",
			content:     "- record: foo\n  expr: count(ALERTS)\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "ALERTS_FOR_STATE without alertname",
			content:     "- record: foo\n  expr: count(ALERTS_FOR_STATE)\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
		},
		{
			description: "ALERTS with regex alertname",
			content:     "- record: foo\n  expr: count(ALERTS{alertname=~\"my.*\"})\n",
			checker:     newSeriesCheck,
			prometheus:  newSimpleProm,
		},
	}
	runTests(t, testCases)
}
