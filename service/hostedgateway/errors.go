package hostedgateway

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrGatewayUnavailable = errors.New("hostedgateway: unavailable")
	ErrUnauthorized       = errors.New("hostedgateway: unauthorized")
	ErrInvalidRequest     = errors.New("hostedgateway: invalid request")
	ErrDeviceNotMonthly   = errors.New("hostedgateway: device not monthly")
	ErrJobsRequired       = errors.New("hostedgateway: jobs required")
	ErrDeviceBusy         = errors.New("hostedgateway: device busy")
	ErrTimeout            = errors.New("hostedgateway: timeout")
	ErrUpstream           = errors.New("hostedgateway: upstream")
)

// APIError is a mapped gateway HTTP failure. Status and Code are stable
// for callers; Body is truncated and must never be logged with secrets.
type APIError struct {
	Status  int
	Code    string
	Message string
	PhoneID string
	Body    string
	err     error
}

func (e *APIError) Error() string {
	if e == nil {
		return "hostedgateway: error"
	}
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("hostedgateway: HTTP %d %s: %s", e.Status, e.Code, e.Message)
	}
	if e.Message != "" {
		return fmt.Sprintf("hostedgateway: HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("hostedgateway: HTTP %d", e.Status)
}

func (e *APIError) Unwrap() error {
	if e == nil || e.err == nil {
		return nil
	}
	return e.err
}

func wrapAPI(status int, code, message, phoneID, body string, sentinel error) *APIError {
	return &APIError{
		Status:  status,
		Code:    strings.TrimSpace(code),
		Message: strings.TrimSpace(message),
		PhoneID: strings.TrimSpace(phoneID),
		Body:    truncateBody(body),
		err:     sentinel,
	}
}

func truncateBody(s string) string {
	const max = 400
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max]
}
