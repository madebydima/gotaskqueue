package gotaskqueue

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// Dashboard представляет веб-интерфейс для мониторинга очереди
type Dashboard struct {
	queue *Queue
	addr  string
}

// NewDashboard создает новый dashboard
func (q *Queue) NewDashboard(addr string) *Dashboard {
	return &Dashboard{
		queue: q,
		addr:  addr,
	}
}

// Start запускает веб-сервер dashboard
func (d *Dashboard) Start() error {
	r := mux.NewRouter()

	r.HandleFunc("/", d.handleDashboard)
	r.HandleFunc("/api/stats", d.handleStatsAPI)
	r.HandleFunc("/api/tasks", d.handleTasksAPI)
	r.HandleFunc("/api/tasks/{id}", d.handleTaskAPI)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/",
		http.FileServer(http.Dir("./static/"))))

	log.Printf("Dashboard starting on %s", d.addr)
	return http.ListenAndServe(d.addr, r)
}

// handleDashboard отображает главную страницу dashboard
func (d *Dashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Go TaskQueue Dashboard</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .stats { display: flex; gap: 20px; margin-bottom: 20px; }
        .stat-card { padding: 15px; border: 1px solid #ddd; border-radius: 5px; min-width: 150px; }
        .stat-value { font-size: 24px; font-weight: bold; }
        .task-list { margin-top: 20px; }
        .task { padding: 10px; border: 1px solid #ddd; margin: 5px 0; border-radius: 3px; }
        .task.pending { border-left: 4px solid #ffc107; }
        .task.processing { border-left: 4px solid #17a2b8; }
        .task.completed { border-left: 4px solid #28a745; }
        .task.failed { border-left: 4px solid #dc3545; }
    </style>
</head>
<body>
    <h1>TaskQueue Dashboard</h1>
    
    <div class="stats">
        <div class="stat-card">
            <div>Pending</div>
            <div class="stat-value" id="pending">0</div>
        </div>
        <div class="stat-card">
            <div>Processing</div>
            <div class="stat-value" id="processing">0</div>
        </div>
        <div class="stat-card">
            <div>Completed</div>
            <div class="stat-value" id="completed">0</div>
        </div>
        <div class="stat-card">
            <div>Failed</div>
            <div class="stat-value" id="failed">0</div>
        </div>
        <div class="stat-card">
            <div>Total</div>
            <div class="stat-value" id="total">0</div>
        </div>
    </div>

    <canvas id="statsChart" width="400" height="200"></canvas>

    <div class="task-list" id="taskList">
        <h3>Recent Tasks</h3>
    </div>

        <script>
        let chart;
        
        async function loadStats() {
            try {
                const response = await fetch('/api/stats');
                const stats = await response.json();
                
                document.getElementById('pending').textContent = stats.pending;
                document.getElementById('processing').textContent = stats.processing;
                document.getElementById('completed').textContent = stats.completed;
                document.getElementById('failed').textContent = stats.failed;
                document.getElementById('total').textContent = stats.total;
                
                updateChart(stats);
                loadTasks();
            } catch (error) {
                console.error('Error loading stats:', error);
            }
        }
        
        function updateChart(stats) {
            const ctx = document.getElementById('statsChart').getContext('2d');
            
            if (chart) {
                chart.destroy();
            }
            
            chart = new Chart(ctx, {
                type: 'doughnut',
                data: {
                    labels: ['Pending', 'Processing', 'Completed', 'Failed'],
                    datasets: [{
                        data: [stats.pending, stats.processing, stats.completed, stats.failed],
                        backgroundColor: ['#ffc107', '#17a2b8', '#28a745', '#dc3545']
                    }]
                }
            });
        }
        
        async function loadTasks() {
            try {
                const response = await fetch('/api/tasks?limit=10');
                const tasks = await response.json();
                
                const taskList = document.getElementById('taskList');
                taskList.innerHTML = '<h3>Recent Tasks</h3>';
                
                tasks.forEach(task => {
                    const taskEl = document.createElement('div');
                    taskEl.className = 'task ' + task.status;
                    taskEl.innerHTML = 
                        '<strong>ID:</strong> ' + task.id + '<br>' +
                        '<strong>Type:</strong> ' + task.type + '<br>' +
                        '<strong>Status:</strong> ' + task.status + '<br>' +
                        '<strong>Created:</strong> ' + new Date(task.created_at).toLocaleString() + '<br>' +
                        (task.error ? '<strong>Error:</strong> ' + task.error + '<br>' : '');
                    taskList.appendChild(taskEl);
                });
            } catch (error) {
                console.error('Error loading tasks:', error);
            }
        }
        
        // Обновляем статистику каждые 5 секунд
        setInterval(loadStats, 5000);
        loadStats();
    </script>
</body>
</html>
    `

	t, err := template.New("dashboard").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, nil)
}

// handleStatsAPI возвращает статистику в формате JSON
func (d *Dashboard) handleStatsAPI(w http.ResponseWriter, r *http.Request) {
	stats, err := d.queue.GetStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleTasksAPI возвращает список задач
func (d *Dashboard) handleTasksAPI(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	// Получаем все задачи (в реальной реализации нужно добавить пагинацию)
	taskKeys, err := d.queue.client.HKeys(d.queue.ctx, d.queue.key("tasks")).Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tasks := make([]*Task, 0, len(taskKeys))
	for i, taskID := range taskKeys {
		if i >= limit {
			break
		}

		task, err := d.queue.GetTask(taskID)
		if err == nil {
			tasks = append(tasks, task)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
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

// StartDashboard запускает dashboard для очереди
func (q *Queue) StartDashboard(addr string) {
	dashboard := q.NewDashboard(addr)
	go func() {
		if err := dashboard.Start(); err != nil {
			log.Printf("Dashboard error: %v", err)
		}
	}()
}
