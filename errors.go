package gotaskqueue

import "errors"

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrHandlerNotFound    = errors.New("handler not found for task type")
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
	ErrInvalidTaskData    = errors.New("invalid task data")
	ErrQueueStopped       = errors.New("queue is stopped")
	ErrRedisConnection    = errors.New("redis connection error")
)
