package gotaskqueue

import "time"

type QueueOptions struct {
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	Namespace      string
	MaxRetries     int
	RetryDelay     time.Duration
	Backoff        BackoffStrategy
	TaskTTL        time.Duration
	MaxMemoryTasks int
}

func DefaultOptions() *QueueOptions {
	return &QueueOptions{
		RedisAddr:      "localhost:6379",
		RedisPassword:  "",
		RedisDB:        0,
		Namespace:      "gotaskqueue",
		MaxRetries:     3,
		RetryDelay:     time.Second * 5,
		Backoff:        ExponentialBackoff{InitialDelay: time.Second, MaxDelay: time.Minute, Multiplier: 2},
		TaskTTL:        time.Hour * 24 * 7,
		MaxMemoryTasks: 10000,
	}
}

type Option func(*QueueOptions)

func WithRedisAddr(addr string) Option {
	return func(o *QueueOptions) {
		o.RedisAddr = addr
	}
}

func WithRedisPassword(password string) Option {
	return func(o *QueueOptions) {
		o.RedisPassword = password
	}
}

func WithRedisDB(db int) Option {
	return func(o *QueueOptions) {
		o.RedisDB = db
	}
}

func WithNamespace(namespace string) Option {
	return func(o *QueueOptions) {
		o.Namespace = namespace
	}
}

func WithMaxRetries(retries int) Option {
	return func(o *QueueOptions) {
		o.MaxRetries = retries
	}
}

func WithRetryDelay(delay time.Duration) Option {
	return func(o *QueueOptions) {
		o.RetryDelay = delay
	}
}

func WithBackoffStrategy(strategy BackoffStrategy) Option {
	return func(o *QueueOptions) {
		o.Backoff = strategy
	}
}

func WithTaskTTL(ttl time.Duration) Option {
	return func(o *QueueOptions) {
		o.TaskTTL = ttl
	}
}

func WithMaxMemoryTasks(max int) Option {
	return func(o *QueueOptions) {
		o.MaxMemoryTasks = max
	}
}
