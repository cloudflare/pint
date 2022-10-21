package promapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"

	"github.com/go-json-experiment/json"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func IsUnavailableError(err error) bool {
	var e1 APIError
	if ok := errors.As(err, &e1); ok {
		return e1.ErrorType == v1.ErrServer
	}

	return true
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
	ErrUnknown    v1.ErrorType = "unknown"
	ErrJSONStream v1.ErrorType = "json_stream"
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

	var e1 APIError
	if ok := errors.As(err, &e1); ok {
		return fmt.Sprintf("%s: %s", e1.ErrorType, e1.Err)
	}

	return err.Error()
}

type FailedResponse struct {
	Status    string `json:"status"`
	Error     string `json:"error"`
	ErrorType string `json:"errorType"`
}

func tryDecodingAPIError(resp *http.Response) error {
	var decoded FailedResponse
	if json.UnmarshalFull(resp.Body, &decoded) == nil {
		return APIError{Status: decoded.Status, ErrorType: decodeErrorType(decoded.ErrorType), Err: decoded.Error}
	}

	switch resp.StatusCode / 100 {
	case 4:
		return APIError{Status: "error", ErrorType: v1.ErrClient, Err: fmt.Sprintf("client error: %d", resp.StatusCode)}
	case 5:
		return APIError{Status: "error", ErrorType: v1.ErrServer, Err: fmt.Sprintf("server error: %d", resp.StatusCode)}
	}
	return APIError{Status: "error", ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("bad response code: %d", resp.StatusCode)}
}
