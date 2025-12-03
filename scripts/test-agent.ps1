# Simple test - Claude without MCP
Write-Host "Starting Claude WITHOUT MCP config..."
Write-Host "Project: C:\Users\Admin\Documents\VS Projects\MSS"
Set-Location "C:\Users\Admin\Documents\VS Projects\MSS"

# Test 1: Just Claude with --dangerously-skip-permissions
claude --dangerously-skip-permissions "Say hello and confirm you're working"
