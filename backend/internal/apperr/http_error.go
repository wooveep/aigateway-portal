package apperr

import (
	"errors"
)

type HTTPError struct {
	Status  int
	Message string
	Detail  string
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return "http error"
}

func New(status int, message string, detail ...string) error {
	err := &HTTPError{
		Status:  status,
		Message: message,
	}
	if len(detail) > 0 {
		err.Detail = detail[0]
	}
	return err
}

func As(err error) (*HTTPError, bool) {
	if err == nil {
		return nil, false
	}
	var target *HTTPError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
