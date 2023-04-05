package promapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"syscall"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prymitive/current"
	"github.com/rs/zerolog/log"
)

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
		if strings.HasPrefix(e1.Err, "query processing would load too many samples into memory in ") {
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
	log.Debug().Int("code", resp.StatusCode).Msg("Trying to parse Prometheus error response")

	var status, errType, errText string
	decoder := current.Object(
		current.Key("status", current.Value(func(s string, isNil bool) {
			status = s
		})),
		current.Key("error", current.Value(func(s string, isNil bool) {
			errText = s
		})),
		current.Key("errorType", current.Value(func(s string, isNil bool) {
			errType = s
		})),
	)

	dec := json.NewDecoder(resp.Body)
	if err := decoder.Stream(dec); err != nil {
		switch resp.StatusCode / 100 {
		case 4:
			return APIError{Status: "error", ErrorType: v1.ErrClient, Err: fmt.Sprintf("client error: %d", resp.StatusCode)}
		case 5:
			return APIError{Status: "error", ErrorType: v1.ErrServer, Err: fmt.Sprintf("server error: %d", resp.StatusCode)}
		}
		return APIError{Status: "error", ErrorType: v1.ErrBadResponse, Err: fmt.Sprintf("bad response code: %d", resp.StatusCode)}
	}

	return APIError{Status: status, ErrorType: decodeErrorType(errType), Err: errText}
}
