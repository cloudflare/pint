package promapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"syscall"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func IsUnavailableError(err error) bool {
	var apiErr *v1.Error
	if ok := errors.As(err, &apiErr); ok {
		return apiErr.Type == v1.ErrServer
	}

	return true
}

type APIError struct {
	Status    string       `json:"status"`
	ErrorType v1.ErrorType `json:"errorType"`
	Error     string       `json:"error"`
}

func decodeError(err error) string {
	if errors.Is(err, context.Canceled) {
		return context.Canceled.Error()
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection refused"
	}

	var neterr net.Error
	if ok := errors.As(err, &neterr); ok && neterr.Timeout() {
		return "connection timeout"
	}

	var apiErr *v1.Error
	if ok := errors.As(err, &apiErr); ok {
		// {"status":"error","errorType":"timeout","error":"query timed out in expression evaluation"}
		// Comes with 503 and ends up being reported as server_error but Detail contains raw JSON
		// which we can parse and fix the error type
		var v1e APIError
		if err2 := json.Unmarshal([]byte(apiErr.Detail), &v1e); err2 == nil && v1e.ErrorType == v1.ErrTimeout {
			apiErr.Type = v1.ErrTimeout
			apiErr.Msg = v1e.Error
		}

		return fmt.Sprintf("%s: %s", apiErr.Type, apiErr.Msg)
	}

	return err.Error()
}
