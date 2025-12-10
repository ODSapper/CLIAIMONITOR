/**
 * MetricsDashboard
 * Real-time metrics and cost tracking dashboard
 */

class MetricsDashboard {
    constructor() {
        this.ws = null;
        this.reconnectInterval = null;
        this.metrics = {
            byModel: {},
            byAgent: {},
            pipeline: {
                pending: 0,
                green: 0,
                purple: 0,
                complete: 0
            }
        };

        this.init();
    }

    async init() {
        console.log('[METRICS] Initializing metrics dashboard...');

        // Fetch initial metrics
        await this.fetchMetrics();

        // Connect WebSocket for real-time updates
        this.connectWebSocket();

        // Update status indicators
        this.updateConnectionStatus();

        // Poll for updates every 30 seconds
        setInterval(() => this.fetchMetrics(), 30000);
    }

    async fetchMetrics() {
        try {
            console.log('[METRICS] Fetching metrics from API...');

            // Fetch all three endpoints in parallel
            const [modelRes, agentTypeRes, agentRes] = await Promise.all([
                fetch('/api/metrics/by-model'),
                fetch('/api/metrics/by-agent-type'),
                fetch('/api/metrics/by-agent')
            ]);

            if (!modelRes.ok || !agentTypeRes.ok || !agentRes.ok) {
                throw new Error(`HTTP error: model=${modelRes.status}, agentType=${agentTypeRes.status}, agent=${agentRes.status}`);
            }

            const [modelData, agentTypeData, agentData] = await Promise.all([
                modelRes.json(),
                agentTypeRes.json(),
                agentRes.json()
            ]);

            console.log('[METRICS] Received metrics:', { modelData, agentTypeData, agentData });

            this.metrics.byModel = modelData.metrics || [];
            this.metrics.byAgentType = agentTypeData.metrics || [];
            this.metrics.byAgent = agentData.metrics || [];

            // Render all sections
            this.renderModelMetrics();
            this.renderAgentTypeMetrics();
            this.renderAgentMetrics();

        } catch (error) {
            console.error('[METRICS] Failed to fetch metrics:', error);
        }
    }

    renderModelMetrics() {
        const grid = document.getElementById('model-costs-grid');
        if (!grid) return;

        const models = this.metrics.byModel;

        if (!models || models.length === 0) {
            grid.innerHTML = '<div class="empty-state">No metrics available yet</div>';
            return;
        }

        // Sort models by cost (descending)
        models.sort((a, b) => (b.total_cost || 0) - (a.total_cost || 0));

        grid.innerHTML = models.map(metrics => {
            const modelType = this.formatModelName(metrics.model);

            return `
                <div class="model-cost-card ${modelType}">
                    <div class="model-name">${this.capitalizeFirst(modelType)}</div>
                    <div class="cost-stats">
                        <div class="cost-stat">
                            <span class="cost-label">Reports:</span>
                            <span class="cost-value">${metrics.report_count || 0}</span>
                        </div>
                        <div class="cost-stat">
                            <span class="cost-label">Total Tokens:</span>
                            <span class="cost-value">${this.formatTokens(metrics.total_tokens || 0)}</span>
                        </div>
                        <div class="cost-stat">
                            <span class="cost-label">Avg. Tokens/Report:</span>
                            <span class="cost-value">${this.formatTokens(metrics.avg_tokens_per_report || 0)}</span>
                        </div>
                        <div class="cost-stat" style="margin-top: 0.5rem; padding-top: 0.5rem; border-top: 1px solid var(--border-color);">
                            <span class="cost-label">Estimated Cost:</span>
                            <span class="cost-value total">$${(metrics.total_cost || 0).toFixed(2)}</span>
                        </div>
                    </div>
                </div>
            `;
        }).join('');
    }

    renderAgentTypeMetrics() {
        const grid = document.getElementById('agent-type-grid');
        if (!grid) return;

        const agentTypes = this.metrics.byAgentType;

        if (!agentTypes || agentTypes.length === 0) {
            grid.innerHTML = '<div class="empty-state">No agent type metrics available yet</div>';
            return;
        }

        // Sort by cost (descending)
        agentTypes.sort((a, b) => (b.total_cost || 0) - (a.total_cost || 0));

        // Calculate total for percentages
        const totalCost = agentTypes.reduce((sum, m) => sum + (m.total_cost || 0), 0);

        grid.innerHTML = agentTypes.map(metrics => {
            const percentage = totalCost > 0 ? ((metrics.total_cost / totalCost) * 100).toFixed(1) : 0;
            const typeLabel = this.formatAgentType(metrics.agent_type);
            const typeClass = metrics.agent_type || 'unknown';

            return `
                <div class="agent-type-card ${typeClass}">
                    <div class="agent-type-header">
                        <span class="agent-type-name">${typeLabel}</span>
                        <span class="agent-type-percent">${percentage}%</span>
                    </div>
                    <div class="agent-type-stats">
                        <div class="stat-row">
                            <span class="stat-label">Agents:</span>
                            <span class="stat-value">${metrics.agent_count || 0}</span>
                        </div>
                        <div class="stat-row">
                            <span class="stat-label">Reports:</span>
                            <span class="stat-value">${metrics.report_count || 0}</span>
                        </div>
                        <div class="stat-row">
                            <span class="stat-label">Tokens:</span>
                            <span class="stat-value">${this.formatTokens(metrics.total_tokens || 0)}</span>
                        </div>
                        <div class="stat-row total">
                            <span class="stat-label">Cost:</span>
                            <span class="stat-value">$${(metrics.total_cost || 0).toFixed(2)}</span>
                        </div>
                    </div>
                </div>
            `;
        }).join('');
    }

    renderAgentMetrics() {
        const tbody = document.getElementById('agent-performance-body');
        if (!tbody) return;

        const agents = this.metrics.byAgent;

        if (!agents || agents.length === 0) {
            tbody.innerHTML = '<tr><td colspan="8" class="empty-state">No agent data available yet</td></tr>';
            return;
        }

        // Sort agents by total cost (descending)
        agents.sort((a, b) => (b.total_cost || 0) - (a.total_cost || 0));

        tbody.innerHTML = agents.map(metrics => {
            const modelType = this.formatModelName(metrics.model || 'unknown');
            const agentTypeLabel = this.formatAgentType(metrics.agent_type);
            const avgTokens = metrics.report_count > 0 ? Math.round(metrics.total_tokens / metrics.report_count) : 0;

            return `
                <tr>
                    <td>${metrics.agent_id}</td>
                    <td><span class="type-badge ${metrics.agent_type}">${agentTypeLabel}</span></td>
                    <td><span class="model-badge ${modelType}">${this.capitalizeFirst(modelType)}</span></td>
                    <td>${metrics.parent_agent || '-'}</td>
                    <td>${metrics.report_count || 0}</td>
                    <td>${this.formatTokens(metrics.total_tokens || 0)}</td>
                    <td>$${(metrics.total_cost || 0).toFixed(2)}</td>
                    <td>${this.formatTokens(avgTokens)}</td>
                </tr>
            `;
        }).join('');
    }

    updatePipelineCounts(counts) {
        // Update pipeline visualization
        document.getElementById('pipeline-pending').textContent = counts.pending || 0;
        document.getElementById('pipeline-green').textContent = counts.green || 0;
        document.getElementById('pipeline-purple').textContent = counts.purple || 0;
        document.getElementById('pipeline-complete').textContent = counts.complete || 0;
    }

    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        console.log('[METRICS] Connecting to WebSocket:', wsUrl);

        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
            console.log('[METRICS] WebSocket connected');
            this.updateConnectionStatus();

            if (this.reconnectInterval) {
                clearInterval(this.reconnectInterval);
                this.reconnectInterval = null;
            }
        };

        this.ws.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                this.handleWebSocketMessage(data);
            } catch (error) {
                console.error('[METRICS] Failed to parse WebSocket message:', error);
            }
        };

        this.ws.onerror = (error) => {
            console.error('[METRICS] WebSocket error:', error);
        };

        this.ws.onclose = () => {
            console.log('[METRICS] WebSocket disconnected');
            this.updateConnectionStatus();

            // Attempt to reconnect
            if (!this.reconnectInterval) {
                this.reconnectInterval = setInterval(() => {
                    console.log('[METRICS] Attempting to reconnect WebSocket...');
                    this.connectWebSocket();
                }, 5000);
            }
        };
    }

    handleWebSocketMessage(data) {
        console.log('[METRICS] WebSocket message:', data.type);

        switch (data.type) {
            case 'state_update':
                // Refresh metrics when state updates
                this.fetchMetrics();
                break;

            case 'agent_registered':
            case 'agent_heartbeat':
            case 'task_completed':
                // Refresh metrics on relevant events
                this.fetchMetrics();
                break;

            case 'nats_connected':
            case 'nats_disconnected':
                this.updateConnectionStatus();
                break;
        }
    }

    updateConnectionStatus() {
        // Update NATS status
        const natsStatus = document.getElementById('nats-status');
        const natsDot = natsStatus?.querySelector('.dot');

        // Update Captain status
        const captainStatus = document.getElementById('captain-status');
        const captainDot = captainStatus?.querySelector('.dot');
        const captainText = captainStatus?.querySelector('.status-text');

        // Check connection status
        const isConnected = this.ws && this.ws.readyState === WebSocket.OPEN;

        if (natsDot) {
            natsDot.classList.toggle('connected', isConnected);
            natsDot.classList.toggle('disconnected', !isConnected);
        }

        if (captainDot) {
            captainDot.classList.toggle('connected', isConnected);
            captainDot.classList.toggle('disconnected', !isConnected);
        }

        if (captainText) {
            captainText.textContent = isConnected ? 'Online' : 'Offline';
        }
    }

    // Utility functions

    formatModelName(model) {
        if (!model) return 'unknown';

        const lower = model.toLowerCase();
        if (lower.includes('opus')) return 'opus';
        if (lower.includes('sonnet')) return 'sonnet';
        if (lower.includes('haiku')) return 'haiku';
        return 'unknown';
    }

    formatAgentType(agentType) {
        if (!agentType) return 'Unknown';

        const labels = {
            'captain': 'Captain',
            'sgt': 'SGT',
            'spawned_window': 'Spawned',
            'subagent': 'Sub-Agent'
        };
        return labels[agentType] || this.capitalizeFirst(agentType);
    }

    formatTokens(tokens) {
        if (tokens >= 1000000) {
            return `${(tokens / 1000000).toFixed(2)}M`;
        } else if (tokens >= 1000) {
            return `${(tokens / 1000).toFixed(1)}K`;
        }
        return tokens.toString();
    }

    capitalizeFirst(str) {
        if (!str) return '';
        return str.charAt(0).toUpperCase() + str.slice(1);
    }
}

// Initialize dashboard when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        console.log('[METRICS] DOM loaded, initializing dashboard...');
        new MetricsDashboard();
    });
} else {
    console.log('[METRICS] DOM already loaded, initializing dashboard...');
    new MetricsDashboard();
}
