package promapi_test

import (
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
		mock    httpmock.Mocker
		name    string
		version string
		err     string
		timeout time.Duration
	}

	testCases := []testCaseT{
		// Verifies that a valid buildinfo response is parsed correctly.
		{
			name:    "success",
			timeout: time.Second,
			version: "2.49.0",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathBuildInfo).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"version":"2.49.0","revision":"abc","branch":"HEAD","buildUser":"","buildDate":"","goVersion":"go1.21"}}`).
					UnlimitedTimes()
			}),
		},
		// Verifies that a request timeout produces a connection timeout error.
		{
			name:    "slow",
			timeout: time.Millisecond * 10,
			err:     "connection timeout",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathBuildInfo).
					Run(func(_ *http.Request) ([]byte, error) {
						time.Sleep(time.Second * 2)
						return []byte(`{"status":"success","data":{"version":"2.49.0","revision":"abc","branch":"HEAD","buildUser":"","buildDate":"","goVersion":"go1.21"}}`), nil
					}).
					UnlimitedTimes()
			}),
		},
		// Verifies that a non-2xx HTTP status produces a server error.
		{
			name:    "error",
			timeout: time.Second,
			err:     "server_error: 500 Internal Server Error",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathBuildInfo).
					ReturnCode(http.StatusInternalServerError).
					Return("fake error\n").
					UnlimitedTimes()
			}),
		},
		// Verifies that invalid JSON produces a bad_response error.
		{
			name:    "badJson",
			timeout: time.Second,
			err:     `bad_response: JSON parse error: invalid character '}' after object key`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathBuildInfo).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"xxx"}}`).
					UnlimitedTimes()
			}),
		},
		// Verifies that an API-level error with a custom message is forwarded.
		{
			name:    "apiError",
			timeout: time.Second,
			err:     `bad_data: custom error message`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathBuildInfo).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data","error":"custom error message"}`).
					UnlimitedTimes()
			}),
		},
		// Verifies that an API-level error without a message uses the fallback.
		{
			name:    "emptyError",
			timeout: time.Second,
			err:     `bad_data: empty response object`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathBuildInfo).
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

			bi, err := fg.BuildInfo(t.Context())
			if tc.err != "" {
				require.EqualError(t, err, tc.err, tc)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.version, bi.Version)
			}
		})
	}
}
