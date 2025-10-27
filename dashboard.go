package gotaskqueue

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// Dashboard представляет улучшенный веб-интерфейс для мониторинга очереди
type Dashboard struct {
	queue    *Queue
	addr     string
	template *template.Template
}

// DashboardData содержит все данные для отображения в дашборде
type DashboardData struct {
	Stats     *QueueStats
	Tasks     []*Task
	Config    *QueueOptions
	Timestamp time.Time
}

// NewDashboard создает новый dashboard
func (q *Queue) NewDashboard(addr string) *Dashboard {
	// Парсим шаблоны при создании
	tmpl := template.Must(template.New("dashboard").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("02.01.2006 15:04:05")
		},
		"formatDuration": func(d time.Duration) string {
			return d.Round(time.Second).String()
		},
		"percent": func(part, total int64) string {
			if total == 0 {
				return "0%"
			}
			return fmt.Sprintf("%.1f%%", float64(part)/float64(total)*100)
		},
		"taskStatusColor": func(status TaskStatus) string {
			switch status {
			case TaskStatusPending:
				return "bg-warning"
			case TaskStatusProcessing:
				return "bg-info"
			case TaskStatusCompleted:
				return "bg-success"
			case TaskStatusFailed:
				return "bg-danger"
			case TaskStatusRetry:
				return "bg-secondary"
			default:
				return "bg-light"
			}
		},
		"slice": func(start, end int, s string) string {
			if start < 0 {
				start = 0
			}
			if end > len(s) {
				end = len(s)
			}
			return s[start:end]
		},
	}).Parse(dashboardTemplate))

	return &Dashboard{
		queue:    q,
		addr:     addr,
		template: tmpl,
	}
}

// Start запускает веб-сервер dashboard
func (d *Dashboard) Start() error {
	r := mux.NewRouter()

	// Статические файлы из embed
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", StaticHandler()))

	// API endpoints
	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/stats", d.handleStatsAPI)
	api.HandleFunc("/tasks", d.handleTasksAPI)
	api.HandleFunc("/tasks/{id}", d.handleTaskAPI)
	api.HandleFunc("/tasks/{id}/retry", d.handleTaskRetryAPI).Methods("POST")
	api.HandleFunc("/health", d.handleHealthAPI)
	api.HandleFunc("/config", d.handleConfigAPI)

	// Web routes
	r.HandleFunc("/", d.handleDashboard)
	r.HandleFunc("/tasks", d.handleTasksPage)
	r.HandleFunc("/task/{id}", d.handleTaskDetailPage)

	log.Printf("🚀 Dashboard starting on http://%s", d.addr)
	return http.ListenAndServe(d.addr, r)
}

// handleDashboard отображает главную страницу dashboard
func (d *Dashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := d.queue.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tasks, err := d.getRecentTasks(10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := DashboardData{
		Stats:     stats,
		Tasks:     tasks,
		Config:    d.queue.options,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := d.template.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleTasksPage отображает страницу со списком задач
func (d *Dashboard) handleTasksPage(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	statusFilter := r.URL.Query().Get("status")
	limit := 50

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	tasks, err := d.getFilteredTasks(limit, statusFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stats, err := d.queue.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := DashboardData{
		Stats:     stats,
		Tasks:     tasks,
		Config:    d.queue.options,
		Timestamp: time.Now(),
	}

	// Используем тот же шаблон, но JavaScript покажет список задач
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := d.template.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleTaskDetailPage отображает детальную страницу задачи
func (d *Dashboard) handleTaskDetailPage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	task, err := d.queue.GetTask(taskID)
	if err != nil {
		if err == ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	stats, err := d.queue.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := DashboardData{
		Stats:     stats,
		Tasks:     []*Task{task},
		Config:    d.queue.options,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := d.template.Execute(w, data); err != nil {
		log.Printf("Template execution error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleStatsAPI возвращает статистику в формате JSON
func (d *Dashboard) handleStatsAPI(w http.ResponseWriter, r *http.Request) {
	stats, err := d.queue.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":     stats,
		"timestamp": time.Now(),
	})
}

// handleTasksAPI возвращает список задач с пагинацией и фильтрацией
func (d *Dashboard) handleTasksAPI(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	statusFilter := r.URL.Query().Get("status")
	// typeFilter := r.URL.Query().Get("type")

	limit := 50
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	tasks, err := d.getFilteredTasks(limit, statusFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Применяем пагинацию
	if offset > len(tasks) {
		offset = len(tasks)
	}
	end := offset + limit
	if end > len(tasks) {
		end = len(tasks)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks":     tasks[offset:end],
		"total":     len(tasks),
		"hasMore":   end < len(tasks),
		"timestamp": time.Now(),
	})
}

// handleTaskAPI возвращает информацию о конкретной задаче
func (d *Dashboard) handleTaskAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	task, err := d.queue.GetTask(taskID)
	if err != nil {
		if err == ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// handleTaskRetryAPI повторно запускает задачу
func (d *Dashboard) handleTaskRetryAPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	task, err := d.queue.GetTask(taskID)
	if err != nil {
		if err == ErrTaskNotFound {
			http.Error(w, "Task not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Сбрасываем статус задачи и добавляем обратно в очередь
	task.Status = TaskStatusPending
	task.Retries = 0
	task.Error = ""

	taskData, err := task.Marshal()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Обновляем задачу в хранилище
	if err := d.queue.client.HSet(d.queue.ctx, d.queue.key("tasks"), task.ID, taskData).Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Добавляем в очередь
	if err := d.queue.client.LPush(d.queue.ctx, d.queue.key("queue", task.Type), taskData).Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Task queued for retry",
		"taskId":  task.ID,
	})
}

// handleHealthAPI возвращает статус здоровья системы
func (d *Dashboard) handleHealthAPI(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
	}

	// Проверяем соединение с Redis
	if err := d.queue.client.Ping(d.queue.ctx).Err(); err != nil {
		health["status"] = "unhealthy"
		health["error"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleConfigAPI возвращает конфигурацию очереди
func (d *Dashboard) handleConfigAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d.queue.options)
}

// getRecentTasks возвращает последние задачи
func (d *Dashboard) getRecentTasks(limit int) ([]*Task, error) {
	taskKeys, err := d.queue.client.HKeys(d.queue.ctx, d.queue.key("tasks")).Result()
	if err != nil {
		return nil, err
	}

	var tasks []*Task
	for i, taskID := range taskKeys {
		if i >= limit {
			break
		}
		task, err := d.queue.GetTask(taskID)
		if err == nil {
			tasks = append(tasks, task)
		}
	}

	// Сортируем по времени создания (новые первые)
	for i, j := 0, len(tasks)-1; i < j; i, j = i+1, j-1 {
		tasks[i], tasks[j] = tasks[j], tasks[i]
	}

	return tasks, nil
}

// getFilteredTasks возвращает задачи с фильтрацией
func (d *Dashboard) getFilteredTasks(limit int, statusFilter string) ([]*Task, error) {
	taskKeys, err := d.queue.client.HKeys(d.queue.ctx, d.queue.key("tasks")).Result()
	if err != nil {
		return nil, err
	}

	var tasks []*Task
	for _, taskID := range taskKeys {
		task, err := d.queue.GetTask(taskID)
		if err != nil {
			continue
		}

		// Применяем фильтр по статусу
		if statusFilter != "" && string(task.Status) != statusFilter {
			continue
		}

		tasks = append(tasks, task)
	}

	// Сортируем по времени создания (новые первые)
	for i, j := 0, len(tasks)-1; i < j; i, j = i+1, j-1 {
		tasks[i], tasks[j] = tasks[j], tasks[i]
	}

	// Ограничиваем количество
	if len(tasks) > limit {
		tasks = tasks[:limit]
	}

	return tasks, nil
}

// StartDashboard запускает dashboard для очереди
func (q *Queue) StartDashboard(addr string) {
	dashboard := q.NewDashboard(addr)
	go func() {
		if err := dashboard.Start(); err != nil {
			log.Printf("Dashboard error: %v", err)
		}
	}()
}

const dashboardTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go TaskQueue Dashboard</title>
    <link href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css" rel="stylesheet">
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <link href="/static/css/dashboard.css" rel="stylesheet">
</head>
<body>
    <div class="dashboard-container">
        <!-- Header -->
        <div class="dashboard-header">
            <h1>Go TaskQueue Dashboard</h1>
            <p class="subtitle">Monitor and manage your task queue</p>
            <div class="header-controls">
                <div class="auto-refresh">
                    <input type="checkbox" id="autoRefreshToggle" checked>
                    <label for="autoRefreshToggle">Auto-refresh</label>
                </div>
                <div class="last-updated">
                    Updated: <span id="lastUpdated">{{.Timestamp | formatTime}}</span>
                </div>
            </div>
        </div>

        <!-- Navigation -->
        <div class="nav-tabs">
            <button class="nav-tab active" data-view="overview">
                <i class="fas fa-chart-pie"></i> Overview
            </button>
            <button class="nav-tab" data-view="tasks">
                <i class="fas fa-tasks"></i> Tasks
            </button>
            <button class="nav-tab" data-view="configuration">
                <i class="fas fa-cog"></i> Configuration
            </button>
        </div>

        <!-- Overview View -->
        <div id="overviewView" class="view-section">
            <!-- Stats Grid -->
            <div class="stats-grid">
                <div class="stat-card pending">
                    <div class="stat-label">Pending</div>
                    <div class="stat-value" id="pending">{{.Stats.Pending}}</div>
                    <div class="progress">
                        <div class="progress-bar progress-pending" id="pending-progress"></div>
                    </div>
                </div>
                <div class="stat-card processing">
                    <div class="stat-label">Processing</div>
                    <div class="stat-value" id="processing">{{.Stats.Processing}}</div>
                    <div class="progress">
                        <div class="progress-bar progress-processing" id="processing-progress"></div>
                    </div>
                </div>
                <div class="stat-card completed">
                    <div class="stat-label">Completed</div>
                    <div class="stat-value" id="completed">{{.Stats.Completed}}</div>
                    <div class="progress">
                        <div class="progress-bar progress-completed" id="completed-progress"></div>
                    </div>
                </div>
                <div class="stat-card failed">
                    <div class="stat-label">Failed</div>
                    <div class="stat-value" id="failed">{{.Stats.Failed}}</div>
                    <div class="progress">
                        <div class="progress-bar progress-failed" id="failed-progress"></div>
                    </div>
                </div>
                <div class="stat-card total">
                    <div class="stat-label">Total Tasks</div>
                    <div class="stat-value" id="total">{{.Stats.Total}}</div>
                    <div class="progress">
                        <div class="progress-bar" style="width: 100%; background: var(--secondary);"></div>
                    </div>
                </div>
            </div>

            <!-- Charts -->
            <div class="charts-container">
                <div class="chart-card">
                    <h3>Task Distribution</h3>
                    <div style="height: 300px;">
                        <canvas id="statsChart"></canvas>
                    </div>
                </div>
                <div class="chart-card">
                    <h3>Processing Timeline</h3>
                    <div style="height: 300px;">
                        <canvas id="timelineChart"></canvas>
                    </div>
                </div>
            </div>

            <!-- Recent Tasks -->
            <div class="tasks-section">
                <div class="controls-row">
                    <h3>Recent Tasks</h3>
                    <a href="/tasks" class="btn btn-primary">
                        <i class="fas fa-external-link-alt"></i> View All
                    </a>
                </div>
                <div class="table-responsive">
                    <table class="tasks-table">
                        <thead>
                            <tr>
                                <th>ID</th>
                                <th>Type</th>
                                <th>Status</th>
                                <th>Retries</th>
                                <th>Created</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody id="tasksContainer">
                            {{range .Tasks}}
                            <tr>
                                <td><code class="task-id">{{.ID | slice 0 8}}...</code></td>
                                <td><span class="badge badge-type">{{.Type}}</span></td>
                                <td>
                                    <span class="status-badge status-{{.Status}}">
                                        {{.Status}}
                                    </span>
                                </td>
                                <td>{{.Retries}} / {{.MaxRetries}}</td>
                                <td>{{.CreatedAt | formatTime}}</td>
                                <td>
                                    <div style="display: flex; gap: 4px;">
                                        <a href="/task/{{.ID}}" class="btn btn-sm" title="View details">
                                            <i class="fas fa-eye"></i>
                                        </a>
                                        {{if eq .Status "failed"}}
                                        <button class="btn btn-warning btn-sm retry-task" 
                                                data-task-id="{{.ID}}" title="Retry task">
                                            <i class="fas fa-redo"></i>
                                        </button>
                                        {{end}}
                                    </div>
                                </td>
                            </tr>
                            {{end}}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>

        <!-- Tasks View -->
        <div id="tasksView" class="view-section" style="display: none;">
            <div class="tasks-section">
                <div class="controls-row">
                    <h3>All Tasks</h3>
                    <div style="display: flex; gap: 8px;">
                        <input type="text" id="taskSearch" class="form-control" 
                               placeholder="Search tasks..." style="width: 200px;">
                        <select id="statusFilter" class="form-select">
                            <option value="">All Statuses</option>
                            <option value="pending">Pending</option>
                            <option value="processing">Processing</option>
                            <option value="completed">Completed</option>
                            <option value="failed">Failed</option>
                            <option value="retry">Retry</option>
                        </select>
                    </div>
                </div>
                <div class="table-responsive">
                    <table class="tasks-table">
                        <thead>
                            <tr>
                                <th>ID</th>
                                <th>Type</th>
                                <th>Status</th>
                                <th>Retries</th>
                                <th>Created</th>
                                <th>Processed</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody id="allTasksContainer">
                            <!-- JavaScript will populate this -->
                        </tbody>
                    </table>
                </div>
            </div>
        </div>

        <!-- Configuration View -->
        <div id="configurationView" class="view-section" style="display: none;">
            <div class="chart-card">
                <h3>Queue Configuration</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <strong>Redis Address</strong>
                        <span>{{.Config.RedisAddr}}</span>
                    </div>
                    <div class="config-item">
                        <strong>Namespace</strong>
                        <span>{{.Config.Namespace}}</span>
                    </div>
                    <div class="config-item">
                        <strong>Max Retries</strong>
                        <span>{{.Config.MaxRetries}}</span>
                    </div>
                    <div class="config-item">
                        <strong>Retry Delay</strong>
                        <span>{{.Config.RetryDelay | formatDuration}}</span>
                    </div>
                    <div class="config-item">
                        <strong>Task TTL</strong>
                        <span>{{.Config.TaskTTL | formatDuration}}</span>
                    </div>
                    <div class="config-item">
                        <strong>Max Memory Tasks</strong>
                        <span>{{.Config.MaxMemoryTasks}}</span>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <script src="/static/js/dashboard.js"></script>
</body>
</html>
`
