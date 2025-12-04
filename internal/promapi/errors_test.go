package promapi

import (
	"testing"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/require"
)

func TestDecodeErrorType(t *testing.T) {
	type testCaseT struct {
		input    string
		expected v1.ErrorType
	}

	testCases := []testCaseT{
		{input: "bad_data", expected: v1.ErrBadData},
		{input: "timeout", expected: v1.ErrTimeout},
		{input: "canceled", expected: v1.ErrCanceled},
		{input: "execution", expected: v1.ErrExec},
		{input: "bad_response", expected: v1.ErrBadResponse},
		{input: "server_error", expected: v1.ErrServer},
		{input: "client_error", expected: v1.ErrClient},
		{input: "unknown_type", expected: ErrUnknown},
		{input: "", expected: ErrUnknown},
		{input: "random", expected: ErrUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := decodeErrorType(tc.input)
			require.Equal(t, tc.expected, result)
		})
	}
}
