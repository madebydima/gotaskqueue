package gotaskqueue

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Handler функция для обработки задач
type Handler func(*Task) error

// Worker представляет воркер для обработки задач
type Worker struct {
	queue    *Queue
	handlers map[string]Handler
	wg       sync.WaitGroup
	stopChan chan struct{}
	stopped  bool
	mu       sync.RWMutex
	config   *WorkerConfig
}

// WorkerConfig содержит настройки воркера
type WorkerConfig struct {
	Concurrency  int
	PollInterval time.Duration
	TaskTypes    []string
}

// DefaultWorkerConfig возвращает настройки воркера по умолчанию
func DefaultWorkerConfig() *WorkerConfig {
	return &WorkerConfig{
		Concurrency:  1,
		PollInterval: time.Second,
		TaskTypes:    []string{},
	}
}

// WorkerOption функция для настройки воркера
type WorkerOption func(*WorkerConfig)

// WithConcurrency устанавливает количество concurrent воркеров
func WithConcurrency(concurrency int) WorkerOption {
	return func(c *WorkerConfig) {
		c.Concurrency = concurrency
	}
}

// WithPollInterval устанавливает интервал опроса очереди
func WithPollInterval(interval time.Duration) WorkerOption {
	return func(c *WorkerConfig) {
		c.PollInterval = interval
	}
}

// WithTaskTypes устанавливает типы задач для обработки
func WithTaskTypes(taskTypes ...string) WorkerOption {
	return func(c *WorkerConfig) {
		c.TaskTypes = taskTypes
	}
}

// NewWorker создает нового воркера
func (q *Queue) NewWorker(options ...WorkerOption) *Worker {
	config := DefaultWorkerConfig()
	for _, option := range options {
		option(config)
	}

	return &Worker{
		queue:    q,
		handlers: make(map[string]Handler),
		stopChan: make(chan struct{}),
		stopped:  false,
		config:   config,
	}
}

// Handle регистрирует обработчик для типа задачи
func (w *Worker) Handle(taskType string, handler Handler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[taskType] = handler
}

// getHandler возвращает обработчик для типа задачи
func (w *Worker) getHandler(taskType string) (Handler, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	handler, exists := w.handlers[taskType]
	return handler, exists
}

// Start запускает воркер
func (w *Worker) Start() {
	if w.stopped {
		return
	}

	log.Printf("Starting worker with %d concurrent workers", w.config.Concurrency)

	for i := 0; i < w.config.Concurrency; i++ {
		w.wg.Add(1)
		go w.work(i)
	}
}

// work основной цикл работы воркера
func (w *Worker) work(workerID int) {
	defer w.wg.Done()

	log.Printf("Worker %d started", workerID)

	// Обработка паники в горутине
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker %d panic recovered: %v", workerID, r)
		}
	}()

	for {
		select {
		case <-w.stopChan:
			log.Printf("Worker %d stopped", workerID)
			return
		default:
			if err := w.processTasks(workerID); err != nil {
				log.Printf("Worker %d error processing tasks: %v", workerID, err)
				// Пауза при ошибках
				time.Sleep(time.Second * 5)
			}
			time.Sleep(w.config.PollInterval)
		}
	}
}

// processTasks обрабатывает задачи из очереди
func (w *Worker) processTasks(workerID int) error {
	taskTypes := w.getTaskTypes()

	for _, taskType := range taskTypes {
		if w.stopped {
			return nil
		}

		task, err := w.queue.dequeue(taskType)
		if err != nil {
			return fmt.Errorf("dequeue error for type %s: %w", taskType, err)
		}

		if task == nil {
			continue // Нет задач в очереди
		}

		w.processTask(workerID, task)
	}

	return nil
}

// processTask обрабатывает одну задачу
func (w *Worker) processTask(workerID int, task *Task) {
	log.Printf("Worker %d: processing task %s of type %s", workerID, task.ID, task.Type)

	handler, exists := w.getHandler(task.Type)
	if !exists {
		log.Printf("Worker %d: no handler for task type %s", workerID, task.Type)
		if err := w.queue.failTask(task, ErrHandlerNotFound); err != nil {
			log.Printf("Worker %d: failed to mark task as failed: %v", workerID, err)
		}
		return
	}

	// Выполняем обработку с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic: %v", r)
			}
		}()
		errCh <- handler(task)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			log.Printf("Worker %d: task %s failed: %v", workerID, task.ID, err)
			if task.ShouldRetry() {
				if retryErr := w.queue.retryTask(task, err); retryErr != nil {
					log.Printf("Worker %d: failed to retry task %s: %v", workerID, task.ID, retryErr)
				} else {
					log.Printf("Worker %d: task %s scheduled for retry (%d/%d)",
						workerID, task.ID, task.Retries, task.MaxRetries)
				}
			} else {
				if failErr := w.queue.failTask(task, err); failErr != nil {
					log.Printf("Worker %d: failed to mark task as failed: %v", workerID, failErr)
				}
			}
		} else {
			if completeErr := w.queue.completeTask(task); completeErr != nil {
				log.Printf("Worker %d: failed to mark task as completed: %v", workerID, completeErr)
			} else {
				log.Printf("Worker %d: task %s completed successfully", workerID, task.ID)
			}
		}

	case <-ctx.Done():
		log.Printf("Worker %d: task %s timed out", workerID, task.ID)
		if retryErr := w.queue.retryTask(task, ctx.Err()); retryErr != nil {
			log.Printf("Worker %d: failed to retry task after timeout: %v", workerID, retryErr)
		}

	case <-w.stopChan:
		// Воркер останавливается, возвращаем задачу в очередь
		task.Status = TaskStatusPending
		taskData, err := task.Marshal()
		if err != nil {
			log.Printf("Worker %d: failed to marshal task %s for requeue: %v", workerID, task.ID, err)
			return
		}

		// Кладем задачу обратно в очередь
		if err := w.queue.client.LPush(w.queue.ctx, w.queue.key("queue", task.Type), taskData).Err(); err != nil {
			log.Printf("Worker %d: failed to requeue task %s: %v", workerID, task.ID, err)
		} else {
			log.Printf("Worker %d: returned task %s to queue due to shutdown", workerID, task.ID)
		}
	}
}

// getTaskTypes возвращает типы задач для обработки
func (w *Worker) getTaskTypes() []string {
	if len(w.config.TaskTypes) > 0 {
		return w.config.TaskTypes
	}

	// Если типы не указаны, возвращаем все зарегистрированные типы
	w.mu.RLock()
	defer w.mu.RUnlock()

	taskTypes := make([]string, 0, len(w.handlers))
	for taskType := range w.handlers {
		taskTypes = append(taskTypes, taskType)
	}

	return taskTypes
}

// Stop останавливает воркер
func (w *Worker) Stop() {
	if w.stopped {
		return
	}

	log.Println("Stopping worker...")
	w.stopped = true
	close(w.stopChan)
	w.wg.Wait()
	log.Println("Worker stopped")
}

// IsRunning проверяет, работает ли воркер
func (w *Worker) IsRunning() bool {
	return !w.stopped
}
