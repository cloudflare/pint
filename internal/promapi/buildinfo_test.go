package promapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestBuildInfo(t *testing.T) {
	type testCaseT struct {
		ctx       func(t *testing.T) context.Context
		mock      httpmock.Mocker
		assertErr func(t *testing.T, err error)
		name      string
		version   string
		timeout   time.Duration
	}

	testCases := []testCaseT{
		{
			name:    "valid response",
			timeout: time.Second,
			version: "2.49.0",
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathBuildInfo).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"version":"2.49.0","revision":"abc","branch":"HEAD","buildUser":"","buildDate":"","goVersion":"go1.21"}}`).
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
				s.ExpectGet(promapi.APIPathBuildInfo).
					Run(func(_ *http.Request) ([]byte, error) {
						time.Sleep(time.Second * 2)
						return []byte(`{"status":"success","data":{"version":"2.49.0","revision":"abc","branch":"HEAD","buildUser":"","buildDate":"","goVersion":"go1.21"}}`), nil
					}).
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
				s.ExpectGet(promapi.APIPathBuildInfo).
					ReturnCode(http.StatusInternalServerError).
					Return("fake error\n").
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
				s.ExpectGet(promapi.APIPathBuildInfo).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"xxx"}}`).
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
				s.ExpectGet(promapi.APIPathBuildInfo).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data","error":"custom error message"}`).
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
				s.ExpectGet(promapi.APIPathBuildInfo).
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

			bi, err := fg.BuildInfo(ctx).Wait()
			tc.assertErr(t, err)
			if bi != nil {
				require.Equal(t, tc.version, bi.Version)
			}
		})
	}
}
