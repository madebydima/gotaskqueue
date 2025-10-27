package gotaskqueue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Data        []byte     `json:"data"`
	CreatedAt   time.Time  `json:"created_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	Retries     int        `json:"retries"`
	MaxRetries  int        `json:"max_retries"`
	Status      TaskStatus `json:"status"`
	Error       string     `json:"error,omitempty"`
}

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusRetry      TaskStatus = "retry"
)

func NewTask(taskType string, data any, maxRetries int) (*Task, error) {
	var dataBytes []byte
	var err error

	if data != nil {
		dataBytes, err = json.Marshal(data)
		if err != nil {
			return nil, ErrInvalidTaskData
		}
	}

	return &Task{
		ID:         uuid.New().String(),
		Type:       taskType,
		Data:       dataBytes,
		CreatedAt:  time.Now(),
		Retries:    0,
		MaxRetries: maxRetries,
		Status:     TaskStatusPending,
	}, nil
}

func (t *Task) UnmarshalData(v any) error {
	if len(t.Data) == 0 {
		return nil
	}
	return json.Unmarshal(t.Data, v)
}

func (t *Task) Marshal() ([]byte, error) {
	return json.Marshal(t)
}

func UnmarshalTask(data []byte) (*Task, error) {
	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (t *Task) ShouldRetry() bool {
	return t.Retries < t.MaxRetries
}

func (t *Task) MarkProcessing() {
	now := time.Now()
	t.ProcessedAt = &now
	t.Status = TaskStatusProcessing
}

func (t *Task) MarkCompleted() {
	t.Status = TaskStatusCompleted
}

func (t *Task) MarkFailed(err error) {
	t.Status = TaskStatusFailed
	if err != nil {
		t.Error = err.Error()
	}
}

func (t *Task) MarkForRetry(err error) {
	t.Retries++
	t.Status = TaskStatusRetry
	if err != nil {
		t.Error = err.Error()
	}
}
