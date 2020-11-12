package notifier

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotifyErr_Error(t *testing.T) {
	err := &NotifyErr{
		Type:    TypeContextCanceled,
		Message: "test error message",
		Err:     errors.New("test err"),
	}

	assert.Equal(t, "test error message: test err", err.Error())
}

func TestNotifyErr_Unwrap(t *testing.T) {
	original := errors.New("test err")
	err := &NotifyErr{
		Type:    TypeContextCanceled,
		Message: "test error message",
		Err:     original,
	}

	assert.True(t, errors.Is(err, original))
}

func TestNotifyErr_Is(t *testing.T) {
	err := &NotifyErr{
		Type:    TypeContextCanceled,
		Message: "test error message",
		Err:     nil,
	}
	assert.True(t, errors.Is(err, &NotifyErr{Type: TypeContextCanceled}))
}
