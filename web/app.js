// CLIAIMONITOR Metrics Dashboard
// Simplified metrics-only dashboard

class Dashboard {
    constructor() {
        this.ws = null;
        this.state = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.sessionStartTime = null;

        this.init();
    }

    init() {
        this.connectWebSocket();
        this.loadInitialState();
        this.loadSessionStats();
        this.loadModelMetrics();

        // Update stats every 30 seconds
        setInterval(() => {
            this.loadSessionStats();
            this.loadModelMetrics();
        }, 30000);

        // Update uptime every second
        setInterval(() => this.updateUptime(), 1000);
    }

    // WebSocket Connection
    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            console.log('[DASHBOARD] WebSocket connected');
            this.updateConnectionStatus(true);
            this.reconnectAttempts = 0;
        };

        this.ws.onclose = () => {
            console.log('[DASHBOARD] WebSocket disconnected');
            this.updateConnectionStatus(false);
            this.scheduleReconnect();
        };

        this.ws.onerror = (error) => {
            console.error('[DASHBOARD] WebSocket error:', error);
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleMessage(message);
        };
    }

    scheduleReconnect() {
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
            this.reconnectAttempts++;
            const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
            console.log(`[DASHBOARD] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
            setTimeout(() => this.connectWebSocket(), delay);
        }
    }

    updateConnectionStatus(connected) {
        const dot = document.getElementById('ws-status-dot');
        const text = document.getElementById('ws-status-text');

        if (dot) {
            dot.className = connected ? 'dot connected' : 'dot disconnected';
        }
        if (text) {
            text.textContent = connected ? 'Connected' : 'Disconnected';
        }
    }

    // Message Handling
    handleMessage(message) {
        switch (message.type) {
            case 'state_update':
                this.state = message.data;
                this.render();
                break;
            case 'metrics_update':
                this.loadModelMetrics();
                break;
        }
    }

    // API Calls
    async loadInitialState() {
        try {
            const response = await fetch('/api/state');
            this.state = await response.json();
            console.log('[DASHBOARD] Initial state loaded');
            this.render();
        } catch (error) {
            console.error('[DASHBOARD] Failed to load state:', error);
        }
    }

    async loadSessionStats() {
        try {
            const response = await fetch('/api/stats');
            const stats = await response.json();
            this.renderSessionStats(stats);
        } catch (error) {
            console.error('[DASHBOARD] Failed to load session stats:', error);
        }
    }

    async loadModelMetrics() {
        try {
            const response = await fetch('/api/metrics/by-model');
            const data = await response.json();
            this.renderModelMetrics(data.metrics || []);
        } catch (error) {
            console.error('[DASHBOARD] Failed to load model metrics:', error);
        }
    }

    // Rendering
    render() {
        if (!this.state) return;
        // Agent monitoring removed - focusing on Captain interaction and metrics only
    }

    renderModelMetrics(metrics) {
        const container = document.getElementById('model-metrics');
        if (!container) return;

        if (!metrics || metrics.length === 0) {
            container.innerHTML = '<div class="empty-state">No metrics yet</div>';
            return;
        }

        container.innerHTML = metrics.map(m => {
            const modelName = this.getShortModelName(m.model);
            return `
                <div class="model-card">
                    <div class="model-name">${this.escapeHtml(modelName)}</div>
                    <div class="model-stats">
                        <div class="model-stat">
                            <span class="stat-label">Tokens</span>
                            <span class="stat-value">${this.formatNumber(m.total_tokens || 0)}</span>
                        </div>
                        <div class="model-stat">
                            <span class="stat-label">Cost</span>
                            <span class="stat-value">$${(m.total_cost || 0).toFixed(2)}</span>
                        </div>
                        <div class="model-stat">
                            <span class="stat-label">Reports</span>
                            <span class="stat-value">${m.report_count || 0}</span>
                        </div>
                    </div>
                </div>
            `;
        }).join('');
    }

    renderSessionStats(stats) {
        // Store session start time for uptime calculation
        if (stats.session_started_at) {
            this.sessionStartTime = new Date(stats.session_started_at);
        }

        // Update session metrics panel
        const totalTokens = document.getElementById('metric-total-tokens');
        const totalCost = document.getElementById('metric-total-cost');
        const agentsSpawned = document.getElementById('metric-agents-spawned');
        const tasksCompleted = document.getElementById('metric-tasks-completed');

        if (totalTokens) totalTokens.textContent = this.formatNumber(stats.total_tokens_used || 0);
        if (totalCost) totalCost.textContent = '$' + (stats.total_estimated_cost || 0).toFixed(2);
        if (agentsSpawned) agentsSpawned.textContent = stats.total_agents_spawned || 0;
        if (tasksCompleted) tasksCompleted.textContent = stats.completed_tasks || 0;

        // Update summary bar
        const summaryTokens = document.getElementById('summary-tokens');
        const summaryCost = document.getElementById('summary-cost');

        if (summaryTokens) summaryTokens.textContent = this.formatNumber(stats.total_tokens_used || 0);
        if (summaryCost) summaryCost.textContent = '$' + (stats.total_estimated_cost || 0).toFixed(2);
    }


    updateUptime() {
        if (!this.sessionStartTime) return;

        const uptime = this.formatUptime(this.sessionStartTime);
        const summaryUptime = document.getElementById('summary-uptime');
        if (summaryUptime) summaryUptime.textContent = uptime;
    }

    // Utilities
    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    formatNumber(num) {
        if (num >= 1000000) {
            return (num / 1000000).toFixed(1) + 'M';
        } else if (num >= 1000) {
            return (num / 1000).toFixed(1) + 'K';
        }
        return num.toLocaleString();
    }


    formatUptime(startTime) {
        const now = new Date();
        const diffMs = now - startTime;
        const diffSec = Math.floor(diffMs / 1000);
        const diffMin = Math.floor(diffSec / 60);
        const diffHour = Math.floor(diffMin / 60);
        const diffDay = Math.floor(diffHour / 24);

        if (diffDay > 0) {
            const hours = diffHour % 24;
            return `${diffDay}d ${hours}h`;
        } else if (diffHour > 0) {
            const minutes = diffMin % 60;
            return `${diffHour}h ${minutes}m`;
        } else if (diffMin > 0) {
            const seconds = diffSec % 60;
            return `${diffMin}m ${seconds}s`;
        } else {
            return `${diffSec}s`;
        }
    }


    getShortModelName(model) {
        if (!model) return 'Unknown';
        // Shorten long model names
        if (model.includes('opus')) return 'Opus';
        if (model.includes('sonnet')) return 'Sonnet';
        if (model.includes('haiku')) return 'Haiku';
        // Remove common prefixes
        return model.replace('claude-', '').replace('-20241022', '').replace('-20250514', '');
    }
}

// Initialize dashboard
const dashboard = new Dashboard();
