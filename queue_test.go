package gotaskqueue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewQueue(t *testing.T) {
	t.Run("should create queue with default options", func(t *testing.T) {
		queue, err := New(WithRedisAddr("localhost:6379"))
		require.NoError(t, err)
		require.NotNil(t, queue)
		defer queue.Close()

		assert.NoError(t, queue.HealthCheck())
	})

	t.Run("should fail with invalid redis address", func(t *testing.T) {
		queue, err := New(WithRedisAddr("invalid-host:9999"))
		assert.Error(t, err)
		assert.Nil(t, queue)
		assert.True(t, errors.Is(err, ErrRedisConnection))
	})

	t.Run("should create queue with custom namespace", func(t *testing.T) {
		queue, err := New(
			WithRedisAddr("localhost:6379"),
			WithNamespace("test-namespace"),
		)
		require.NoError(t, err)
		defer queue.Close()

		assert.Equal(t, "test-namespace", queue.namespace)
	})
}

func TestQueueEnqueue(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-enqueue"),
	)
	require.NoError(t, err)
	defer queue.Close()

	t.Run("should enqueue task successfully", func(t *testing.T) {
		taskData := map[string]string{"action": "process"}
		taskID, err := queue.Enqueue("test_task", taskData)
		require.NoError(t, err)
		assert.NotEmpty(t, taskID)

		task, err := queue.GetTask(taskID)
		require.NoError(t, err)
		assert.Equal(t, taskID, task.ID)
		assert.Equal(t, "test_task", task.Type)
		assert.Equal(t, TaskStatusPending, task.Status)
	})

	t.Run("should enqueue task with custom retries", func(t *testing.T) {
		taskID, err := queue.EnqueueWithRetry("retry_task", "data", 5)
		require.NoError(t, err)

		task, err := queue.GetTask(taskID)
		require.NoError(t, err)
		assert.Equal(t, 5, task.MaxRetries)
	})

	t.Run("should enqueue delayed task", func(t *testing.T) {
		start := time.Now()
		taskID, err := queue.EnqueueDelayed("delayed_task", "data", 100*time.Millisecond)
		require.NoError(t, err)

		task, err := queue.dequeue("delayed_task")
		assert.NoError(t, err)
		assert.Nil(t, task)

		time.Sleep(150 * time.Millisecond)

		queue.processDelayedTasks()

		task, err = queue.dequeue("delayed_task")
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, taskID, task.ID)
		assert.WithinDuration(t, start, time.Now(), 200*time.Millisecond)
	})

	t.Run("should fail to enqueue when queue is stopped", func(t *testing.T) {
		queue.Close()
		taskID, err := queue.Enqueue("test", "data")
		assert.Error(t, err)
		assert.Empty(t, taskID)
		assert.Equal(t, ErrQueueStopped, err)
	})
}

func TestQueueDequeue(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-dequeue"),
	)
	require.NoError(t, err)
	defer queue.Close()

	t.Run("should dequeue tasks in order", func(t *testing.T) {
		task1ID, err := queue.Enqueue("test_task", "data1")
		require.NoError(t, err)
		task2ID, err := queue.Enqueue("test_task", "data2")
		require.NoError(t, err)

		task1, err := queue.dequeue("test_task")
		require.NoError(t, err)
		require.NotNil(t, task1)
		assert.Equal(t, task1ID, task1.ID)
		assert.Equal(t, TaskStatusProcessing, task1.Status)

		task2, err := queue.dequeue("test_task")
		require.NoError(t, err)
		require.NotNil(t, task2)
		assert.Equal(t, task2ID, task2.ID)
	})

	t.Run("should return nil when queue is empty", func(t *testing.T) {
		task, err := queue.dequeue("non_existent_type")
		require.NoError(t, err)
		assert.Nil(t, task)
	})
}

func TestQueueRetryLogic(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-retry"),
		WithMaxRetries(2),
		WithRetryDelay(200*time.Millisecond),
	)
	require.NoError(t, err)
	defer queue.Close()

	t.Run("should retry task when it fails", func(t *testing.T) {
		taskID, err := queue.Enqueue("flaky_task", "data")
		require.NoError(t, err)

		task, err := queue.dequeue("flaky_task")
		require.NoError(t, err)
		require.NotNil(t, task)

		err = queue.retryTask(task, errors.New("simulated failure"))
		require.NoError(t, err)

		time.Sleep(300 * time.Millisecond)
		queue.processDelayedTasks()

		retriedTask, err := queue.dequeue("flaky_task")
		require.NoError(t, err)
		require.NotNil(t, retriedTask)
		assert.Equal(t, taskID, retriedTask.ID)
		assert.Equal(t, 1, retriedTask.Retries)
		assert.Equal(t, TaskStatusProcessing, retriedTask.Status)
	})

	t.Run("should fail task when max retries exceeded", func(t *testing.T) {
		task := &Task{
			ID:         "test-task",
			Type:       "test",
			Retries:    2,
			MaxRetries: 2,
			Status:     TaskStatusProcessing,
		}

		err := queue.retryTask(task, errors.New("final failure"))
		require.NoError(t, err)

		failedTask, err := queue.GetTask(task.ID)
		require.NoError(t, err)
		assert.Equal(t, TaskStatusFailed, failedTask.Status)
		assert.Equal(t, ErrMaxRetriesExceeded.Error(), failedTask.Error)
	})
}

func TestQueueStats(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-stats"),
	)
	require.NoError(t, err)
	defer queue.Close()

	ctx := context.Background()
	queue.client.Del(ctx, queue.key("tasks"))

	t.Run("should return accurate statistics", func(t *testing.T) {
		stats, err := queue.GetStats()
		require.NoError(t, err)
		assert.Equal(t, int64(0), stats.Total)

		task1, _ := NewTask("type1", "data1", 3)
		task1.Status = TaskStatusPending
		task1Data, _ := task1.Marshal()
		queue.client.HSet(ctx, queue.key("tasks"), task1.ID, task1Data)

		task2, _ := NewTask("type2", "data2", 3)
		task2.Status = TaskStatusCompleted
		task2Data, _ := task2.Marshal()
		queue.client.HSet(ctx, queue.key("tasks"), task2.ID, task2Data)

		task3, _ := NewTask("type3", "data3", 3)
		task3.Status = TaskStatusFailed
		task3Data, _ := task3.Marshal()
		queue.client.HSet(ctx, queue.key("tasks"), task3.ID, task3Data)

		stats, err = queue.GetStats()
		require.NoError(t, err)
		assert.Equal(t, int64(3), stats.Total)
		assert.Equal(t, int64(1), stats.Pending)
		assert.Equal(t, int64(0), stats.Processing)
		assert.Equal(t, int64(1), stats.Completed)
		assert.Equal(t, int64(1), stats.Failed)
	})
}

func TestQueueCleanup(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-cleanup"),
		WithTaskTTL(50*time.Millisecond),
		WithMaxMemoryTasks(2),
	)
	require.NoError(t, err)
	defer queue.Close()

	ctx := context.Background()

	t.Run("should cleanup old tasks", func(t *testing.T) {
		oldTask, _ := NewTask("old_task", "data", 1)
		oldTask.Status = TaskStatusCompleted
		oldTask.CreatedAt = time.Now().Add(-time.Hour)
		oldTaskData, _ := oldTask.Marshal()
		queue.client.HSet(ctx, queue.key("tasks"), oldTask.ID, oldTaskData)

		recentTask, _ := NewTask("recent_task", "data", 1)
		recentTask.Status = TaskStatusCompleted
		recentTaskData, _ := recentTask.Marshal()
		queue.client.HSet(ctx, queue.key("tasks"), recentTask.ID, recentTaskData)

		queue.cleanupOldTasks()

		exists, err := queue.client.HExists(ctx, queue.key("tasks"), oldTask.ID).Result()
		require.NoError(t, err)
		assert.False(t, exists)

		exists, err = queue.client.HExists(ctx, queue.key("tasks"), recentTask.ID).Result()
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("should respect max memory limit", func(t *testing.T) {
		queue.client.Del(ctx, queue.key("tasks"))

		for i := range 5 {
			task, _ := NewTask("test", i, 1)
			task.Status = TaskStatusCompleted
			taskData, _ := task.Marshal()
			queue.client.HSet(ctx, queue.key("tasks"), task.ID, taskData)
		}

		queue.cleanupExcessTasks([]string{
			"task1", "task2", "task3", "task4", "task5",
		})

		count, err := queue.client.HLen(ctx, queue.key("tasks")).Result()
		require.NoError(t, err)
		assert.LessOrEqual(t, count, int64(2))
	})
}

func TestQueueDelayedTasks(t *testing.T) {
	queue, err := New(
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-delayed-queue"),
	)
	require.NoError(t, err)
	defer queue.Close()

	t.Run("should process delayed tasks after specified time", func(t *testing.T) {
		start := time.Now()
		delay := 200 * time.Millisecond

		taskID, err := queue.EnqueueDelayed("delayed_task", "data", delay)
		require.NoError(t, err)

		task, err := queue.dequeue("delayed_task")
		require.NoError(t, err)
		assert.Nil(t, task)

		time.Sleep(delay + 50*time.Millisecond)

		queue.processDelayedTasks()

		task, err = queue.dequeue("delayed_task")
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, taskID, task.ID)
		assert.WithinDuration(t, start.Add(delay), time.Now(), 100*time.Millisecond)
	})
}
