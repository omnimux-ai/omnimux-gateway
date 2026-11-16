// Package errs 业务错误码与标准化错误类型。
package errs

import (
	"errors"
	"fmt"
)

type Error struct {
	Code    string
	Status  int
	Msg     string
	WrapErr error
}

func (e *Error) Error() string {
	if e.WrapErr != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.WrapErr)
	}
	return e.Msg
}

// FromErr 提取 error 为 *Error，普通 error 归为 INTERNAL。
func FromErr(err error) *Error {
	if err == nil {
		return &Error{Code: "OK", Status: 200, Msg: "ok"}
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return &Error{Code: "INTERNAL", Status: 500, Msg: err.Error()}
}

// Wrap 构造带 http 状态的业务错误。
func Wrap(code string, status int, msg string, err error) *Error {
	return &Error{Code: code, Status: status, Msg: msg, WrapErr: err}
}

// 预定义错误
var (
	ErrInvalidParam = &Error{Code: "INVALID_PARAM", Status: 400, Msg: "invalid parameter"}
	ErrUnauthorized = &Error{Code: "UNAUTHORIZED", Status: 401, Msg: "unauthorized"}
	ErrNotFound     = &Error{Code: "NOT_FOUND", Status: 404, Msg: "not found"}
	ErrDuplicateURL = &Error{Code: "DUPLICATE_URL", Status: 409, Msg: "source_url already exists"}
	ErrDuplicateTag = &Error{Code: "DUPLICATE_TAG", Status: 409, Msg: "tag already exists"}
	ErrBulkLimit    = &Error{Code: "BULK_LIMIT", Status: 409, Msg: "bulk exceeds 200 items"}
	ErrRateLimited  = &Error{Code: "RATE_LIMITED", Status: 429, Msg: "rate limited"}
	ErrDBUnavail    = &Error{Code: "DB_UNAVAILABLE", Status: 503, Msg: "database unavailable"}
)
