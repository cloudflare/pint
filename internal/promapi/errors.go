package promapi

import (
	"encoding/json"
	"errors"
	"net"
	"strings"
	"syscall"
	"time"

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

func CanRetryError(err error, delta time.Duration) (time.Duration, bool) {
	if errors.Is(err, syscall.ECONNREFUSED) {
		return delta, false
	}

	var neterr net.Error
	if ok := errors.As(err, &neterr); ok && neterr.Timeout() {
		return (delta / 2).Round(time.Minute), true
	}

	var apiErr *v1.Error
	if ok := errors.As(err, &apiErr); ok {
		// {"status":"error","errorType":"timeout","error":"query timed out in expression evaluation"}
		// Comes with 503 and ends up being reported as server_error but Detail contains raw JSON
		// which we can parse and fix the error type
		var v1e APIError
		if err2 := json.Unmarshal([]byte(apiErr.Detail), &v1e); err2 == nil && v1e.ErrorType == v1.ErrTimeout {
			apiErr.Type = v1.ErrTimeout
		}

		switch apiErr.Type {
		case v1.ErrBadData:
		case v1.ErrTimeout:
			return (delta / 4).Round(time.Minute), true
		case v1.ErrCanceled:
		case v1.ErrExec:
			if strings.Contains(apiErr.Msg, "query processing would load too many samples into memory in ") {
				return (delta / 4).Round(time.Minute), true
			}
			return delta / 2, true
		case v1.ErrBadResponse:
		case v1.ErrServer:
		case v1.ErrClient:
			return (delta / 2).Round(time.Minute), true
		}
	}

	return delta, false
}
