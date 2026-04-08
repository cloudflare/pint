package promapi_test

import (
	"net/http"
	"testing"
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.nhat.io/httpmock"

	"github.com/cloudflare/pint/internal/promapi"
)

func TestFlags(t *testing.T) {
	type testCaseT struct {
		flags   v1.FlagsResult
		mock    httpmock.Mocker
		name    string
		err     string
		timeout time.Duration
	}

	testCases := []testCaseT{
		{
			name:    "default",
			timeout: time.Second,
			flags:   v1.FlagsResult{},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathFlags).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "foo",
			timeout: time.Second,
			flags:   v1.FlagsResult{"foo": "bar"},
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathFlags).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"foo":"bar"}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "slow",
			timeout: time.Millisecond * 10,
			err:     "connection timeout",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathFlags).
					Run(func(_ *http.Request) ([]byte, error) {
						time.Sleep(time.Second * 2)
						return []byte(`{"status":"success","data":{}}`), nil
					}).
					UnlimitedTimes()
			}),
		},
		{
			name:    "error",
			timeout: time.Second,
			err:     "server_error: 500 Internal Server Error",
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathFlags).
					ReturnCode(http.StatusInternalServerError).
					Return("fake error\n").
					UnlimitedTimes()
			}),
		},
		{
			name:    "badJson",
			timeout: time.Second,
			err:     `bad_response: JSON parse error: invalid character '}' after object key`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathFlags).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"success","data":{"xxx"}}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "apiError",
			timeout: time.Second,
			err:     `bad_data: custom error message`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathFlags).
					ReturnHeader("Content-Type", "application/json").
					Return(`{"status":"error","errorType":"bad_data","error":"custom error message"}`).
					UnlimitedTimes()
			}),
		},
		{
			name:    "emptyError",
			timeout: time.Second,
			err:     `bad_data: empty response object`,
			mock: httpmock.New(func(s *httpmock.Server) {
				s.ExpectGet(promapi.APIPathFlags).
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

			flags, err := fg.Flags(t.Context())
			if tc.err != "" {
				require.EqualError(t, err, tc.err, tc)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.flags, flags.Flags)
			}
		})
	}
}
