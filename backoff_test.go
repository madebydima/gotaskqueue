package gotaskqueue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConstantBackoff(t *testing.T) {
	backoff := ConstantBackoff{Delay: time.Second}

	for i := 0; i < 5; i++ {
		delay := backoff.NextDelay(i)
		assert.Equal(t, time.Second, delay)
	}
}

func TestExponentialBackoff(t *testing.T) {
	backoff := ExponentialBackoff{
		InitialDelay: time.Second,
		MaxDelay:     time.Minute,
		Multiplier:   2,
	}

	tests := []struct {
		retries  int
		expected time.Duration
	}{
		{0, time.Second},     // 1 * 2^0 = 1s
		{1, 2 * time.Second}, // 1 * 2^1 = 2s
		{2, 4 * time.Second}, // 1 * 2^2 = 4s
		{3, 8 * time.Second}, // 1 * 2^3 = 8s
		{10, time.Minute},    // Capped at max delay
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			delay := backoff.NextDelay(tt.retries)
			assert.Equal(t, tt.expected, delay)
		})
	}
}

func TestJitterBackoff(t *testing.T) {
	backoff := JitterBackoff{
		MinDelay: time.Second,
		MaxDelay: 5 * time.Second,
	}

	for i := 0; i < 10; i++ {
		delay := backoff.NextDelay(i)
		assert.True(t, delay >= time.Second && delay <= 5*time.Second,
			"Delay %v should be between 1s and 5s", delay)
	}
}

func TestCompositeBackoff(t *testing.T) {
	strategy1 := ConstantBackoff{Delay: time.Second}
	strategy2 := ConstantBackoff{Delay: 2 * time.Second}

	backoff := CompositeBackoff{
		Strategies: []BackoffStrategy{strategy1, strategy2},
	}

	// Should alternate between strategies
	assert.Equal(t, time.Second, backoff.NextDelay(0))   // strategy1
	assert.Equal(t, 2*time.Second, backoff.NextDelay(1)) // strategy2
	assert.Equal(t, time.Second, backoff.NextDelay(2))   // strategy1
	assert.Equal(t, 2*time.Second, backoff.NextDelay(3)) // strategy2
}

func TestCompositeBackoffEmpty(t *testing.T) {
	backoff := CompositeBackoff{} // No strategies

	delay := backoff.NextDelay(0)
	assert.Equal(t, time.Second, delay) // Default to 1 second
}
