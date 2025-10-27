# Go TaskQueue 🚀

A high-performance, Redis-backed distributed task queue for Go applications. Because your background jobs deserve more than just a `goroutine` and a prayer.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go)](https://golang.org)
[![Redis](https://img.shields.io/badge/Redis-DC382D?style=for-the-badge&logo=redis&logoColor=white)](https://redis.io)
[![License](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)

## ⚡ "I need background jobs, but I don't need Kubernetes-level complexity"

We've all been there. You need to process emails, generate reports, or handle webhooks, but setting up RabbitMQ or Kafka feels like using a sledgehammer to crack a nut. **Go TaskQueue** is here to help - it's the Goldilocks solution: not too simple, not too complex, just right.

### Why Go TaskQueue? 🤔

| Feature | Plain Goroutines | Go TaskQueue | Heavyweight Solutions |
|---------|------------------|-----------|----------------------|
| Persistence | ❌ Gone on restart | ✅ Redis-backed | ✅ |
| Retry Logic | ❌ You implement | ✅ Built-in | ✅ |
| Monitoring | ❌ Good luck | ✅ Web Dashboard | ✅ |
| Scalability | ❌ Manual | ✅ Multi-worker | ✅ |
| Delayed Jobs | ❌ `time.Sleep()` | ✅ Built-in | ✅ |
| Setup Time | 1 minute | 5 minutes | 1 hour+ |

## 🎯 Features

### 🚀 Production Ready
- **Persistent Storage**: Tasks survive restarts (thanks, Redis!)
- **Retry Mechanisms**: Exponential backoff, jitter, and custom strategies
- **Graceful Shutdown**: No lost tasks during deployment
- **Health Checks**: Built-in monitoring endpoints
- **Memory Management**: Automatic cleanup of old tasks

### 🎪 Developer Experience
- **Simple API**: `queue.Enqueue("send_email", data)` - that's it!
- **Typed Handlers**: Full Go generics support
- **Web Dashboard**: Real-time monitoring without setting up Prometheus
- **Sensible Defaults**: Works out of the box
- **Comprehensive Testing**: 90%+ test coverage

### 📈 Scalable Architecture
- **Multiple Workers**: Scale horizontally
- **Redis Cluster**: Ready for high-availability setups
- **Backpressure Handling**: Won't melt down under load
- **Efficient Processing**: Minimal Redis operations

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Redis 6+ (because we like modern things)

### Installation

```bash
go get github.com/madebydima/gotaskqueue
```

### Basic Usage

```go
package main

import (
    "log"
    "time"
    
    "github.com/madebydima/gotaskqueue"
)

func main() {
    // Create a queue (connects to localhost:6379 by default)
    queue, err := gotaskqueue.New()
    if err != nil {
        log.Fatal("Failed to create queue:", err)
    }
    defer queue.Close()

    // Start the web dashboard (optional but recommended)
    queue.StartDashboard(":8080")

    // Create a worker
    worker := queue.NewWorker()

    // Register task handlers
    worker.Handle("send_email", sendEmailHandler)
    worker.Handle("generate_report", generateReportHandler)

    // Start processing
    go worker.Start()
    defer worker.Stop()

    // Enqueue tasks
    taskID, err := queue.Enqueue("send_email", map[string]interface{}{
        "to":      "user@example.com",
        "subject": "Welcome!",
        "body":    "Thanks for joining!",
    })
    if err != nil {
        log.Printf("Failed to enqueue: %v", err)
    } else {
        log.Printf("Enqueued task: %s", taskID)
    }

    // Keep the application running
    select {}
}

func sendEmailHandler(task *gotaskqueue.Task) error {
    var data struct {
        To      string `json:"to"`
        Subject string `json:"subject"`
        Body    string `json:"body"`
    }
    
    if err := task.UnmarshalData(&data); err != nil {
        return err
    }
    
    log.Printf("Sending email to %s: %s", data.To, data.Subject)
    // Your email sending logic here
    return nil
}

func generateReportHandler(task *gotaskqueue.Task) error {
    log.Printf("Generating report...")
    time.Sleep(5 * time.Second) // Simulate work
    return nil
}
```

## 📊 Web Dashboard

Start the dashboard and visit `http://localhost:8080` to see:

![Dashboard](https://via.placeholder.com/800x400/35495E/FFFFFF?text=TaskQueue+Dashboard)

- Real-time queue statistics
- Task status overview
- Failed tasks and error details
- Processing rates and performance metrics

## ⚙️ Configuration

### Queue Options

```go
queue, err := gotaskqueue.New(
    gotaskqueue.WithRedisAddr("redis-server:6379"),
    gotaskqueue.WithRedisPassword("secret"),
    gotaskqueue.WithNamespace("myapp"),
    gotaskqueue.WithMaxRetries(5),
    gotaskqueue.WithTaskTTL(7 * 24 * time.Hour), // Keep tasks for 1 week
    gotaskqueue.WithBackoffStrategy(
        gotaskqueue.ExponentialBackoff{
            InitialDelay: time.Second,
            MaxDelay:     time.Minute,
            Multiplier:   2,
        },
    ),
)
```

### Worker Options

```go
worker := queue.NewWorker(
    gotaskqueue.WithConcurrency(10),           // 10 concurrent workers
    gotaskqueue.WithPollInterval(2 * time.Second),
    gotaskqueue.WithTaskTypes("email", "report"), // Only process these types
)
```

## 🎪 Advanced Features

### Delayed Tasks

```go
// Send welcome email in 1 hour
queue.EnqueueDelayed("send_email", welcomeData, time.Hour)
```

### Custom Retry Strategies

```go
// Retry with exponential backoff and jitter
strategy := gotaskqueue.CompositeBackoff{
    Strategies: []gotaskqueue.BackoffStrategy{
        gotaskqueue.ExponentialBackoff{
            InitialDelay: time.Second,
            MaxDelay:     time.Minute * 5,
            Multiplier:   2,
        },
        gotaskqueue.JitterBackoff{
            MinDelay: time.Second * 30,
            MaxDelay: time.Minute * 2,
        },
    },
}

queue, err := gotaskqueue.New(
    gotaskqueue.WithBackoffStrategy(strategy),
)
```

### Batch Operations

```go
// Enqueue multiple tasks efficiently
for i := 0; i < 1000; i++ {
    queue.Enqueue("process_item", Item{ID: i})
}
```

## 🏗️ Architecture

### How It Works

```go
// Simplified internal flow
func Magic() {
    for {
        // 1. Check for delayed tasks ready to process
        q.processDelayedTasks()
        
        // 2. Get next task from queue
        task := q.dequeue("email")
        
        // 3. Process with retry logic
        if err := process(task); err != nil {
            if task.ShouldRetry() {
                q.retryTask(task, err) // With smart backoff
            } else {
                q.failTask(task, err)  // Dead letter queue
            }
        } else {
            q.completeTask(task)       // Mark success
        }
    }
}
```

### Data Model

```
Redis Structure:
- gotaskqueue:queue:{type}        (List) - Pending tasks
- gotaskqueue:delayed             (Sorted Set) - Delayed tasks
- gotaskqueue:tasks               (Hash) - Task metadata and status
```

## 📈 Performance

On average hardware (4-core CPU, 8GB RAM):

- **Throughput**: ~5,000 tasks/second
- **Latency**: < 10ms (queue to start processing)
- **Memory**: ~2KB per task (including metadata)
- **Redis**: Minimal CPU usage, efficient memory patterns

## 🚨 Production Checklist

Before deploying to production:

- [ ] Set appropriate `MaxRetries` for your use case
- [ ] Configure Redis persistence (`appendonly yes`)
- [ ] Set memory limits with `WithMaxMemoryTasks()`
- [ ] Configure health check endpoints
- [ ] Set up Redis monitoring
- [ ] Configure appropriate timeouts
- [ ] Test graceful shutdown procedures

## 🤝 Contributing

We love contributions! Especially:

- **Bug Reports**: The more details, the better!
- **Feature Ideas**: What would make your life easier?
- **Documentation**: Found something unclear? Fix it!
- **Code**: Pull requests are always welcome.

### Development Setup

```bash
# 1. Clone the repository
git clone https://github.com/madebydima/gotaskqueue.git
cd gotaskqueue

# 2. Start Redis
docker run -d -p 6379:6379 redis:7-alpine

# 3. Run tests
make test

# 4. Run the example
make run-example
```

## 📝 License

MIT License - feel free to use this in personal, internal, or commercial projects. Just don't sue us if your email sending gets too fast and breaks the internet.

## 🆘 Troubleshooting

### Common Issues

**Problem**: "Redis connection failed"
```go
// Solution: Check your Redis connection
queue, err := gotaskqueue.New(
    gotaskqueue.WithRedisAddr("localhost:6379"),
)
```

**Problem**: Tasks not processing
```bash
# Solution: Check if workers are running
redis-cli LLEN gotaskqueue:queue:email
```

**Problem**: Memory usage growing
```go
// Solution: Set task TTL
queue, err := gotaskqueue.New(
    gotaskqueue.WithTaskTTL(24 * time.Hour),
    gotaskqueue.WithMaxMemoryTasks(10000),
)
```

## 🎉 Acknowledgments

Built with ❤️ by [MADEBYDIMA](https://github.com/madebydima) and the power of:

- [Redis](https://redis.io) for being ridiculously fast
- [Go](https://golang.org) for making concurrency actually enjoyable
- [Caffeine](https://en.wikipedia.org/wiki/Caffeine) for making late-night coding possible

---

**Ready to queue?** 

```go
// What are you waiting for?
queue.Enqueue("get_started", nil)
```