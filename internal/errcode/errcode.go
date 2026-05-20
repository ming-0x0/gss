package errcode

import "errors"

type ErrorCode int

//go:generate go tool stringer -type=ErrorCode
const (
	OK ErrorCode = iota
	Canceled
	Unknown
	InvalidArgument
	DeadlineExceeded
	NotFound
	AlreadyExists
	PermissionDenied
	ResourceExhausted
	FailedPrecondition
	Aborted
	OutOfRange
	Unimplemented
	Internal
	Unavailable
	DataLoss
	Unauthenticated
)

func (e ErrorCode) Int() int {
	return int(e)
}

func (e ErrorCode) IsServerError() bool {
	switch e {
	case Internal, Unknown, Unavailable:
		return true
	default:
		return false
	}
}

type Error struct {
	code ErrorCode
	msg  string
	err  error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	if e.err != nil {
		return e.err.Error()
	}

	return e.Message()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.err
}

func (e *Error) Code() ErrorCode {
	if e == nil {
		return Unknown
	}

	return e.code
}

func (e *Error) Message() string {
	if e == nil {
		return ""
	}

	if e.msg != "" {
		return e.msg
	}

	return e.code.String()
}

func Is(code ErrorCode, err error) bool {
	if err == nil {
		return false
	}

	if target, ok := errors.AsType[*Error](err); ok {
		return target.Code() == code
	}

	return false
}

func WithCause(code ErrorCode, cause error) *Error {
	return &Error{
		code: code,
		err:  cause,
	}
}

func WithMessage(code ErrorCode, msg string, cause error) *Error {
	return &Error{
		code: code,
		msg:  msg,
		err:  cause,
	}
}

func WithCode(code ErrorCode) *Error {
	return &Error{
		code: code,
	}
}

// FromError extracts the error code, message, and details from any error.
func FromError(err error) (code ErrorCode, msg string, details string) {
	if err == nil {
		return OK, OK.String(), ""
	}

	if target, ok := errors.AsType[*Error](err); ok {
		var details string
		if target.err != nil {
			details = target.err.Error()
		}

		code := target.Code()
		msg := target.Message()
		if code.IsServerError() {
			return Internal, "Internal Server Error", details
		}

		return code, msg, details
	}

	return Internal, "Internal Server Error", err.Error()
}
