package notifier

import (
	"fmt"
)

const (
	// TypeContextCanceled used by NotifyErr when context has been canceled.
	TypeContextCanceled = iota
	// TypeWorkersLimitExceeded used by NotifyErr when workers limit exceeded.
	TypeWorkersLimitExceeded
	// TypeSendError used by NotifyErr when unable to send a message.
	TypeSendError
)

const (
	msgSendErrorRequest     = "Fail send message, unable to create request"
	msgSendErrorRateLimiter = "Fail send message, rate limiter error"
	msgSendErrorClient      = "Fail send message, unable to do request"
)

// NotifyErr custom error used by the Client.
type NotifyErr struct {
	Type    int
	Message string
	Err     error
}

// Error implements error interface.
func (e *NotifyErr) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

// Unwrap unwraps underlying error.
func (e *NotifyErr) Unwrap() error { return e.Err }

// Is implements error equity check.
func (e *NotifyErr) Is(target error) bool {
	t, ok := target.(*NotifyErr)
	if !ok {
		return false
	}
	return e.Type == t.Type
}
