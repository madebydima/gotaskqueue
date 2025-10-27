package gotaskqueue

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorTypes(t *testing.T) {
	t.Run("error messages", func(t *testing.T) {
		assert.Equal(t, "task not found", ErrTaskNotFound.Error())
		assert.Equal(t, "handler not found for task type", ErrHandlerNotFound.Error())
		assert.Equal(t, "max retries exceeded", ErrMaxRetriesExceeded.Error())
		assert.Equal(t, "invalid task data", ErrInvalidTaskData.Error())
		assert.Equal(t, "queue is stopped", ErrQueueStopped.Error())
		assert.Equal(t, "redis connection error", ErrRedisConnection.Error())
	})

	t.Run("error comparison", func(t *testing.T) {
		assert.True(t, errors.Is(ErrTaskNotFound, ErrTaskNotFound))
		assert.True(t, errors.Is(ErrHandlerNotFound, ErrHandlerNotFound))

		customErr := errors.New("task not found")
		assert.False(t, errors.Is(customErr, ErrTaskNotFound))
	})
}
