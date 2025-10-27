package gotaskqueue

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

// TestRedisClient создает тестового Redis клиента
func TestRedisClient(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Используем базу 1 для тестов
	})

	// Проверяем подключение
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("Failed to connect to test Redis: %v", err)
	}

	// Очищаем базу перед тестом
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Failed to flush test Redis: %v", err)
	}

	t.Cleanup(func() {
		client.FlushDB(ctx)
		client.Close()
	})

	return client
}

// WaitForCondition ждет пока условие не станет true или не истечет таймаут
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, checkInterval time.Duration) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if condition() {
				return true
			}
		}
	}
}

// CreateTestQueue создает тестовую очередь
func CreateTestQueue(t *testing.T, options ...Option) *Queue {
	t.Helper()

	opts := append([]Option{
		WithRedisAddr("localhost:6379"),
		WithNamespace("test-" + t.Name()),
		WithMaxRetries(3),
		WithRetryDelay(10 * time.Millisecond),
	}, options...)

	queue, err := New(opts...)
	if err != nil {
		t.Fatalf("Failed to create test queue: %v", err)
	}

	t.Cleanup(func() {
		queue.Close()
	})

	return queue
}

// CreateTestWorker создает тестового воркера
func CreateTestWorker(t *testing.T, queue *Queue, options ...WorkerOption) *Worker {
	t.Helper()

	worker := queue.NewWorker(options...)

	t.Cleanup(func() {
		if worker.IsRunning() {
			worker.Stop()
		}
	})

	return worker
}
