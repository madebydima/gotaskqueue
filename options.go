package gotaskqueue

import "time"

// QueueOptions содержит настройки для очереди
type QueueOptions struct {
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	Namespace      string
	MaxRetries     int
	RetryDelay     time.Duration
	Backoff        BackoffStrategy
	TaskTTL        time.Duration // TTL для хранения выполненных задач
	MaxMemoryTasks int           // Максимальное количество хранимых задач
}

// DefaultOptions возвращает настройки по умолчанию
func DefaultOptions() *QueueOptions {
	return &QueueOptions{
		RedisAddr:      "localhost:6379",
		RedisPassword:  "",
		RedisDB:        0,
		Namespace:      "gotaskqueue",
		MaxRetries:     3,
		RetryDelay:     time.Second * 5,
		Backoff:        ExponentialBackoff{InitialDelay: time.Second, MaxDelay: time.Minute, Multiplier: 2},
		TaskTTL:        time.Hour * 24 * 7, // 1 неделя
		MaxMemoryTasks: 10000,
	}
}

// Option функция для настройки очереди
type Option func(*QueueOptions)

// WithRedisAddr устанавливает адрес Redis
func WithRedisAddr(addr string) Option {
	return func(o *QueueOptions) {
		o.RedisAddr = addr
	}
}

// WithRedisPassword устанавливает пароль Redis
func WithRedisPassword(password string) Option {
	return func(o *QueueOptions) {
		o.RedisPassword = password
	}
}

// WithRedisDB устанавливает базу данных Redis
func WithRedisDB(db int) Option {
	return func(o *QueueOptions) {
		o.RedisDB = db
	}
}

// WithNamespace устанавливает пространство имен
func WithNamespace(namespace string) Option {
	return func(o *QueueOptions) {
		o.Namespace = namespace
	}
}

// WithMaxRetries устанавливает максимальное количество повторов
func WithMaxRetries(retries int) Option {
	return func(o *QueueOptions) {
		o.MaxRetries = retries
	}
}

// WithRetryDelay устанавливает задержку между повторами
func WithRetryDelay(delay time.Duration) Option {
	return func(o *QueueOptions) {
		o.RetryDelay = delay
	}
}

// WithBackoffStrategy устанавливает стратегию backoff
func WithBackoffStrategy(strategy BackoffStrategy) Option {
	return func(o *QueueOptions) {
		o.Backoff = strategy
	}
}

// WithTaskTTL устанавливает TTL для задач
func WithTaskTTL(ttl time.Duration) Option {
	return func(o *QueueOptions) {
		o.TaskTTL = ttl
	}
}

// WithMaxMemoryTasks устанавливает максимальное количество хранимых задач
func WithMaxMemoryTasks(max int) Option {
	return func(o *QueueOptions) {
		o.MaxMemoryTasks = max
	}
}
