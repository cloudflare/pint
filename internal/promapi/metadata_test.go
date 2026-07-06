package promapi_test

import (
	"context"
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
		ctx       func(t *testing.T) context.Context
		mock      httpmock.Mocker
		name      string
		assertErr func(t *testing.T, err error)
		metadata  []v1.Metadata
		timeout   time.Duration
	}

	testCases := []testCaseT{
		{
			name:    "offline",
			timeout: time.Second,
			ctx: func(t *testing.T) context.Context {
				return promapi.WithOffline(t.Context(), true)
			},
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "disabled by --offline flag")
			},
			mock: httpmock.New(func(_ *httpmock.Server) {}),
		},
		{
			name:     "gauge",
			timeout:  time.Second,
			metadata: []v1.Metadata{{Type: "gauge", Help: "Text", Unit: ""}},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
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
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
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
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"mixed":[{"type":"gauge","help":"Text1","unit":"abc"},{"type":"counter","help":"Text2","unit":""}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "connection timeout",
			timeout: time.Millisecond * 10,
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "connection timeout")
			},
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
			name:    "empty response",
			timeout: time.Second,
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "unknown: empty response object")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "500 error",
			timeout: time.Second,
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "server_error: 500 Internal Server Error")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^" + promapi.APIPathMetadata)).
					ReturnCode(http.StatusInternalServerError).
					Return("fake error\n").
					UnlimitedTimes()
			}),
		},
		{
			name:     "cached response",
			timeout:  time.Second,
			metadata: []v1.Metadata{{Type: "gauge", Help: "Text", Unit: ""}},
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"cached response":[{"type":"gauge","help":"Text","unit":""}]}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "API error with message",
			timeout: time.Second,
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "bad_data: custom error message")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data","error":"custom error message"}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "invalid JSON",
			timeout: time.Second,
			assertErr: func(t *testing.T, err error) {
				require.EqualError(
					t, err,
					`bad_response: JSON parse error: invalid character '}' after object key`,
				)
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"gauge"}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "API error without message",
			timeout: time.Second,
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "bad_data: empty response object")
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(regexp.MustCompile("^"+promapi.APIPathMetadata)).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data"}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "context cancelled",
			timeout: time.Second,
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "context canceled")
			},
			mock: httpmock.New(func(_ *httpmock.Server) {}),
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

			ctx := t.Context()
			if tc.ctx != nil {
				ctx = tc.ctx(t)
			}

			metadata, err := fg.Metadata(ctx, tc.name).Wait()
			tc.assertErr(t, err)
			if metadata != nil {
				require.Equal(t, tc.metadata, metadata.Metadata)
			}
		})
	}
}
