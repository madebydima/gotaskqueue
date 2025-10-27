package gotaskqueue

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWorker(t *testing.T) {
	queue, err := New(WithRedisAddr("localhost:6379"), WithNamespace("test-worker"))
	require.NoError(t, err)
	defer queue.Close()

	t.Run("should create worker with default config", func(t *testing.T) {
		worker := queue.NewWorker()
		require.NotNil(t, worker)
		assert.Equal(t, queue, worker.queue)
		assert.Equal(t, 1, worker.config.Concurrency)
		assert.Equal(t, time.Second, worker.config.PollInterval)
	})

	t.Run("should create worker with custom config", func(t *testing.T) {
		worker := queue.NewWorker(
			WithConcurrency(5),
			WithPollInterval(2*time.Second),
			WithTaskTypes("email", "report"),
		)
		require.NotNil(t, worker)
		assert.Equal(t, 5, worker.config.Concurrency)
		assert.Equal(t, 2*time.Second, worker.config.PollInterval)
		assert.Equal(t, []string{"email", "report"}, worker.config.TaskTypes)
	})
}

func TestWorkerHandle(t *testing.T) {
	queue, err := New(WithRedisAddr("localhost:6379"), WithNamespace("test-handle"))
	require.NoError(t, err)
	defer queue.Close()

	worker := queue.NewWorker()

	t.Run("should register handler for task type", func(t *testing.T) {
		var called bool
		handler := func(task *Task) error {
			called = true
			return nil
		}

		worker.Handle("test_task", handler)

		storedHandler, exists := worker.getHandler("test_task")
		assert.True(t, exists)
		assert.NotNil(t, storedHandler)

		task := &Task{Type: "test_task"}
		err := storedHandler(task)
		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("should return false for unregistered task type", func(t *testing.T) {
		handler, exists := worker.getHandler("non_existent")
		assert.False(t, exists)
		assert.Nil(t, handler)
	})
}

func TestWorkerProcessTask(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-process"),
		WithMaxRetries(2),
	)
	require.NoError(t, err)
	defer queue.Close()

	t.Run("should process task successfully", func(t *testing.T) {
		worker := queue.NewWorker(WithConcurrency(1))

		var processed bool
		var processedTask *Task

		worker.Handle("success_task", func(task *Task) error {
			processed = true
			processedTask = task
			return nil
		})

		taskID, err := queue.Enqueue("success_task", "test data")
		require.NoError(t, err)

		task, err := queue.dequeue("success_task")
		require.NoError(t, err)
		require.NotNil(t, task)

		worker.processTask(0, task)

		assert.True(t, processed)
		assert.Equal(t, taskID, processedTask.ID)

		completedTask, err := queue.GetTask(taskID)
		require.NoError(t, err)
		assert.Equal(t, TaskStatusCompleted, completedTask.Status)
	})

	t.Run("should retry task on failure", func(t *testing.T) {
		worker := queue.NewWorker(WithConcurrency(1))

		attempts := 0
		worker.Handle("flaky_task", func(task *Task) error {
			attempts++
			if attempts < 2 {
				return assert.AnError
			}
			return nil
		})

		taskID, err := queue.Enqueue("flaky_task", "data")
		require.NoError(t, err)

		task, err := queue.dequeue("flaky_task")
		require.NoError(t, err)
		worker.processTask(0, task)

		assert.Equal(t, 1, attempts)

		time.Sleep(10 * time.Millisecond)
		queue.processDelayedTasks()

		task, err = queue.dequeue("flaky_task")
		require.NoError(t, err)
		worker.processTask(0, task)

		assert.Equal(t, 2, attempts)

		completedTask, err := queue.GetTask(taskID)
		require.NoError(t, err)
		assert.Equal(t, TaskStatusCompleted, completedTask.Status)
	})

	t.Run("should handle panic in handler", func(t *testing.T) {
		worker := queue.NewWorker(WithConcurrency(1))

		worker.Handle("panic_task", func(task *Task) error {
			panic("something went wrong!")
		})

		taskID, err := queue.Enqueue("panic_task", "data")
		require.NoError(t, err)

		task, err := queue.dequeue("panic_task")
		require.NoError(t, err)

		assert.NotPanics(t, func() {
			worker.processTask(0, task)
		})

		finalTask, err := queue.GetTask(taskID)
		require.NoError(t, err)
		assert.Contains(t, []TaskStatus{TaskStatusRetry, TaskStatusFailed}, finalTask.Status)
	})

	t.Run("should handle timeout", func(t *testing.T) {
		worker := queue.NewWorker(WithConcurrency(1))

		worker.Handle("slow_task", func(task *Task) error {
			time.Sleep(2 * time.Second)
			return nil
		})

		taskID, err := queue.Enqueue("slow_task", "data")
		require.NoError(t, err)

		task, err := queue.dequeue("slow_task")
		require.NoError(t, err)

		worker.processTask(0, task)

		finalTask, _ := queue.GetTask(taskID)
		assert.NotNil(t, finalTask)
	})
}

func TestWorkerStartStop(t *testing.T) {
	queue, err := New(WithRedisAddr("localhost:6379"), WithNamespace("test-startstop"))
	require.NoError(t, err)
	defer queue.Close()

	t.Run("should start and stop worker", func(t *testing.T) {
		worker := queue.NewWorker(WithConcurrency(2))

		go worker.Start()

		time.Sleep(10 * time.Millisecond)

		assert.True(t, worker.IsRunning())

		worker.Stop()
		assert.False(t, worker.IsRunning())
	})

	t.Run("should process multiple tasks concurrently", func(t *testing.T) {
		worker := queue.NewWorker(WithConcurrency(3))

		var mu sync.Mutex
		processing := make(map[int]bool)
		processed := make(chan int, 3)

		worker.Handle("concurrent_task", func(task *Task) error {
			var data int
			task.UnmarshalData(&data)

			mu.Lock()
			processing[data] = true
			mu.Unlock()

			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			delete(processing, data)
			mu.Unlock()

			processed <- data
			return nil
		})

		for i := range 3 {
			_, err := queue.Enqueue("concurrent_task", i)
			require.NoError(t, err)
		}

		go worker.Start()
		defer worker.Stop()

		timeout := time.After(2 * time.Second)
		completed := 0

		for completed < 3 {
			select {
			case <-processed:
				completed++
			case <-timeout:
				t.Fatal("Timeout waiting for tasks to complete")
			}
		}

		assert.Equal(t, 3, completed)
	})
}
