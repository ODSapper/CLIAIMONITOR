# Dashboard Task Display Improvement

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show agent current tasks prominently instead of "disconnected" status

**Architecture:** Modify the agent card to display the current task as a primary element, with a smaller status indicator. Replace confusing "disconnected" with "working" when agent has an active task.

**Tech Stack:** HTML, CSS, JavaScript (vanilla)

---

## Current Problem

Agent cards show "disconnected" status even when agents are actively working and reporting tasks via MCP. This is confusing because:
1. "Disconnected" sounds like an error
2. The task text is crammed into a small status badge
3. Users can't see at a glance what each agent is doing

## Solution

1. Add a dedicated task display line under the agent name/role
2. Change status to "working" when there's an active task (regardless of SSE connection)
3. Show relative time since last update
4. Make task text wrap and be more readable

---

### Task 1: Update Agent Card HTML Structure

**Files:**
- Modify: `web/app.js:331-355` (renderAgents function)

**Step 1: Modify renderAgents to show task prominently**

Replace the `renderAgents` method body with:

```javascript
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
        const lastSeen = this.formatRelativeTime(agent.last_seen);

        // Determine display status: if has task, show "working" not "disconnected"
        let displayStatus = agent.status;
        let statusClass = agent.status;
        if (hasTask && agent.status === 'disconnected') {
            displayStatus = 'working';
            statusClass = 'working';
        }

        return `
            <div class="agent-card" style="--agent-color: ${agent.color}">
                <div class="agent-card-header">
                    <div>
                        <div class="agent-name">${this.escapeHtml(agent.id)}</div>
                        <div class="agent-role">${this.escapeHtml(agent.role)}</div>
                    </div>
                    <div class="agent-status-container">
                        <span class="agent-status ${statusClass}">${displayStatus}</span>
                        <span class="agent-last-seen">${lastSeen}</span>
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
                    <button class="btn btn-danger" onclick="dashboard.stopAgent('${agent.id}')">Stop</button>
                </div>
            </div>
        `;
    }).join('');
}
```

**Step 2: Run server and verify HTML renders**

Run: Open http://localhost:3000 in browser
Expected: Agent cards render with new structure (styling may be off)

**Step 3: Commit**

```bash
git add web/app.js
git commit -m "feat: update agent card to show task prominently"
```

---

### Task 2: Add CSS for Task Display

**Files:**
- Modify: `web/style.css` (add after line 276, after .agent-actions styles)

**Step 1: Add CSS for the new task display elements**

Add these styles after `.agent-actions .btn` block (around line 276):

```css
/* Agent Status Container */
.agent-status-container {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 2px;
}

.agent-last-seen {
    font-size: 0.65rem;
    color: var(--text-secondary);
}

/* Agent Current Task */
.agent-current-task {
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
    padding: 0.5rem;
    margin-top: 0.5rem;
    background: var(--bg-primary);
    border-radius: 4px;
    font-size: 0.8rem;
    min-height: 2.5rem;
}

.agent-current-task .task-icon {
    flex-shrink: 0;
}

.agent-current-task .task-text {
    color: var(--text-primary);
    line-height: 1.4;
    word-break: break-word;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
}

.agent-current-task.empty {
    opacity: 0.5;
}

.agent-current-task.empty .task-text {
    font-style: italic;
    color: var(--text-secondary);
}
```

**Step 2: Verify styling in browser**

Run: Refresh http://localhost:3000
Expected: Agent cards now show task in a distinct box with proper styling

**Step 3: Commit**

```bash
git add web/style.css
git commit -m "style: add CSS for agent task display"
```

---

### Task 3: Remove Old getAgentStatusDisplay Method

**Files:**
- Modify: `web/app.js:505-527` (remove unused method)

**Step 1: Delete getAgentStatusDisplay method**

The `getAgentStatusDisplay` method (lines 505-527) is no longer used since we handle status directly in `renderAgents`. Remove it:

```javascript
// DELETE THIS ENTIRE METHOD (lines 505-527):
// getAgentStatusDisplay(agent) {
//     ... entire method body ...
// }
```

**Step 2: Verify no errors**

Run: Refresh http://localhost:3000, open browser console
Expected: No JavaScript errors, dashboard renders correctly

**Step 3: Commit**

```bash
git add web/app.js
git commit -m "refactor: remove unused getAgentStatusDisplay method"
```

---

### Task 4: Rebuild and Test

**Files:**
- None (testing only)

**Step 1: Rebuild the Go binary**

```bash
go build -o cliaimonitor.exe ./cmd/cliaimonitor/main.go
```

**Step 2: Restart server**

```bash
./cliaimonitor.exe --no-supervisor
```

**Step 3: Spawn test agent and verify display**

Open http://localhost:3000 and spawn an agent, verify:
- Task appears in dedicated box
- Status shows "working" instead of "disconnected" when task present
- Last seen time shows correctly
- Metrics still display properly

**Step 4: Commit if any final fixes needed**

```bash
git add -A
git commit -m "test: verify dashboard task display improvements"
```

---

## Summary of Changes

| File | Change |
|------|--------|
| `web/app.js` | Updated `renderAgents()` to show task prominently, removed `getAgentStatusDisplay()` |
| `web/style.css` | Added `.agent-status-container`, `.agent-current-task` styles |

## Visual Result

Before:
```
[OpusPurple008]    [disconnected]
Go Developer
🪙 0  ❌ 0
[Stop]
```

After:
```
[OpusPurple008]    [working]
Go Developer         2m ago
📋 Analyzing existing S3 backup code to understand required fixes
🪙 0  ❌ 0
[Stop]
```
