package gotaskqueue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	require.NotNil(t, opts)

	assert.Equal(t, "localhost:6379", opts.RedisAddr)
	assert.Equal(t, "", opts.RedisPassword)
	assert.Equal(t, 0, opts.RedisDB)
	assert.Equal(t, "gotaskqueue", opts.Namespace)
	assert.Equal(t, 3, opts.MaxRetries)
	assert.Equal(t, 5*time.Second, opts.RetryDelay)
	assert.NotNil(t, opts.Backoff)
	assert.Equal(t, 24*7*time.Hour, opts.TaskTTL) // 1 week
	assert.Equal(t, 10000, opts.MaxMemoryTasks)
}

func TestOptionFunctions(t *testing.T) {
	t.Run("WithRedisAddr", func(t *testing.T) {
		opts := DefaultOptions()
		WithRedisAddr("redis.example.com:6379")(opts)
		assert.Equal(t, "redis.example.com:6379", opts.RedisAddr)
	})

	t.Run("WithRedisPassword", func(t *testing.T) {
		opts := DefaultOptions()
		WithRedisPassword("secret")(opts)
		assert.Equal(t, "secret", opts.RedisPassword)
	})

	t.Run("WithRedisDB", func(t *testing.T) {
		opts := DefaultOptions()
		WithRedisDB(5)(opts)
		assert.Equal(t, 5, opts.RedisDB)
	})

	t.Run("WithNamespace", func(t *testing.T) {
		opts := DefaultOptions()
		WithNamespace("myapp")(opts)
		assert.Equal(t, "myapp", opts.Namespace)
	})

	t.Run("WithMaxRetries", func(t *testing.T) {
		opts := DefaultOptions()
		WithMaxRetries(10)(opts)
		assert.Equal(t, 10, opts.MaxRetries)
	})

	t.Run("WithRetryDelay", func(t *testing.T) {
		opts := DefaultOptions()
		WithRetryDelay(30 * time.Second)(opts)
		assert.Equal(t, 30*time.Second, opts.RetryDelay)
	})

	t.Run("WithTaskTTL", func(t *testing.T) {
		opts := DefaultOptions()
		WithTaskTTL(24 * time.Hour)(opts)
		assert.Equal(t, 24*time.Hour, opts.TaskTTL)
	})

	t.Run("WithMaxMemoryTasks", func(t *testing.T) {
		opts := DefaultOptions()
		WithMaxMemoryTasks(5000)(opts)
		assert.Equal(t, 5000, opts.MaxMemoryTasks)
	})
}

func TestBackoffOptions(t *testing.T) {
	t.Run("WithConstantBackoff", func(t *testing.T) {
		opts := DefaultOptions()
		backoff := ConstantBackoff{Delay: time.Second * 10}
		WithBackoffStrategy(backoff)(opts)

		assert.IsType(t, ConstantBackoff{}, opts.Backoff)
		assert.Equal(t, time.Second*10, opts.Backoff.NextDelay(0))
		assert.Equal(t, time.Second*10, opts.Backoff.NextDelay(5)) // Always the same
	})

	t.Run("WithExponentialBackoff", func(t *testing.T) {
		opts := DefaultOptions()
		backoff := ExponentialBackoff{
			InitialDelay: time.Second,
			MaxDelay:     time.Minute,
			Multiplier:   2,
		}
		WithBackoffStrategy(backoff)(opts)

		assert.IsType(t, ExponentialBackoff{}, opts.Backoff)
		assert.Equal(t, time.Second, opts.Backoff.NextDelay(0))
		assert.Equal(t, 2*time.Second, opts.Backoff.NextDelay(1))
		assert.Equal(t, 4*time.Second, opts.Backoff.NextDelay(2))
	})

	t.Run("WithJitterBackoff", func(t *testing.T) {
		opts := DefaultOptions()
		backoff := JitterBackoff{
			MinDelay: time.Second,
			MaxDelay: 5 * time.Second,
		}
		WithBackoffStrategy(backoff)(opts)

		assert.IsType(t, JitterBackoff{}, opts.Backoff)

		// Test multiple calls to ensure it stays within bounds
		for i := 0; i < 10; i++ {
			delay := opts.Backoff.NextDelay(i)
			assert.True(t, delay >= time.Second && delay <= 5*time.Second,
				"Delay %v should be between 1s and 5s", delay)
		}
	})

	t.Run("WithCompositeBackoff", func(t *testing.T) {
		opts := DefaultOptions()
		strategies := []BackoffStrategy{
			ConstantBackoff{Delay: time.Second},
			ConstantBackoff{Delay: 2 * time.Second},
		}
		backoff := CompositeBackoff{Strategies: strategies}
		WithBackoffStrategy(backoff)(opts)

		assert.IsType(t, CompositeBackoff{}, opts.Backoff)
		assert.Equal(t, time.Second, opts.Backoff.NextDelay(0))   // First strategy
		assert.Equal(t, 2*time.Second, opts.Backoff.NextDelay(1)) // Second strategy
		assert.Equal(t, time.Second, opts.Backoff.NextDelay(2))   // First strategy again
	})
}

func TestWorkerOptions(t *testing.T) {
	t.Run("WithConcurrency", func(t *testing.T) {
		queue, err := New(WithRedisAddr("localhost:6379"))
		require.NoError(t, err)
		defer queue.Close()

		worker := queue.NewWorker(WithConcurrency(5))
		assert.Equal(t, 5, worker.config.Concurrency)
	})

	t.Run("WithPollInterval", func(t *testing.T) {
		queue, err := New(WithRedisAddr("localhost:6379"))
		require.NoError(t, err)
		defer queue.Close()

		worker := queue.NewWorker(WithPollInterval(2 * time.Second))
		assert.Equal(t, 2*time.Second, worker.config.PollInterval)
	})

	t.Run("WithTaskTypes", func(t *testing.T) {
		queue, err := New(WithRedisAddr("localhost:6379"))
		require.NoError(t, err)
		defer queue.Close()

		worker := queue.NewWorker(WithTaskTypes("email", "report", "cleanup"))
		assert.Equal(t, []string{"email", "report", "cleanup"}, worker.config.TaskTypes)
	})
}

func TestOptionChaining(t *testing.T) {
	opts := DefaultOptions()

	// Apply multiple options
	WithRedisAddr("custom.redis:6379")(opts)
	WithNamespace("test-app")(opts)
	WithMaxRetries(5)(opts)
	WithRetryDelay(10 * time.Second)(opts)

	assert.Equal(t, "custom.redis:6379", opts.RedisAddr)
	assert.Equal(t, "test-app", opts.Namespace)
	assert.Equal(t, 5, opts.MaxRetries)
	assert.Equal(t, 10*time.Second, opts.RetryDelay)
}
