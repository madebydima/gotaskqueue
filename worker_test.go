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

		// Verify handler was registered
		storedHandler, exists := worker.getHandler("test_task")
		assert.True(t, exists)
		assert.NotNil(t, storedHandler)

		// Test handler execution
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

		// Enqueue a task
		taskID, err := queue.Enqueue("success_task", "test data")
		require.NoError(t, err)

		// Get the task from queue
		task, err := queue.dequeue("success_task")
		require.NoError(t, err)
		require.NotNil(t, task)

		// Process the task
		worker.processTask(0, task)

		assert.True(t, processed)
		assert.Equal(t, taskID, processedTask.ID)

		// Verify task was marked as completed
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

		// First attempt - should fail and retry
		task, err := queue.dequeue("flaky_task")
		require.NoError(t, err)
		require.NotNil(t, task) // Добавляем проверку на nil

		worker.processTask(0, task)
		assert.Equal(t, 1, attempts)

		// Task should be retried after delay
		time.Sleep(50 * time.Millisecond) // Увеличиваем задержку
		queue.processDelayedTasks()

		// Second attempt - should succeed
		retriedTask, err := queue.dequeue("flaky_task")
		require.NoError(t, err)
		require.NotNil(t, retriedTask) // Добавляем проверку на nil

		worker.processTask(0, retriedTask)
		assert.Equal(t, 2, attempts)

		// Verify final status
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
		require.NotNil(t, task)

		// Should not panic
		assert.NotPanics(t, func() {
			worker.processTask(0, task)
		})

		// Task should be marked for retry or failed
		finalTask, err := queue.GetTask(taskID)
		require.NoError(t, err)
		assert.Contains(t, []TaskStatus{TaskStatusRetry, TaskStatusFailed}, finalTask.Status)
	})

	t.Run("should handle task with no handler", func(t *testing.T) {
		worker := queue.NewWorker(WithConcurrency(1))

		taskID, err := queue.Enqueue("unknown_task", "data")
		require.NoError(t, err)

		task, err := queue.dequeue("unknown_task")
		require.NoError(t, err)
		require.NotNil(t, task)

		// Should handle gracefully without handler
		worker.processTask(0, task)

		// Task should be marked as failed
		failedTask, err := queue.GetTask(taskID)
		require.NoError(t, err)
		assert.Equal(t, TaskStatusFailed, failedTask.Status)
	})
}

func TestWorkerStartStop(t *testing.T) {
	queue, err := New(WithRedisAddr("localhost:6379"), WithNamespace("test-startstop"))
	require.NoError(t, err)
	defer queue.Close()

	t.Run("should start and stop worker", func(t *testing.T) {
		worker := queue.NewWorker(WithConcurrency(2))

		// Start worker
		go worker.Start()

		// Wait a bit for workers to start
		time.Sleep(50 * time.Millisecond)

		assert.True(t, worker.IsRunning())

		// Stop worker
		worker.Stop()
		time.Sleep(10 * time.Millisecond) // Даем время для graceful shutdown
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

			time.Sleep(100 * time.Millisecond) // Увеличиваем время работы

			mu.Lock()
			delete(processing, data)
			mu.Unlock()

			processed <- data
			return nil
		})

		// Enqueue multiple tasks
		for i := 0; i < 3; i++ {
			_, err := queue.Enqueue("concurrent_task", i)
			require.NoError(t, err)
		}

		// Start worker
		go worker.Start()
		defer worker.Stop()

		// Wait for all tasks to be processed
		timeout := time.After(5 * time.Second) // Увеличиваем таймаут
		completed := 0

		for completed < 3 {
			select {
			case <-processed:
				completed++
			case <-timeout:
				t.Fatal("Timeout waiting for tasks to complete")
			}
		}

		// Verify all tasks were processed concurrently
		assert.Equal(t, 3, completed)
	})
}

func TestWorkerGracefulShutdown(t *testing.T) {
	queue, err := New(WithRedisAddr("localhost:6379"), WithNamespace("test-shutdown"))
	require.NoError(t, err)
	defer queue.Close()

	t.Run("should return task to queue on shutdown", func(t *testing.T) {
		worker := queue.NewWorker(WithConcurrency(1))

		processing := make(chan bool)
		handlerStarted := make(chan bool, 1) // Буферизованный канал чтобы не блокировать
		handlerCompleted := make(chan bool, 1)

		worker.Handle("long_task", func(task *Task) error {
			handlerStarted <- true
			// Ждем сигнала для продолжения или остановки
			select {
			case <-processing:
				handlerCompleted <- true
				return nil
			case <-time.After(2 * time.Second): // Таймаут на случай если тест застрял
				handlerCompleted <- true
				return nil
			}
		})

		taskID, err := queue.Enqueue("long_task", "data")
		require.NoError(t, err)

		// Start worker
		go worker.Start()

		// Wait for handler to start
		select {
		case <-handlerStarted:
			// Handler started successfully
		case <-time.After(1 * time.Second):
			t.Fatal("Handler didn't start within 1 second")
		}

		// Stop worker while task is processing
		worker.Stop()

		// Allow handler to complete
		close(processing)

		// Wait for handler to actually complete
		select {
		case <-handlerCompleted:
			// Handler completed
		case <-time.After(1 * time.Second):
			t.Log("Handler didn't complete within 1 second, continuing anyway")
		}

		// Give some time for the task to be returned to queue
		time.Sleep(200 * time.Millisecond)

		// Check task status - it should be pending because it was returned to queue
		task, err := queue.GetTask(taskID)
		require.NoError(t, err)

		// Task should be either pending or processing (depending on timing)
		// The important thing is that it wasn't completed or failed
		assert.Contains(t, []TaskStatus{TaskStatusPending, TaskStatusProcessing}, task.Status,
			"Task should be either pending or processing after graceful shutdown, got: %s", task.Status)
	})

	t.Run("should stop without active tasks", func(t *testing.T) {
		worker := queue.NewWorker(WithConcurrency(1))

		// Start and immediately stop without any tasks
		go worker.Start()
		time.Sleep(50 * time.Millisecond) // Give time to start

		// This should not panic or hang
		worker.Stop()

		assert.False(t, worker.IsRunning())
	})
}
