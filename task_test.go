package gotaskqueue

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTask(t *testing.T) {
	t.Run("should create task with valid data", func(t *testing.T) {
		taskData := map[string]interface{}{
			"email":   "test@example.com",
			"subject": "Test Subject",
		}

		task, err := NewTask("send_email", taskData, 3)
		require.NoError(t, err)
		require.NotNil(t, task)

		assert.NotEmpty(t, task.ID)
		assert.Equal(t, "send_email", task.Type)
		assert.Equal(t, 3, task.MaxRetries)
		assert.Equal(t, 0, task.Retries)
		assert.Equal(t, TaskStatusPending, task.Status)
		assert.WithinDuration(t, time.Now(), task.CreatedAt, time.Second)
	})

	t.Run("should handle nil data", func(t *testing.T) {
		task, err := NewTask("cleanup", nil, 1)
		require.NoError(t, err)
		assert.Empty(t, task.Data)
	})

	t.Run("should return error for invalid data", func(t *testing.T) {
		// Create data that cannot be marshaled to JSON
		invalidData := make(chan int)

		task, err := NewTask("invalid", invalidData, 1)
		assert.Error(t, err)
		assert.Nil(t, task)
		assert.Equal(t, ErrInvalidTaskData, err)
	})
}

func TestTaskMarshalUnmarshal(t *testing.T) {
	taskData := map[string]string{"key": "value"}
	task, err := NewTask("test", taskData, 2)
	require.NoError(t, err)

	// Test Marshal
	data, err := task.Marshal()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test Unmarshal
	unmarshaledTask, err := UnmarshalTask(data)
	require.NoError(t, err)
	assert.Equal(t, task.ID, unmarshaledTask.ID)
	assert.Equal(t, task.Type, unmarshaledTask.Type)
	assert.Equal(t, task.MaxRetries, unmarshaledTask.MaxRetries)
	assert.Equal(t, task.Status, unmarshaledTask.Status)
}

func TestTaskUnmarshalData(t *testing.T) {
	type EmailData struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
	}

	originalData := EmailData{
		To:      "user@example.com",
		Subject: "Welcome",
	}

	task, err := NewTask("send_email", originalData, 3)
	require.NoError(t, err)

	// Test UnmarshalData
	var unmarshaledData EmailData
	err = task.UnmarshalData(&unmarshaledData)
	require.NoError(t, err)
	assert.Equal(t, originalData.To, unmarshaledData.To)
	assert.Equal(t, originalData.Subject, unmarshaledData.Subject)
}

func TestTaskStatusTransitions(t *testing.T) {
	task := &Task{
		ID:         "test-task",
		Type:       "test",
		Retries:    0,
		MaxRetries: 3,
		Status:     TaskStatusPending,
	}

	t.Run("mark processing", func(t *testing.T) {
		task.MarkProcessing()
		assert.Equal(t, TaskStatusProcessing, task.Status)
		assert.NotNil(t, task.ProcessedAt)
	})

	t.Run("mark completed", func(t *testing.T) {
		task.MarkCompleted()
		assert.Equal(t, TaskStatusCompleted, task.Status)
	})

	t.Run("mark failed", func(t *testing.T) {
		task.MarkFailed(assert.AnError)
		assert.Equal(t, TaskStatusFailed, task.Status)
		assert.Equal(t, assert.AnError.Error(), task.Error)
	})

	t.Run("mark for retry", func(t *testing.T) {
		task.Status = TaskStatusPending
		task.Retries = 0
		task.MarkForRetry(assert.AnError)
		assert.Equal(t, TaskStatusRetry, task.Status)
		assert.Equal(t, 1, task.Retries)
		assert.Equal(t, assert.AnError.Error(), task.Error)
	})
}

func TestTaskShouldRetry(t *testing.T) {
	tests := []struct {
		name       string
		retries    int
		maxRetries int
		expected   bool
	}{
		{"should retry when under limit", 0, 3, true},
		{"should retry when at limit but less than max", 2, 3, true},
		{"should not retry when at max", 3, 3, false},
		{"should not retry when over max", 4, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Retries:    tt.retries,
				MaxRetries: tt.maxRetries,
			}
			assert.Equal(t, tt.expected, task.ShouldRetry())
		})
	}
}

func TestTaskJSONCompatibility(t *testing.T) {
	task := &Task{
		ID:         "test-id",
		Type:       "test-type",
		Data:       []byte(`{"key":"value"}`),
		CreatedAt:  time.Now().UTC().Truncate(time.Second), // Truncate for JSON precision
		Retries:    1,
		MaxRetries: 3,
		Status:     TaskStatusProcessing,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(task)
	require.NoError(t, err)

	// Unmarshal from JSON
	var unmarshaledTask Task
	err = json.Unmarshal(jsonData, &unmarshaledTask)
	require.NoError(t, err)

	// Compare fields
	assert.Equal(t, task.ID, unmarshaledTask.ID)
	assert.Equal(t, task.Type, unmarshaledTask.Type)
	assert.Equal(t, task.Data, unmarshaledTask.Data)
	assert.Equal(t, task.Retries, unmarshaledTask.Retries)
	assert.Equal(t, task.MaxRetries, unmarshaledTask.MaxRetries)
	assert.Equal(t, task.Status, unmarshaledTask.Status)
	assert.WithinDuration(t, task.CreatedAt, unmarshaledTask.CreatedAt, time.Second)
}
