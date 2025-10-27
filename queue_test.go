package gotaskqueue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQueue(t *testing.T) {
	queue, err := New(WithRedisAddr("localhost:6379"))
	require.NoError(t, err)
	assert.NotNil(t, queue)
	defer queue.Close()

	assert.NoError(t, queue.HealthCheck())
}

func TestEnqueueAndProcess(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test"),
	)
	require.NoError(t, err)
	defer queue.Close()

	// Создаем воркер
	worker := queue.NewWorker(WithConcurrency(1))

	processed := make(chan bool, 1)

	// Регистрируем обработчик
	worker.Handle("test_task", func(task *Task) error {
		var data map[string]string
		task.UnmarshalData(&data)
		assert.Equal(t, "value", data["key"])
		processed <- true
		return nil
	})

	go worker.Start()
	defer worker.Stop()

	// Добавляем задачу
	taskID, err := queue.Enqueue("test_task", map[string]string{"key": "value"})
	require.NoError(t, err)
	assert.NotEmpty(t, taskID)

	// Ждем обработки
	select {
	case <-processed:
		// Успех
	case <-time.After(5 * time.Second):
		t.Fatal("Task wasn't processed in time")
	}

	// Проверяем статус задачи
	task, err := queue.GetTask(taskID)
	require.NoError(t, err)
	assert.Equal(t, TaskStatusCompleted, task.Status)
}

func TestRetryMechanism(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test_retry"),
		WithMaxRetries(2),
	)
	require.NoError(t, err)
	defer queue.Close()

	worker := queue.NewWorker(WithConcurrency(1))

	attempts := 0
	worker.Handle("flaky_task", func(task *Task) error {
		attempts++
		if attempts < 3 {
			return assert.AnError
		}
		return nil
	})

	go worker.Start()
	defer worker.Stop()

	taskID, err := queue.Enqueue("flaky_task", nil)
	require.NoError(t, err)

	// Ждем завершения всех попыток
	assert.NoError(t, queue.WaitForTasks(10*time.Second))

	task, err := queue.GetTask(taskID)
	require.NoError(t, err)
	assert.Equal(t, TaskStatusCompleted, task.Status)
	assert.Equal(t, 2, task.Retries) // 2 неудачные попытки + 1 успешная
}

func TestDelayedTask(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test_delayed"),
	)
	require.NoError(t, err)
	defer queue.Close()

	worker := queue.NewWorker(WithConcurrency(1))

	processed := make(chan time.Time, 1)
	startTime := time.Now()

	worker.Handle("delayed_task", func(task *Task) error {
		processed <- time.Now()
		return nil
	})

	go worker.Start()
	defer worker.Stop()

	// Добавляем задачу с задержкой 2 секунды
	_, err = queue.EnqueueDelayed("delayed_task", nil, 2*time.Second)
	require.NoError(t, err)

	select {
	case processedTime := <-processed:
		// Проверяем что прошло至少 2 секунды
		assert.True(t, processedTime.Sub(startTime) >= 2*time.Second)
	case <-time.After(5 * time.Second):
		t.Fatal("Delayed task wasn't processed in time")
	}
}
