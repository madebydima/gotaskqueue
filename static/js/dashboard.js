class TaskQueueDashboard {
    constructor() {
        this.stats = null;
        this.tasks = [];
        this.charts = {};
        this.autoRefresh = true;
        this.currentView = 'overview';
        this.init();
    }

    init() {
        this.loadStats();
        this.loadTasks();
        this.setupEventListeners();
        this.startAutoRefresh();
        this.initCharts();
    }

    setupEventListeners() {
        // Auto-refresh toggle
        document.getElementById('autoRefreshToggle')?.addEventListener('change', (e) => {
            this.autoRefresh = e.target.checked;
            if (this.autoRefresh) {
                this.startAutoRefresh();
            } else {
                this.stopAutoRefresh();
            }
        });

        // Navigation
        document.querySelectorAll('.nav-tab').forEach(tab => {
            tab.addEventListener('click', (e) => {
                this.switchView(e.target.dataset.view);
            });
        });

        // Task actions
        document.addEventListener('click', (e) => {
            if (e.target.classList.contains('retry-task') || 
                e.target.parentElement.classList.contains('retry-task')) {
                const taskId = e.target.dataset.taskId || e.target.parentElement.dataset.taskId;
                this.retryTask(taskId);
            }
        });

        // Search and filter
        const taskSearch = document.getElementById('taskSearch');
        const statusFilter = document.getElementById('statusFilter');

        taskSearch?.addEventListener('input', (e) => {
            this.filterTasks(e.target.value);
        });

        statusFilter?.addEventListener('change', (e) => {
            this.loadTasks(e.target.value);
        });
    }

    async loadStats() {
        try {
            const response = await fetch('/api/stats');
            const data = await response.json();
            this.stats = data.stats;
            this.updateStatsDisplay();
            this.updateCharts();
        } catch (error) {
            console.error('Failed to load stats:', error);
            this.showError('Failed to load statistics');
        }
    }

    async loadTasks(statusFilter = '') {
        try {
            const url = statusFilter ? `/api/tasks?status=${statusFilter}` : '/api/tasks';
            const response = await fetch(url);
            const data = await response.json();
            this.tasks = data.tasks;
            this.updateTasksDisplay();
        } catch (error) {
            console.error('Failed to load tasks:', error);
            this.showError('Failed to load tasks');
        }
    }

    updateStatsDisplay() {
        if (!this.stats) return;

        // Update stat cards
        this.updateStatCard('pending', this.stats.pending);
        this.updateStatCard('processing', this.stats.processing);
        this.updateStatCard('completed', this.stats.completed);
        this.updateStatCard('failed', this.stats.failed);
        this.updateStatCard('total', this.stats.total);

        // Update progress bars
        this.updateProgressBars();

        // Update timestamp
        document.getElementById('lastUpdated').textContent = 
            new Date().toLocaleTimeString();
    }

    updateStatCard(stat, value) {
        const element = document.getElementById(stat);
        if (element) {
            element.textContent = value.toLocaleString();
        }
    }

    updateProgressBars() {
        const total = this.stats.total || 1;
        const stats = ['pending', 'processing', 'completed', 'failed'];
        
        stats.forEach(stat => {
            const progressBar = document.getElementById(`${stat}-progress`);
            if (progressBar) {
                const value = this.stats[stat] || 0;
                const percentage = (value / total) * 100;
                progressBar.style.width = `${percentage}%`;
            }
        });
    }

    updateTasksDisplay() {
        const container = document.getElementById('tasksContainer');
        const allTasksContainer = document.getElementById('allTasksContainer');
        
        if (container) {
            container.innerHTML = this.tasks.map(task => this.createTaskRow(task)).join('');
        }
        
        if (allTasksContainer) {
            allTasksContainer.innerHTML = this.tasks.map(task => this.createTaskRow(task, true)).join('');
        }
    }

    createTaskRow(task, showProcessed = false) {
        const created = new Date(task.created_at).toLocaleString();
        const processed = task.processed_at ? 
            new Date(task.processed_at).toLocaleString() : '-';
        
        return `
            <tr>
                <td><code class="task-id">${task.id.slice(0, 8)}...</code></td>
                <td><span class="badge badge-type">${task.type}</span></td>
                <td>
                    <span class="status-badge status-${task.status}">
                        ${task.status}
                    </span>
                </td>
                <td>${task.retries} / ${task.max_retries}</td>
                <td>${created}</td>
                ${showProcessed ? `<td>${processed}</td>` : ''}
                <td>
                    <div style="display: flex; gap: 4px;">
                        <a href="/task/${task.id}" class="btn btn-sm" title="View details">
                            <i class="fas fa-eye"></i>
                        </a>
                        ${task.status === 'failed' ? `
                        <button class="btn btn-warning btn-sm retry-task" 
                                data-task-id="${task.id}" title="Retry task">
                            <i class="fas fa-redo"></i>
                        </button>
                        ` : ''}
                    </div>
                </td>
            </tr>
        `;
    }

    initCharts() {
        // Main statistics chart
        const statsCtx = document.getElementById('statsChart');
        if (statsCtx) {
            this.charts.stats = new Chart(statsCtx, {
                type: 'doughnut',
                data: {
                    labels: ['Pending', 'Processing', 'Completed', 'Failed'],
                    datasets: [{
                        data: [0, 0, 0, 0],
                        backgroundColor: [
                            '#d97706', '#2563eb', '#059669', '#dc2626'
                        ],
                        borderWidth: 0,
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    plugins: {
                        legend: {
                            position: 'bottom',
                            labels: {
                                padding: 15,
                                usePointStyle: true,
                                pointStyle: 'circle'
                            }
                        }
                    },
                    cutout: '60%'
                }
            });
        }

        // Timeline chart
        const timelineCtx = document.getElementById('timelineChart');
        if (timelineCtx) {
            this.charts.timeline = new Chart(timelineCtx, {
                type: 'line',
                data: {
                    labels: [],
                    datasets: [
                        {
                            label: 'Completed',
                            data: [],
                            borderColor: '#059669',
                            backgroundColor: 'rgba(5, 150, 105, 0.1)',
                            tension: 0.4,
                            fill: true
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        y: {
                            beginAtZero: true,
                            grid: {
                                drawBorder: false
                            }
                        },
                        x: {
                            grid: {
                                display: false
                            }
                        }
                    },
                    plugins: {
                        legend: {
                            display: false
                        }
                    }
                }
            });
        }
    }

    updateCharts() {
        if (!this.stats || !this.charts.stats) return;

        // Update main chart
        this.charts.stats.data.datasets[0].data = [
            this.stats.pending,
            this.stats.processing,
            this.stats.completed,
            this.stats.failed
        ];
        this.charts.stats.update();

        // Update timeline
        if (this.charts.timeline) {
            const now = new Date();
            const labels = Array.from({length: 6}, (_, i) => {
                const d = new Date(now - (5 - i) * 60000);
                return d.toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'});
            });

            this.charts.timeline.data.labels = labels;
            this.charts.timeline.data.datasets[0].data = 
                this.generateTimelineData(6, this.stats.completed);
            this.charts.timeline.update();
        }
    }

    generateTimelineData(count, base) {
        return Array.from({length: count}, (_, i) => {
            const variation = Math.sin(i * 0.5) * 0.3 + 0.7;
            return Math.max(0, Math.floor(base * variation));
        });
    }

    async retryTask(taskId) {
        try {
            const response = await fetch(`/api/tasks/${taskId}/retry`, {
                method: 'POST'
            });
            const result = await response.json();

            if (result.success) {
                this.showSuccess('Task queued for retry');
                this.loadStats();
                this.loadTasks();
            } else {
                this.showError('Failed to retry task');
            }
        } catch (error) {
            console.error('Failed to retry task:', error);
            this.showError('Failed to retry task');
        }
    }

    filterTasks(searchTerm) {
        const rows = document.querySelectorAll('#allTasksContainer tr');
        rows.forEach(row => {
            const text = row.textContent.toLowerCase();
            row.style.display = text.includes(searchTerm.toLowerCase()) ? '' : 'none';
        });
    }

    switchView(view) {
        this.currentView = view;
        
        // Update active tab
        document.querySelectorAll('.nav-tab').forEach(tab => {
            tab.classList.toggle('active', tab.dataset.view === view);
        });

        // Show/hide sections
        document.querySelectorAll('.view-section').forEach(section => {
            section.style.display = section.id === `${view}View` ? 'block' : 'none';
        });

        // Load specific data if needed
        if (view === 'tasks') {
            this.loadTasks();
        }
    }

    startAutoRefresh() {
        this.stopAutoRefresh();
        this.refreshInterval = setInterval(() => {
            if (this.autoRefresh) {
                this.loadStats();
                if (this.currentView === 'tasks') {
                    this.loadTasks();
                }
            }
        }, 5000);
    }

    stopAutoRefresh() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
    }

    showSuccess(message) {
        this.showNotification(message, 'success');
    }

    showError(message) {
        this.showNotification(message, 'error');
    }

    showNotification(message, type = 'info') {
        const notification = document.createElement('div');
        notification.className = `notification notification-${type}`;
        notification.innerHTML = `
            <span>${message}</span>
            <button class="notification-close">&times;</button>
        `;

        document.body.appendChild(notification);

        // Auto-remove after 3 seconds
        setTimeout(() => {
            if (notification.parentNode) {
                notification.parentNode.removeChild(notification);
            }
        }, 3000);

        // Close button
        notification.querySelector('.notification-close').addEventListener('click', () => {
            if (notification.parentNode) {
                notification.parentNode.removeChild(notification);
            }
        });
    }
}

// Initialize dashboard when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    window.dashboard = new TaskQueueDashboard();
});