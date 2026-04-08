package promapi_test

import (
	"net/http"
	"regexp"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestMetadata(t *testing.T) {
	type testCaseT struct {
		mock     httpmock.Mocker
		name     string
		err      string
		metadata []v1.Metadata
		timeout  time.Duration
	}

	testCases := []testCaseT{
		{
			name:     "gauge",
			timeout:  time.Second,
			metadata: []v1.Metadata{{Type: "gauge", Help: "Text", Unit: ""}},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"gauge":[{"type":"gauge","help":"Text","unit":""}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:     "counter",
			timeout:  time.Second,
			metadata: []v1.Metadata{{Type: "counter", Help: "Text", Unit: ""}},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"counter":[{"type":"counter","help":"Text","unit":""}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "mixed",
			timeout: time.Second,
			metadata: []v1.Metadata{
				{Type: "gauge", Help: "Text1", Unit: "abc"},
				{Type: "counter", Help: "Text2", Unit: ""},
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"mixed":[{"type":"gauge","help":"Text1","unit":"abc"},{"type":"counter","help":"Text2","unit":""}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "slow",
			timeout: time.Millisecond * 10,
			err:     "connection timeout",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^" + promapi.APIPathMetadata)).
					Run(func(_ *http.Request) ([]byte, error) {
						time.Sleep(time.Second * 2)
						return []byte(`{"status":"success","data":{}}`), nil
					}).
					UnlimitedTimes()
			}),
		},
		{
			name:    "empty",
			timeout: time.Second,
			err:     "unknown: empty response object",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "error",
			timeout: time.Second,
			err:     "server_error: 500 Internal Server Error",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^" + promapi.APIPathMetadata)).
					ReturnCode(http.StatusInternalServerError).
					Return("fake error\n").
					UnlimitedTimes()
			}),
		},
		{
			name:     "once",
			timeout:  time.Second,
			metadata: []v1.Metadata{{Type: "gauge", Help: "Text", Unit: ""}},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"once":[{"type":"gauge","help":"Text","unit":""}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "apiError",
			timeout: time.Second,
			err:     "bad_data: custom error message",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data","error":"custom error message"}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "badJson",
			timeout: time.Second,
			err:     `bad_response: JSON parse error: invalid character '}' after object key`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"gauge"}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "emptyError",
			timeout: time.Second,
			err:     `bad_data: empty response object`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data"}`).
					UnlimitedTimes()
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := tc.mock(t)

			fg := promapi.NewFailoverGroup("test", srv.URL(), []*promapi.Prometheus{
				promapi.NewPrometheus("test", srv.URL(), "", nil, tc.timeout, 1, 100, nil),
			}, true, "up", nil, nil, nil)
			reg := prometheus.NewRegistry()
			fg.StartWorkers(reg)
			defer fg.Close(reg)

			metadata, err := fg.Metadata(t.Context(), tc.name)
			if tc.err != "" {
				require.EqualError(t, err, tc.err, tc)
			} else {
				require.NoError(t, err)
			}
			if metadata != nil {
				require.Equal(t, tc.metadata, metadata.Metadata)
			}
		})
	}
}
