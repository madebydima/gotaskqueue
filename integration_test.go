package gotaskqueue

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_CompleteWorkflow(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-integration"),
		WithMaxRetries(2),
		WithRetryDelay(100*time.Millisecond),
	)
	require.NoError(t, err)
	defer queue.Close()

	worker := queue.NewWorker(
		WithConcurrency(2),
		WithPollInterval(50*time.Millisecond),
	)

	// Test data
	type EmailData struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
	}

	var processedTasks sync.Map
	var mu sync.Mutex

	// Register handlers
	worker.Handle("send_email", func(task *Task) error {
		var data EmailData
		task.UnmarshalData(&data)

		mu.Lock()
		processedTasks.Store(task.ID, data)
		mu.Unlock()

		return nil
	})

	worker.Handle("flaky_operation", func(task *Task) error {
		var attempt int
		task.UnmarshalData(&attempt)

		if attempt < 2 {
			return assert.AnError // Fail first two attempts
		}
		return nil
	})

	// Start worker
	go worker.Start()
	defer worker.Stop()

	t.Run("should process successful tasks", func(t *testing.T) {
		emailData := EmailData{
			To:      "user@example.com",
			Subject: "Welcome!",
		}

		taskID, err := queue.Enqueue("send_email", emailData)
		require.NoError(t, err)

		// Wait for task to be processed
		assert.Eventually(t, func() bool {
			task, err := queue.GetTask(taskID)
			return err == nil && task.Status == TaskStatusCompleted
		}, 5*time.Second, 200*time.Millisecond) // Увеличиваем таймаут

		// Verify task data was processed correctly
		val, ok := processedTasks.Load(taskID)
		assert.True(t, ok)
		assert.Equal(t, emailData, val)
	})

	t.Run("should handle retries correctly", func(t *testing.T) {
		taskID, err := queue.EnqueueWithRetry("flaky_operation", 1, 3)
		require.NoError(t, err)

		// Wait for task to eventually succeed
		assert.Eventually(t, func() bool {
			task, err := queue.GetTask(taskID)
			return err == nil && task.Status == TaskStatusCompleted
		}, 5*time.Second, 200*time.Millisecond) // Увеличиваем таймаут

		// Verify it took multiple attempts
		task, err := queue.GetTask(taskID)
		require.NoError(t, err)
		assert.Equal(t, 2, task.Retries) // Failed twice, succeeded on third
	})

	t.Run("should handle retries correctly", func(t *testing.T) {
		taskID, err := queue.EnqueueWithRetry("flaky_operation", 1, 3)
		require.NoError(t, err)

		// Wait for task to eventually succeed
		assert.Eventually(t, func() bool {
			task, err := queue.GetTask(taskID)
			return err == nil && task.Status == TaskStatusCompleted
		}, 2*time.Second, 100*time.Millisecond)

		// Verify it took multiple attempts
		task, err := queue.GetTask(taskID)
		require.NoError(t, err)
		assert.Equal(t, 2, task.Retries) // Failed twice, succeeded on third
	})

	t.Run("should handle multiple task types", func(t *testing.T) {
		task1ID, _ := queue.Enqueue("send_email", EmailData{To: "test1@example.com"})
		task2ID, _ := queue.Enqueue("send_email", EmailData{To: "test2@example.com"})

		// Wait for both tasks to complete
		assert.Eventually(t, func() bool {
			task1, _ := queue.GetTask(task1ID)
			task2, _ := queue.GetTask(task2ID)
			return task1 != nil && task1.Status == TaskStatusCompleted &&
				task2 != nil && task2.Status == TaskStatusCompleted
		}, 2*time.Second, 100*time.Millisecond)
	})
}

func TestIntegration_DelayedTasks(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-delayed-integration"),
	)
	require.NoError(t, err)
	defer queue.Close()

	worker := queue.NewWorker(WithPollInterval(50 * time.Millisecond))

	var processedAt time.Time
	worker.Handle("delayed_task", func(task *Task) error {
		processedAt = time.Now()
		return nil
	})

	go worker.Start()
	defer worker.Stop()

	t.Run("should process delayed tasks after specified time", func(t *testing.T) {
		start := time.Now()
		delay := 200 * time.Millisecond // Увеличиваем задержку

		_, err := queue.EnqueueDelayed("delayed_task", "data", delay)
		require.NoError(t, err)

		// Task should not be processed immediately
		time.Sleep(100 * time.Millisecond)
		assert.True(t, processedAt.IsZero(), "Task should not be processed yet")

		// Wait for task to be processed
		assert.Eventually(t, func() bool {
			return !processedAt.IsZero()
		}, 3*time.Second, 100*time.Millisecond) // Увеличиваем таймаут

		// Verify it was processed after the delay
		actualDelay := processedAt.Sub(start)
		assert.True(t, actualDelay >= delay,
			"Task was processed after %v, expected at least %v", actualDelay, delay)
	})
}

func TestIntegration_WorkerScaling(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-scaling"),
	)
	require.NoError(t, err)
	defer queue.Close()

	const numTasks = 10
	const numWorkers = 3

	worker := queue.NewWorker(
		WithConcurrency(numWorkers),
		WithPollInterval(10*time.Millisecond),
	)

	var processing sync.Mutex
	currentWorkers := 0
	maxConcurrent := 0
	completed := 0

	worker.Handle("concurrent_task", func(task *Task) error {
		processing.Lock()
		currentWorkers++
		if currentWorkers > maxConcurrent {
			maxConcurrent = currentWorkers
		}
		processing.Unlock()

		// Simulate work
		time.Sleep(50 * time.Millisecond)

		processing.Lock()
		currentWorkers--
		completed++
		processing.Unlock()

		return nil
	})

	// Enqueue tasks
	for i := 0; i < numTasks; i++ {
		_, err := queue.Enqueue("concurrent_task", i)
		require.NoError(t, err)
	}

	// Start worker
	go worker.Start()
	defer worker.Stop()

	// Wait for all tasks to complete
	assert.Eventually(t, func() bool {
		processing.Lock()
		defer processing.Unlock()
		return completed == numTasks
	}, 5*time.Second, 100*time.Millisecond)

	// Verify we achieved some level of concurrency
	assert.True(t, maxConcurrent > 1,
		"Should have processed tasks concurrently, max was %d", maxConcurrent)
	assert.True(t, maxConcurrent <= numWorkers,
		"Should not exceed configured concurrency limit")
}
