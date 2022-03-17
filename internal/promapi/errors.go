package promapi

import (
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
		switch apiErr.Type {
		case v1.ErrBadData:
		case v1.ErrTimeout:
			return (delta / 2).Round(time.Minute), true
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
