// CLIAIMONITOR Dashboard Application

class Dashboard {
    constructor() {
        this.ws = null;
        this.state = null;
        this.soundEnabled = true;
        this.audioContext = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.currentView = 'dashboard';
        this.tasks = [];

        this.init();
    }

    init() {
        this.bindEvents();
        this.connectWebSocket();
        this.loadInitialState();
        this.loadProjects();
        this.loadSessionStats();
        this.loadTasks();
        // Update stats every 30 seconds
        setInterval(() => this.loadSessionStats(), 30000);
        // Update tasks every 10 seconds
        setInterval(() => this.loadTasks(), 10000);
    }

    // WebSocket Connection
    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.updateConnectionStatus(true);
            this.reconnectAttempts = 0;
        };

        this.ws.onclose = () => {
            console.log('WebSocket disconnected');
            this.updateConnectionStatus(false);
            this.scheduleReconnect();
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
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
            console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
            setTimeout(() => this.connectWebSocket(), delay);
        }
    }

    updateNATSStatus(connected) {
        const el = document.getElementById('nats-status');
        const dot = el.querySelector('.dot');
        if (connected) {
            dot.className = 'dot connected';
        } else {
            dot.className = 'dot disconnected';
        }
    }

    updateCaptainStatus(connected, status) {
        const el = document.getElementById('captain-status');
        const dot = el.querySelector('.dot');
        const text = el.querySelector('.status-text');

        if (connected) {
            dot.className = 'dot connected';
        } else {
            dot.className = 'dot disconnected';
        }

        text.textContent = status || '--';
    }

    updateAgentCount(count) {
        const el = document.getElementById('agent-count');
        const countEl = el.querySelector('.count');
        countEl.textContent = count;
    }

    // Message Handling
    handleMessage(message) {
        switch (message.type) {
            case 'state_update':
                this.state = message.data;
                this.render();
                break;
            case 'alert':
                this.handleAlert(message.data);
                break;
            case 'activity':
                this.addActivityEntry(message.data);
                break;
            case 'escalation_forward':
                this.handleEscalation(message.data);
                break;
        }
    }

    handleAlert(alert) {
        if (alert.severity === 'critical') {
            this.playAlertSound();
        }
        // Re-render to show new alert
        if (this.state) {
            this.state.alerts = this.state.alerts || [];
            this.state.alerts.unshift(alert);
            this.renderAlerts();
        }
    }

    handleEscalation(escalation) {
        // Add escalation to state
        if (this.state) {
            this.state.escalations = this.state.escalations || [];
            this.state.escalations.unshift(escalation);
            this.renderEscalations();
        }
        // Play alert sound for escalations
        this.playAlertSound();
    }

    // Sound
    playAlertSound() {
        if (!this.soundEnabled) return;

        if (!this.audioContext) {
            this.audioContext = new (window.AudioContext || window.webkitAudioContext)();
        }

        const oscillator = this.audioContext.createOscillator();
        const gainNode = this.audioContext.createGain();

        oscillator.connect(gainNode);
        gainNode.connect(this.audioContext.destination);

        oscillator.frequency.value = 800;
        oscillator.type = 'sine';

        gainNode.gain.setValueAtTime(0.3, this.audioContext.currentTime);
        gainNode.gain.exponentialRampToValueAtTime(0.01, this.audioContext.currentTime + 0.5);

        oscillator.start(this.audioContext.currentTime);
        oscillator.stop(this.audioContext.currentTime + 0.5);
    }

    // API Calls
    async loadInitialState() {
        try {
            const response = await fetch('/api/state');
            this.state = await response.json();
            this.render();
        } catch (error) {
            console.error('Failed to load state:', error);
        }
    }

    async loadProjects() {
        try {
            const response = await fetch('/api/projects');
            const data = await response.json();
            this.projects = data.projects || [];
            this.renderProjectsDropdown();
        } catch (error) {
            console.error('Failed to load projects:', error);
        }
    }

    async loadSessionStats() {
        try {
            const response = await fetch('/api/stats');
            const stats = await response.json();
            this.renderSessionStats(stats);
        } catch (error) {
            console.error('Failed to load session stats:', error);
        }
    }

    renderProjectsDropdown() {
        const select = document.getElementById('project-select');
        select.innerHTML = '<option value="">Select project...</option>' +
            this.projects.map(p => `<option value="${this.escapeHtml(p.path)}" title="${this.escapeHtml(p.description)}">${this.escapeHtml(p.name)}${p.has_claude_md ? ' (CLAUDE.md)' : ''}</option>`).join('');
    }

    renderSessionStats(stats) {
        // Calculate uptime
        const startTime = new Date(stats.session_started_at);
        const uptime = this.formatUptime(startTime);
        document.getElementById('stat-uptime').textContent = uptime;

        // Display other stats
        document.getElementById('stat-agents-spawned').textContent = stats.total_agents_spawned || 0;
        document.getElementById('stat-total-tokens').textContent = this.formatNumber(stats.total_tokens_used || 0);
        document.getElementById('stat-total-cost').textContent = '$' + (stats.total_estimated_cost || 0).toFixed(2);
        document.getElementById('stat-completed-tasks').textContent = stats.completed_tasks || 0;
    }

    async spawnAgent(configName, projectPath) {
        try {
            const response = await fetch('/api/agents/spawn', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ config_name: configName, project_path: projectPath })
            });
            return await response.json();
        } catch (error) {
            console.error('Failed to spawn agent:', error);
        }
    }

    async stopAgent(agentId) {
        try {
            await fetch(`/api/agents/${agentId}/stop`, { method: 'POST' });
        } catch (error) {
            console.error('Failed to stop agent:', error);
        }
    }

    async gracefulStopAgent(agentId) {
        try {
            await fetch(`/api/agents/${agentId}/graceful-stop`, { method: 'POST' });
        } catch (error) {
            console.error('Failed to request graceful stop:', error);
        }
    }

    async answerHumanInput(requestId, answer) {
        try {
            await fetch(`/api/human-input/${requestId}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ answer })
            });
        } catch (error) {
            console.error('Failed to submit answer:', error);
        }
    }

    async acknowledgeAlert(alertId) {
        try {
            await fetch(`/api/alerts/${alertId}/ack`, { method: 'POST' });
        } catch (error) {
            console.error('Failed to acknowledge alert:', error);
        }
    }

    async clearAllAlerts() {
        try {
            await fetch('/api/alerts/clear', { method: 'POST' });
        } catch (error) {
            console.error('Failed to clear alerts:', error);
        }
    }

    async saveThresholds(thresholds) {
        try {
            await fetch('/api/thresholds', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(thresholds)
            });
        } catch (error) {
            console.error('Failed to save thresholds:', error);
        }
    }

    async resetMetrics() {
        try {
            await fetch('/api/metrics/reset', { method: 'POST' });
        } catch (error) {
            console.error('Failed to reset metrics:', error);
        }
    }


    switchView(viewName) {
        // Hide all views
        document.querySelectorAll('.view').forEach(view => {
            view.classList.remove('active');
        });

        // Show selected view
        const view = document.getElementById(`${viewName}-view`);
        if (view) {
            view.classList.add('active');
        }

        // Update tab buttons
        document.querySelectorAll('.tab-btn').forEach(btn => {
            btn.classList.remove('active');
            if (btn.dataset.tab === viewName) {
                btn.classList.add('active');
            }
        });

        this.currentView = viewName;
    }

    // Event Binding
    bindEvents() {
        // Tab navigation
        document.querySelectorAll('.tab-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                this.switchView(btn.dataset.tab);
            });
        });

        // Mute toggle
        document.getElementById('mute-toggle').addEventListener('click', () => {
            this.soundEnabled = !this.soundEnabled;
            const btn = document.getElementById('mute-toggle');
            btn.textContent = this.soundEnabled ? '🔔' : '🔕';
            btn.classList.toggle('muted', !this.soundEnabled);
        });

        // Spawn agent
        document.getElementById('spawn-btn').addEventListener('click', () => {
            const configName = document.getElementById('agent-type-select').value;
            const projectPath = document.getElementById('project-select').value;
            const count = parseInt(document.getElementById('agent-count-select').value) || 1;
            if (configName && projectPath) {
                for (let i = 0; i < count; i++) {
                    this.spawnAgent(configName, projectPath);
                }
            }
        });

        // Agent type select
        document.getElementById('agent-type-select').addEventListener('change', () => {
            this.updateSpawnButton();
        });

        // Project select
        document.getElementById('project-select').addEventListener('change', () => {
            this.updateSpawnButton();
        });

        // Save thresholds
        document.getElementById('save-thresholds').addEventListener('click', () => {
            const thresholds = {
                failed_tests_max: parseInt(document.getElementById('threshold-failed-tests').value) || 0,
                idle_time_max_seconds: parseInt(document.getElementById('threshold-idle-time').value) || 0,
                token_usage_max: parseInt(document.getElementById('threshold-tokens').value) || 0,
                consecutive_rejects_max: parseInt(document.getElementById('threshold-rejects').value) || 0
            };
            this.saveThresholds(thresholds);
        });

        // Reset metrics
        document.getElementById('reset-metrics').addEventListener('click', () => {
            if (confirm('Reset all metrics history?')) {
                this.resetMetrics();
            }
        });

        // Clear all alerts
        document.getElementById('clear-alerts-btn').addEventListener('click', () => {
            if (confirm('Clear all alerts?')) {
                this.clearAllAlerts();
            }
        });

        // Activity filter
        document.getElementById('activity-filter').addEventListener('change', (e) => {
            this.renderActivityLog(e.target.value);
        });

        // Task modal bindings
        document.getElementById('new-task-btn')?.addEventListener('click', () => this.openTaskModal());
        document.getElementById('task-form')?.addEventListener('submit', (e) => this.createTask(e));
    }

    // Rendering
    render() {
        if (!this.state) return;

        // Update status indicators
        this.updateNATSStatus(this.state.nats_connected || false);
        this.updateCaptainStatus(this.state.captain_connected || false, this.state.captain_status || '--');
        const agentCount = Object.values(this.state.agents || {}).filter(a => a.id !== 'Supervisor').length;
        this.updateAgentCount(agentCount);

        this.renderAgents();
        this.renderAlerts();
        this.renderEscalations();
        this.renderHumanInput();
        this.renderThresholds();
        this.renderActivityLog();
        this.updateSpawnButton();
    }

    renderAgents() {
        const grid = document.getElementById('agents-grid');
        const agents = Object.values(this.state.agents || {}).filter(a => a.id !== 'Supervisor');

        if (agents.length === 0) {
            grid.innerHTML = '<div class="empty-state">No team agents running</div>';
            return;
        }

        grid.innerHTML = agents.map(agent => {
            const metrics = this.state.metrics?.[agent.id] || {};
            const hasTask = agent.current_task && agent.current_task.trim() !== '';

            // Determine display status: if has task, show "working" not "disconnected"
            let displayStatus = agent.status;
            let statusClass = agent.status;
            if (hasTask && agent.status === 'disconnected') {
                displayStatus = 'working';
                statusClass = 'working';
            }

            // NATS connection indicator
            const natsConnected = agent.status === 'connected' || agent.status === 'working';
            const natsIndicator = natsConnected ?
                '<span class="nats-indicator connected" title="NATS Connected"></span>' :
                '<span class="nats-indicator disconnected" title="NATS Disconnected"></span>';

            return `
                <div class="agent-card" style="--agent-color: ${agent.color}">
                    <div class="agent-card-header">
                        <div>
                            <div class="agent-name">${this.escapeHtml(agent.id)}</div>
                            <div class="agent-role">${this.escapeHtml(agent.role)}</div>
                        </div>
                        <div class="agent-status-container">
                            <span class="agent-status ${statusClass}">${displayStatus}</span>
                            ${natsIndicator}
                        </div>
                    </div>
                    ${hasTask ? `
                    <div class="agent-current-task" title="${this.escapeHtml(agent.current_task)}">
                        <span class="task-icon">📋</span>
                        <span class="task-text">${this.escapeHtml(agent.current_task)}</span>
                    </div>
                    ` : `
                    <div class="agent-current-task empty">
                        <span class="task-text">Waiting for task...</span>
                    </div>
                    `}
                    <div class="agent-metrics">
                        <span title="Tokens used">🪙 ${metrics.tokens_used || 0}</span>
                        <span title="Failed tests">❌ ${metrics.failed_tests || 0}</span>
                    </div>
                    <div class="agent-actions">
                        ${agent.shutdown_requested ? `
                            <span class="shutdown-countdown" data-started="${agent.shutdown_requested_at}">
                                Stopping... <span class="countdown">${this.calculateCountdown(agent.shutdown_requested_at)}</span>
                            </span>
                            <button class="btn btn-danger" onclick="dashboard.stopAgent('${agent.id}')">Force Kill</button>
                        ` : `
                            <button class="btn btn-warning" onclick="dashboard.gracefulStopAgent('${agent.id}')" title="Request graceful shutdown">
                                Stop
                            </button>
                            <button class="btn btn-danger btn-small" onclick="dashboard.stopAgent('${agent.id}')" title="Force kill immediately">
                                Kill
                            </button>
                        `}
                    </div>
                </div>
            `;
        }).join('');
    }

    renderAlerts() {
        const list = document.getElementById('alerts-list');
        const alerts = (this.state.alerts || []).filter(a => !a.acknowledged);

        document.getElementById('alert-count').textContent = alerts.length;
        document.getElementById('alert-count').setAttribute('data-count', alerts.length);

        if (alerts.length === 0) {
            list.innerHTML = '<div class="empty-state">No active alerts</div>';
            return;
        }

        list.innerHTML = alerts.map(alert => `
            <div class="alert-item ${alert.severity}">
                <div class="alert-content">
                    <div class="alert-type">${this.escapeHtml(alert.type)}${alert.agent_id ? ` - ${alert.agent_id}` : ''}</div>
                    <div class="alert-message">${this.escapeHtml(alert.message)}</div>
                    <div class="alert-time">${this.formatTime(alert.created_at)}</div>
                </div>
                <div class="alert-actions">
                    <button class="btn btn-icon" onclick="dashboard.acknowledgeAlert('${alert.id}')" title="Acknowledge">✓</button>
                </div>
            </div>
        `).join('');
    }

    renderHumanInput() {
        const list = document.getElementById('human-input-list');
        const requests = Object.values(this.state.human_requests || {}).filter(r => !r.answered);

        document.getElementById('human-input-count').textContent = requests.length;
        document.getElementById('human-input-count').setAttribute('data-count', requests.length);

        if (requests.length === 0) {
            list.innerHTML = '<div class="empty-state">No pending requests</div>';
            return;
        }

        list.innerHTML = requests.map(req => `
            <div class="human-input-item ${Date.now() - new Date(req.created_at).getTime() > 300000 ? 'urgent' : ''}">
                <div class="human-input-header">
                    <span class="human-input-agent">${this.escapeHtml(req.agent_id)}</span>
                    <span class="human-input-time">${this.formatTime(req.created_at)}</span>
                </div>
                <div class="human-input-question">${this.escapeHtml(req.question)}</div>
                ${req.context ? `<div class="human-input-context">${this.escapeHtml(req.context)}</div>` : ''}
                <div class="human-input-response">
                    <input type="text" id="answer-${req.id}" placeholder="Type your answer..." onkeypress="if(event.key==='Enter')dashboard.submitAnswer('${req.id}')">
                    <button class="btn btn-primary" onclick="dashboard.submitAnswer('${req.id}')">Send</button>
                </div>
            </div>
        `).join('');
    }

    submitAnswer(requestId) {
        const input = document.getElementById(`answer-${requestId}`);
        if (input && input.value.trim()) {
            this.answerHumanInput(requestId, input.value.trim());
        }
    }

    renderEscalations() {
        const list = document.getElementById('escalations-list');
        const escalations = this.state.escalations || [];
        const pending = escalations.filter(e => !e.responded);

        document.getElementById('escalation-count').textContent = pending.length;
        document.getElementById('escalation-count').setAttribute('data-count', pending.length);

        if (pending.length === 0) {
            list.innerHTML = '<div class="empty-state">No pending escalations</div>';
            return;
        }

        list.innerHTML = pending.map(esc => `
            <div class="escalation-card">
                <div class="escalation-header">
                    <span class="escalation-agent">${this.escapeHtml(esc.agent_id)}</span>
                    <span class="escalation-time">${this.formatTime(esc.timestamp)}</span>
                </div>
                <div class="escalation-question">${this.escapeHtml(esc.question)}</div>
                ${esc.captain_context ? `<div class="escalation-context">Captain: ${this.escapeHtml(esc.captain_context)}</div>` : ''}
                ${esc.captain_recommends ? `<div class="escalation-context">Recommends: ${this.escapeHtml(esc.captain_recommends)}</div>` : ''}
                <div class="escalation-response">
                    <input type="text" id="escalation-response-${esc.id}" placeholder="Type your response..." onkeypress="if(event.key==='Enter')dashboard.submitEscalationResponse('${esc.id}')">
                    <button class="btn btn-primary" onclick="dashboard.submitEscalationResponse('${esc.id}')">Send</button>
                </div>
            </div>
        `).join('');
    }

    submitEscalationResponse(escalationId) {
        const input = document.getElementById(`escalation-response-${escalationId}`);
        if (input && input.value.trim()) {
            const response = input.value.trim();
            this.sendEscalationResponse(escalationId, response);
        }
    }

    async sendEscalationResponse(escalationId, response) {
        try {
            await fetch(`/api/escalation/${escalationId}/respond`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ response })
            });
            // Remove from UI on success
            if (this.state && this.state.escalations) {
                const index = this.state.escalations.findIndex(e => e.id === escalationId);
                if (index !== -1) {
                    this.state.escalations[index].responded = true;
                    this.renderEscalations();
                }
            }
        } catch (error) {
            console.error('Failed to submit escalation response:', error);
        }
    }

    renderThresholds() {
        const t = this.state.thresholds || {};
        document.getElementById('threshold-failed-tests').value = t.failed_tests_max || 5;
        document.getElementById('threshold-idle-time').value = t.idle_time_max_seconds || 600;
        document.getElementById('threshold-tokens').value = t.token_usage_max || 100000;
        document.getElementById('threshold-rejects').value = t.consecutive_rejects_max || 3;
    }

    renderActivityLog(filterAgent = '') {
        const log = document.getElementById('activity-log');
        let activities = this.state.activity_log || [];

        // Update filter options
        const filter = document.getElementById('activity-filter');
        const agents = [...new Set(activities.map(a => a.agent_id))];
        const currentValue = filter.value;
        filter.innerHTML = '<option value="">All Agents</option>' +
            agents.map(a => `<option value="${a}" ${a === currentValue ? 'selected' : ''}>${a}</option>`).join('');

        // Apply filter
        if (filterAgent) {
            activities = activities.filter(a => a.agent_id === filterAgent);
        }

        // Show latest 100
        activities = activities.slice(-100).reverse();

        if (activities.length === 0) {
            log.innerHTML = '<div class="empty-state">No activity</div>';
            return;
        }

        log.innerHTML = activities.map(entry => `
            <div class="activity-entry">
                <span class="activity-time">${this.formatTime(entry.timestamp)}</span>
                <span class="activity-agent">${this.escapeHtml(entry.agent_id)}</span>
                <span class="activity-action">${this.escapeHtml(entry.action)}</span>
                <span class="activity-details">${this.escapeHtml(entry.details || '')}</span>
            </div>
        `).join('');
    }

    addActivityEntry(activity) {
        if (!this.state) return;
        this.state.activity_log = this.state.activity_log || [];
        this.state.activity_log.push(activity);
        this.renderActivityLog(document.getElementById('activity-filter').value);
    }

    updateSpawnButton() {
        const btn = document.getElementById('spawn-btn');
        const agentSelect = document.getElementById('agent-type-select');
        const projectSelect = document.getElementById('project-select');
        btn.disabled = !agentSelect.value || !projectSelect.value;
    }

    // Utilities
    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    formatTime(timestamp) {
        if (!timestamp) return '';
        const date = new Date(timestamp);
        return date.toLocaleTimeString('en-US', { hour12: false });
    }

    formatRelativeTime(timestamp) {
        if (!timestamp) return 'never';
        const date = new Date(timestamp);
        const now = new Date();
        const diffMs = now - date;
        const diffSec = Math.floor(diffMs / 1000);
        const diffMin = Math.floor(diffSec / 60);
        const diffHour = Math.floor(diffMin / 60);

        if (diffSec < 10) return 'just now';
        if (diffSec < 60) return `${diffSec}s ago`;
        if (diffMin < 60) return `${diffMin}m ago`;
        if (diffHour < 24) return `${diffHour}h ago`;
        return date.toLocaleDateString();
    }

    calculateCountdown(shutdownRequestedAt) {
        if (!shutdownRequestedAt) return '60s';
        const requestedTime = new Date(shutdownRequestedAt);
        const now = new Date();
        const elapsedMs = now - requestedTime;
        const remainingMs = Math.max(0, 60000 - elapsedMs);
        const remainingSec = Math.ceil(remainingMs / 1000);
        return `${remainingSec}s`;
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

    formatNumber(num) {
        if (num >= 1000000) {
            return (num / 1000000).toFixed(1) + 'M';
        } else if (num >= 1000) {
            return (num / 1000).toFixed(1) + 'K';
        }
        return num.toString();
    }

    // Agent-centric dashboard functions

    // Render agent cards
    renderAgentCards() {
        const container = document.getElementById('agent-cards');
        if (!container) return;

        if (!this.state || !this.state.agents) {
            container.innerHTML = '<p class="empty-state">No agents connected</p>';
            return;
        }

        const agents = Object.values(this.state.agents);
        if (agents.length === 0) {
            container.innerHTML = '<p class="empty-state">No agents connected</p>';
            return;
        }

        container.innerHTML = agents.map(agent => {
            const agentTasks = this.getAgentTasks(agent.id);
            const currentTask = agentTasks.find(t => t.status === 'in_progress');
            const queuedTasks = agentTasks.filter(t => t.status === 'assigned');

            return `
                <div class="agent-card ${agent.status}" style="border-color: ${agent.color}">
                    <div class="agent-header">
                        <span class="agent-status-dot" style="background: ${this.getStatusColor(agent.status)}"></span>
                        <span class="agent-name">${this.escapeHtml(agent.config_name || agent.id)}</span>
                        <span class="agent-role">${this.escapeHtml(agent.role || '')}</span>
                    </div>
                    <div class="agent-current-task">
                        ${currentTask ? `
                            <div class="current-task">
                                <span class="task-indicator">▶</span>
                                <span class="task-id">${this.escapeHtml(currentTask.id)}</span>
                                <span class="task-title">${this.escapeHtml(currentTask.title)}</span>
                                <span class="task-time">${this.formatDuration(currentTask.started_at)}</span>
                            </div>
                        ` : `<span class="idle-state">idle</span>`}
                    </div>
                    <div class="agent-queue">
                        ${queuedTasks.slice(0, 3).map(t => `
                            <div class="queued-task">
                                <span class="queue-indicator">◦</span>
                                <span class="task-id">${this.escapeHtml(t.id)}</span>
                            </div>
                        `).join('')}
                        ${queuedTasks.length > 3 ? `<span class="more-tasks">+${queuedTasks.length - 3} more</span>` : ''}
                    </div>
                </div>
            `;
        }).join('');
    }

    getAgentTasks(agentId) {
        if (!this.tasks) return [];
        return this.tasks.filter(t => t.assigned_to === agentId);
    }

    getStatusColor(status) {
        const colors = {
            'connected': '#00cc66',
            'working': '#00cc66',
            'idle': '#999',
            'blocked': '#ff9900',
            'disconnected': '#cc3333',
            'error': '#cc3333'
        };
        return colors[status] || '#999';
    }

    formatDuration(startTime) {
        if (!startTime) return '';
        const start = new Date(startTime);
        const now = new Date();
        const diff = Math.floor((now - start) / 1000);

        if (diff < 60) return `${diff}s`;
        if (diff < 3600) return `${Math.floor(diff / 60)}m`;
        return `${Math.floor(diff / 3600)}h ${Math.floor((diff % 3600) / 60)}m`;
    }

    // Load and render tasks
    async loadTasks() {
        try {
            const response = await fetch('/api/tasks');
            const data = await response.json();
            this.tasks = data.tasks || [];
            this.renderPendingQueue();
            this.renderAgentCards();
            this.updateSummary();
        } catch (error) {
            console.error('Failed to load tasks:', error);
        }
    }

    renderPendingQueue() {
        const tbody = document.getElementById('pending-queue-body');
        if (!tbody) return;

        const pending = (this.tasks || []).filter(t => t.status === 'pending');

        const pendingCount = document.getElementById('pending-count');
        if (pendingCount) pendingCount.textContent = pending.length;

        if (pending.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" class="empty-state">No pending tasks</td></tr>';
            return;
        }

        tbody.innerHTML = pending.map(task => `
            <tr>
                <td><span class="priority-badge p${task.priority}">P${task.priority}</span></td>
                <td>${this.escapeHtml(task.title)}</td>
                <td>${this.escapeHtml(task.repo || 'local')}</td>
                <td>${task.status}</td>
                <td>${this.formatAge(task.created_at)}</td>
                <td>
                    <button class="btn btn-small" onclick="dashboard.assignTask('${task.id}')">Assign</button>
                </td>
            </tr>
        `).join('');
    }

    formatAge(timestamp) {
        const created = new Date(timestamp);
        const now = new Date();
        const diff = Math.floor((now - created) / 1000);

        if (diff < 60) return 'just now';
        if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
        if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
        return `${Math.floor(diff / 86400)}d ago`;
    }

    updateSummary() {
        const tasks = this.tasks || [];
        const agents = this.state?.agents ? Object.values(this.state.agents) : [];

        const summaryActive = document.getElementById('summary-active');
        const summaryPending = document.getElementById('summary-pending');
        const summaryReview = document.getElementById('summary-review');
        const summaryTokens = document.getElementById('summary-tokens');
        const summaryCost = document.getElementById('summary-cost');

        if (summaryActive) summaryActive.textContent = agents.filter(a => a.status === 'working').length;
        if (summaryPending) summaryPending.textContent = tasks.filter(t => t.status === 'pending').length;
        if (summaryReview) summaryReview.textContent = tasks.filter(t => t.status === 'review').length;

        // Token/cost from session stats
        const stats = this.state?.session_stats || {};
        if (summaryTokens) summaryTokens.textContent = this.formatNumberWithCommas(stats.total_tokens_used || 0);
        if (summaryCost) summaryCost.textContent = `$${(stats.total_estimated_cost || 0).toFixed(2)}`;
    }

    formatNumberWithCommas(n) {
        return n.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',');
    }

    // Task modal
    openTaskModal() {
        document.getElementById('task-modal').style.display = 'flex';
    }

    closeTaskModal() {
        document.getElementById('task-modal').style.display = 'none';
        document.getElementById('task-form').reset();
    }

    async createTask(e) {
        e.preventDefault();

        const task = {
            title: document.getElementById('task-title').value,
            description: document.getElementById('task-description').value,
            priority: parseInt(document.getElementById('task-priority').value),
            repo: document.getElementById('task-repo').value
        };

        try {
            const response = await fetch('/api/tasks', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(task)
            });

            if (response.ok) {
                this.closeTaskModal();
                this.loadTasks();
            } else {
                alert('Failed to create task');
            }
        } catch (error) {
            console.error('Create task error:', error);
            alert('Failed to create task');
        }
    }

}

// Initialize
const dashboard = new Dashboard();

// Make closeTaskModal global for onclick
window.closeTaskModal = () => dashboard.closeTaskModal();

// ============================================================
// Notification Banner Controller
// ============================================================
class NotificationBanner {
    constructor() {
        this.banner = document.getElementById('notification-banner');
        this.message = document.getElementById('notification-message');
        this.dismissBtn = document.getElementById('notification-dismiss');
        this.currentTimeout = null;

        this.initEventListeners();
        console.log('[NOTIFICATION] Banner controller initialized');
    }

    initEventListeners() {
        // Dismiss button click handler
        this.dismissBtn.addEventListener('click', () => {
            this.hide();
        });

        // Listen for notification events
        window.addEventListener('supervisor-needs-input', (event) => {
            const msg = event.detail?.message || 'Supervisor needs your input';
            this.show(msg, 'supervisor', false);
        });

        window.addEventListener('notification', (event) => {
            const { message: msg, type = 'info', autoHide = true } = event.detail || {};
            if (msg) {
                this.show(msg, type, autoHide);
            }
        });
    }

    show(text, type = 'info', autoHide = false) {
        // Clear any existing timeout
        if (this.currentTimeout) {
            clearTimeout(this.currentTimeout);
            this.currentTimeout = null;
        }

        // Set message
        this.message.textContent = text;

        // Set type (info, warning, error, supervisor)
        this.banner.className = 'notification-banner ' + type;

        // Show banner
        this.banner.style.display = 'block';
        document.body.classList.add('notification-active');

        // Auto-hide after 10 seconds for non-supervisor alerts
        if (autoHide && type !== 'supervisor') {
            this.currentTimeout = setTimeout(() => {
                this.hide();
            }, 10000);
        }

        console.log('[NOTIFICATION] Banner shown:', text, 'Type:', type);
    }

    hide() {
        this.banner.style.display = 'none';
        document.body.classList.remove('notification-active');

        if (this.currentTimeout) {
            clearTimeout(this.currentTimeout);
            this.currentTimeout = null;
        }

        console.log('[NOTIFICATION] Banner hidden');
    }

    // Public API
    showInfo(message, autoHide = true) {
        this.show(message, 'info', autoHide);
    }

    showWarning(message, autoHide = true) {
        this.show(message, 'warning', autoHide);
    }

    showError(message, autoHide = true) {
        this.show(message, 'error', autoHide);
    }

    showSupervisorAlert(message) {
        this.show(message, 'supervisor', false);
    }

    clear() {
        this.hide();
    }
}

// Initialize notification banner
const notificationBanner = new NotificationBanner();

// Export to global scope for easy access
window.notificationBanner = notificationBanner;
