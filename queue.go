package gotaskqueue

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type Queue struct {
	client    *redis.Client
	namespace string
	options   *QueueOptions
	ctx       context.Context
	cancel    context.CancelFunc
	stopChan  chan struct{}
	stopped   bool
	mu        sync.RWMutex
}

type QueueStats struct {
	Pending    int64 `json:"pending"`
	Processing int64 `json:"processing"`
	Completed  int64 `json:"completed"`
	Failed     int64 `json:"failed"`
	Total      int64 `json:"total"`
	Workers    int   `json:"workers"`
}

func New(options ...Option) (*Queue, error) {
	opts := DefaultOptions()
	for _, option := range options {
		option(opts)
	}

	client := redis.NewClient(&redis.Options{
		Addr:     opts.RedisAddr,
		Password: opts.RedisPassword,
		DB:       opts.RedisDB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRedisConnection, err)
	}

	mainCtx, mainCancel := context.WithCancel(context.Background())

	queue := &Queue{
		client:    client,
		namespace: opts.Namespace,
		options:   opts,
		ctx:       mainCtx,
		cancel:    mainCancel,
		stopChan:  make(chan struct{}),
		stopped:   false,
	}

	go queue.backgroundTasks()

	return queue, nil
}

func (q *Queue) key(parts ...string) string {
	allParts := append([]string{q.namespace}, parts...)
	key := ""
	for i, part := range allParts {
		if i > 0 {
			key += ":"
		}
		key += part
	}
	return key
}

func (q *Queue) Enqueue(taskType string, data any) (string, error) {
	return q.EnqueueWithOptions(taskType, data, q.options.MaxRetries, 0)
}

func (q *Queue) EnqueueWithRetry(taskType string, data any, maxRetries int) (string, error) {
	return q.EnqueueWithOptions(taskType, data, maxRetries, 0)
}

func (q *Queue) EnqueueDelayed(taskType string, data any, delay time.Duration) (string, error) {
	return q.EnqueueWithOptions(taskType, data, q.options.MaxRetries, delay)
}

func (q *Queue) dequeue(taskType string) (*Task, error) {
	if q.stopped {
		return nil, ErrQueueStopped
	}

	q.processDelayedTasks()

	result, err := q.client.BRPop(q.ctx, time.Second, q.key("queue", taskType)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	if len(result) < 2 {
		return nil, nil
	}

	taskData := result[1]
	task, err := UnmarshalTask([]byte(taskData))
	if err != nil {
		return nil, err
	}

	task.MarkProcessing()
	updatedData, _ := task.Marshal()
	q.client.HSet(q.ctx, q.key("tasks"), task.ID, updatedData)

	return task, nil
}

func (q *Queue) completeTask(task *Task) error {
	task.MarkCompleted()
	taskData, err := task.Marshal()
	if err != nil {
		return err
	}

	return q.client.HSet(q.ctx, q.key("tasks"), task.ID, taskData).Err()
}

func (q *Queue) failTask(task *Task, err error) error {
	task.MarkFailed(err)
	taskData, err := task.Marshal()
	if err != nil {
		return err
	}

	return q.client.HSet(q.ctx, q.key("tasks"), task.ID, taskData).Err()
}

func (q *Queue) GetStats() (*QueueStats, error) {
	stats := &QueueStats{}

	taskKeys, err := q.client.HKeys(q.ctx, q.key("tasks")).Result()
	if err != nil {
		return nil, err
	}

	for _, taskID := range taskKeys {
		taskData, err := q.client.HGet(q.ctx, q.key("tasks"), taskID).Result()
		if err != nil {
			continue
		}

		task, err := UnmarshalTask([]byte(taskData))
		if err != nil {
			continue
		}

		stats.Total++
		switch task.Status {
		case TaskStatusPending:
			stats.Pending++
		case TaskStatusProcessing:
			stats.Processing++
		case TaskStatusCompleted:
			stats.Completed++
		case TaskStatusFailed:
			stats.Failed++
		}
	}

	return stats, nil
}

func (q *Queue) GetTask(taskID string) (*Task, error) {
	taskData, err := q.client.HGet(q.ctx, q.key("tasks"), taskID).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	return UnmarshalTask([]byte(taskData))
}

func (q *Queue) backgroundTasks() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-q.stopChan:
			return
		case <-ticker.C:
			q.cleanupOldTasks()
			q.processDelayedTasks()
		case <-q.ctx.Done():
			return
		}
	}
}

func (q *Queue) cleanupOldTasks() {
	if q.stopped {
		return
	}

	taskKeys, err := q.client.HKeys(q.ctx, q.key("tasks")).Result()
	if err != nil {
		log.Printf("Error getting task keys for cleanup: %v", err)
		return
	}

	if len(taskKeys) > q.options.MaxMemoryTasks {
		q.cleanupExcessTasks(taskKeys)
		return
	}

	cutoff := time.Now().Add(-q.options.TaskTTL)
	for _, taskID := range taskKeys {
		taskData, err := q.client.HGet(q.ctx, q.key("tasks"), taskID).Result()
		if err != nil {
			continue
		}

		task, err := UnmarshalTask([]byte(taskData))
		if err != nil {
			continue
		}

		if (task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed) &&
			task.CreatedAt.Before(cutoff) {
			q.client.HDel(q.ctx, q.key("tasks"), taskID)
		}
	}
}

func (q *Queue) cleanupExcessTasks(taskKeys []string) {
	type taskWithTime struct {
		ID        string
		CreatedAt time.Time
		Status    TaskStatus
	}

	tasks := make([]taskWithTime, 0, len(taskKeys))
	for _, taskID := range taskKeys {
		taskData, err := q.client.HGet(q.ctx, q.key("tasks"), taskID).Result()
		if err != nil {
			continue
		}

		task, err := UnmarshalTask([]byte(taskData))
		if err != nil {
			continue
		}

		tasks = append(tasks, taskWithTime{
			ID:        task.ID,
			CreatedAt: task.CreatedAt,
			Status:    task.Status,
		})
	}

	toDelete := len(tasks) - q.options.MaxMemoryTasks
	if toDelete <= 0 {
		return
	}

	deleted := 0
	for i := range tasks {
		if tasks[i].Status == TaskStatusCompleted || tasks[i].Status == TaskStatusFailed {
			q.client.HDel(q.ctx, q.key("tasks"), tasks[i].ID)
			deleted++
			if deleted >= toDelete {
				break
			}
		}
	}
}

func (q *Queue) processDelayedTasks() {
	if q.stopped {
		return
	}

	now := float64(time.Now().Unix())
	tasks, err := q.client.ZRangeByScore(q.ctx, q.key("delayed"), &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%f", now),
	}).Result()

	if err != nil || len(tasks) == 0 {
		return
	}

	for _, taskData := range tasks {
		select {
		case <-q.stopChan:
			return
		default:
			task, err := UnmarshalTask([]byte(taskData))
			if err != nil {
				continue
			}

			err = q.client.LPush(q.ctx, q.key("queue", task.Type), taskData).Err()
			if err == nil {
				q.client.ZRem(q.ctx, q.key("delayed"), taskData)
			}
		}
	}
}

func (q *Queue) EnqueueWithOptions(taskType string, data any, maxRetries int, delay time.Duration) (string, error) {
	if q.stopped {
		return "", ErrQueueStopped
	}

	if strings.TrimSpace(taskType) == "" {
		return "", errors.New("task type cannot be empty")
	}

	if len(taskType) > 100 {
		return "", errors.New("task type too long")
	}

	task, err := NewTask(taskType, data, maxRetries)
	if err != nil {
		return "", err
	}

	taskData, err := task.Marshal()
	if err != nil {
		return "", err
	}

	if delay > 0 {
		score := float64(time.Now().Add(delay).Unix())
		err = q.client.ZAdd(q.ctx, q.key("delayed"), &redis.Z{
			Score:  score,
			Member: taskData,
		}).Err()
	} else {
		err = q.client.LPush(q.ctx, q.key("queue", taskType), taskData).Err()
	}

	if err != nil {
		return "", err
	}

	err = q.client.HSet(q.ctx, q.key("tasks"), task.ID, taskData).Err()
	if err != nil {
		return "", err
	}

	return task.ID, nil
}

func (q *Queue) retryTask(task *Task, err error) error {
	task.MarkForRetry(err)

	delay := q.options.Backoff.NextDelay(task.Retries)

	if task.ShouldRetry() {
		taskData, marshalErr := task.Marshal()
		if marshalErr != nil {
			return marshalErr
		}

		if updateErr := q.client.HSet(q.ctx, q.key("tasks"), task.ID, taskData).Err(); updateErr != nil {
			return updateErr
		}

		score := float64(time.Now().Add(delay).Unix())
		return q.client.ZAdd(q.ctx, q.key("delayed"), &redis.Z{
			Score:  score,
			Member: taskData,
		}).Err()
	} else {
		return q.failTask(task, ErrMaxRetriesExceeded)
	}
}

func (q *Queue) HealthCheck() error {
	if q.stopped {
		return ErrQueueStopped
	}

	if err := q.client.Ping(q.ctx).Err(); err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	return nil
}

func (q *Queue) WaitForTasks(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			stats, err := q.GetStats()
			if err != nil {
				return err
			}
			if stats.Pending == 0 && stats.Processing == 0 {
				return nil
			}
		}
	}
}

func (q *Queue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.stopped {
		return nil
	}

	q.stopped = true
	close(q.stopChan)
	q.cancel()

	time.Sleep(100 * time.Millisecond)

	return q.client.Close()
}
