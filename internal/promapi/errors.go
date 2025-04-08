package promapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"syscall"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prymitive/current"
)

func isUnsupportedError(err error) bool {
	var e1 APIError
	if ok := errors.As(err, &e1); ok {
		return e1.ErrorType == ErrAPIUnsupported
	}
	return false
}

func IsUnavailableError(err error) bool {
	var e1 APIError
	if ok := errors.As(err, &e1); ok {
		return e1.ErrorType == v1.ErrServer
	}
	return true
}

func IsQueryTooExpensive(err error) bool {
	var e1 APIError
	if ok := errors.As(err, &e1); ok {
		if e1.ErrorType != v1.ErrExec {
			return false
		}
		if strings.HasPrefix(
			e1.Err,
			"query processing would load too many samples into memory in ",
		) {
			return true
		}
		if strings.HasSuffix(e1.Err, "expanding series: context deadline exceeded") {
			return true
		}
	}
	return false
}

type APIError struct {
	Status    string       `json:"status"`
	ErrorType v1.ErrorType `json:"errorType"`
	Err       string       `json:"error"`
}

func (e APIError) Error() string {
	return e.Err
}

const (
	ErrUnknown        v1.ErrorType = "unknown"
	ErrJSONStream     v1.ErrorType = "json_stream"
	ErrAPIUnsupported v1.ErrorType = "unsupported"
)

func decodeErrorType(s string) v1.ErrorType {
	switch s {
	case string(v1.ErrBadData):
		return v1.ErrBadData
	case string(v1.ErrTimeout):
		return v1.ErrTimeout
	case string(v1.ErrCanceled):
		return v1.ErrCanceled
	case string(v1.ErrExec):
		return v1.ErrExec
	case string(v1.ErrBadResponse):
		return v1.ErrBadResponse
	case string(v1.ErrServer):
		return v1.ErrServer
	case string(v1.ErrClient):
		return v1.ErrClient
	default:
		return ErrUnknown
	}
}

const (
	errConnRefused = "connection refused"
	errConnTimeout = "connection timeout"
)

func decodeError(err error) string {
	if errors.Is(err, context.Canceled) {
		return context.Canceled.Error()
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return errConnRefused
	}

	var neterr net.Error
	if ok := errors.As(err, &neterr); ok && neterr.Timeout() {
		return errConnTimeout
	}

	var e1 APIError
	if ok := errors.As(err, &e1); ok {
		return fmt.Sprintf("%s: %s", e1.ErrorType, e1.Err)
	}

	return err.Error()
}

func tryDecodingAPIError(resp *http.Response) error {
	slog.Debug("Trying to parse Prometheus error response", slog.Int("code", resp.StatusCode))

	if resp.StatusCode == http.StatusNotFound {
		var apiPath string
		msg := "some API endpoints"
		if resp.Request != nil {
			switch {
			case strings.HasSuffix(resp.Request.URL.Path, APIPathConfig):
				apiPath = APIPathConfig
			case strings.HasSuffix(resp.Request.URL.Path, APIPathFlags):
				apiPath = APIPathFlags
			case strings.HasSuffix(resp.Request.URL.Path, APIPathMetadata):
				apiPath = APIPathMetadata
			}
			msg = "`" + apiPath + "` API endpoint"
		}
		if apiPath != "" {
			return APIError{
				Status:    "",
				ErrorType: ErrAPIUnsupported,
				Err:       "this server doesn't seem to support " + msg,
			}
		}
	}

	var status, errType, errText string
	decoder := current.Object(
		current.Key("status", current.Value(func(s string, _ bool) {
			status = s
		})),
		current.Key("error", current.Value(func(s string, _ bool) {
			errText = s
		})),
		current.Key("errorType", current.Value(func(s string, _ bool) {
			errType = s
		})),
	)

	dec := json.NewDecoder(resp.Body)
	if err := decoder.Stream(dec); err != nil {
		switch resp.StatusCode / 100 {
		case 4:
			return APIError{Status: "error", ErrorType: v1.ErrClient, Err: resp.Status}
		case 5:
			return APIError{Status: "error", ErrorType: v1.ErrServer, Err: resp.Status}
		}
		return APIError{Status: "error", ErrorType: v1.ErrBadResponse, Err: resp.Status}
	}

	return APIError{Status: status, ErrorType: decodeErrorType(errType), Err: errText}
}
