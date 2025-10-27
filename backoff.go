package gotaskqueue

import (
	"math"
	"math/rand"
	"time"
)

type BackoffStrategy interface {
	NextDelay(retries int) time.Duration
}

type ConstantBackoff struct {
	Delay time.Duration
}

func (b ConstantBackoff) NextDelay(retries int) time.Duration {
	return b.Delay
}

type ExponentialBackoff struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func (b ExponentialBackoff) NextDelay(retries int) time.Duration {
	delay := time.Duration(float64(b.InitialDelay) * math.Pow(b.Multiplier, float64(retries)))
	if delay > b.MaxDelay {
		return b.MaxDelay
	}
	return delay
}

type JitterBackoff struct {
	MinDelay time.Duration
	MaxDelay time.Duration
}

func (b JitterBackoff) NextDelay(retries int) time.Duration {
	min := int64(b.MinDelay)
	max := int64(b.MaxDelay)
	if min >= max {
		return b.MinDelay
	}
	return time.Duration(rand.Int63n(max-min) + min)
}

type CompositeBackoff struct {
	Strategies []BackoffStrategy
}

func (b CompositeBackoff) NextDelay(retries int) time.Duration {
	if len(b.Strategies) == 0 {
		return time.Second
	}
	strategyIndex := retries % len(b.Strategies)
	return b.Strategies[strategyIndex].NextDelay(retries)
}
