param(
    [Parameter(Mandatory=$true)]
    [string]$AgentID,

    [Parameter(Mandatory=$true)]
    [string]$AgentName,

    [Parameter(Mandatory=$true)]
    [string]$Model,

    [Parameter(Mandatory=$true)]
    [string]$Role,

    [Parameter(Mandatory=$true)]
    [string]$Color,

    [Parameter(Mandatory=$true)]
    [string]$ProjectPath,

    [Parameter(Mandatory=$false)]
    [string]$MCPConfigPath = "",

    [Parameter(Mandatory=$true)]
    [string]$SystemPromptPath,

    [Parameter(Mandatory=$false)]
    [switch]$SkipPermissions,

    [Parameter(Mandatory=$false)]
    [string]$InitialPrompt = "",

    [Parameter(Mandatory=$false)]
    [switch]$NoMCP
)

# Verify paths exist
if (-not (Test-Path $ProjectPath)) {
    Write-Error "Project path does not exist: $ProjectPath"
    exit 1
}

if (-not (Test-Path $SystemPromptPath)) {
    Write-Error "System prompt path does not exist: $SystemPromptPath"
    exit 1
}

if (-not $NoMCP -and -not (Test-Path $MCPConfigPath)) {
    Write-Error "MCP config path does not exist: $MCPConfigPath"
    exit 1
}

# Build the Claude command with optional flags
$skipPermissionsFlag = if ($SkipPermissions) { " --dangerously-skip-permissions" } else { "" }
$mcpConfigFlag = if ($NoMCP) { "" } else { " --mcp-config '$MCPConfigPath'" }
# Note: Initial prompt is passed as positional argument, NOT with -p flag
# The -p flag means "print and exit" which is not what we want for interactive agents

# Create a launcher script that will run in the new terminal
# Use unique window title format for reliable process identification
$launcherScript = @"
# Set unique window title for process tracking (CLIAIMONITOR-{AgentID})
`$Host.UI.RawUI.WindowTitle = 'CLIAIMONITOR-$AgentID'

# Write PID to tracking file for reliable termination
`$pidDir = 'C:\Users\Admin\Documents\VS Projects\CLIAIMONITOR\data\pids'
if (-not (Test-Path `$pidDir)) { New-Item -ItemType Directory -Path `$pidDir -Force | Out-Null }
`$PID | Out-File -FilePath (Join-Path `$pidDir '$AgentID.pid') -Encoding ASCII -NoNewline

Write-Host ''
Write-Host '  ================================================' -ForegroundColor Cyan
Write-Host '    CLIAIMONITOR Agent: $AgentID' -ForegroundColor Green
Write-Host '  ================================================' -ForegroundColor Cyan
Write-Host ''
Write-Host '  Role:    $Role' -ForegroundColor Yellow
Write-Host '  Model:   $Model' -ForegroundColor Yellow
Write-Host '  Project: $ProjectPath' -ForegroundColor Yellow
Write-Host "  PID:     `$PID" -ForegroundColor DarkGray
Write-Host ''
Write-Host '  Starting Claude Code...' -ForegroundColor Cyan
Write-Host ''

Set-Location -Path '$ProjectPath'

# Read system prompt from file
`$systemPromptContent = Get-Content -Path '$SystemPromptPath' -Raw -ErrorAction SilentlyContinue
if (`$systemPromptContent) {
    Write-Host '  System prompt loaded' -ForegroundColor DarkGray
    Write-Host "  Length: `$(`$systemPromptContent.Length) chars" -ForegroundColor DarkGray
} else {
    Write-Host '  WARNING: Could not read system prompt!' -ForegroundColor Red
    Write-Host '  Path: $SystemPromptPath' -ForegroundColor Red
}

# Launch Claude with MCP config
Write-Host '  Launching Claude...' -ForegroundColor Green
claude --model '$Model' --mcp-config '$MCPConfigPath' --strict-mcp-config --dangerously-skip-permissions "$InitialPrompt"

if (`$LASTEXITCODE -ne 0) {
    Write-Host "  ERROR: Claude exited with code `$LASTEXITCODE" -ForegroundColor Red
    Write-Host "  Press any key to close..." -ForegroundColor Yellow
    `$null = `$Host.UI.RawUI.ReadKey('NoEcho,IncludeKeyDown')
}
"@

# Save launcher script to temp file
$tempScript = Join-Path $env:TEMP "cliaimonitor-$AgentID-launcher.ps1"
$launcherScript | Out-File -FilePath $tempScript -Encoding UTF8

# Check if Windows Terminal is available
$wtPath = Get-Command "wt.exe" -ErrorAction SilentlyContinue

if ($wtPath) {
    # Launch in Windows Terminal with new tab
    # Use CLIAIMONITOR-{AgentID} format for reliable process tracking
    # The launcher script will set the actual window title to this format
    $wtArgs = @(
        "new-tab",
        "--title", "`"CLIAIMONITOR-$AgentID`"",
        "--tabColor", $Color,
        "-d", "`"$ProjectPath`"",
        "powershell.exe", "-NoExit", "-ExecutionPolicy", "Bypass", "-File", "`"$tempScript`""
    )

    Start-Process "wt.exe" -ArgumentList $wtArgs
    Write-Host "Agent $AgentID launched in Windows Terminal" -ForegroundColor Green
} else {
    # Fallback: launch in new PowerShell window
    Write-Host "Windows Terminal not found, using PowerShell window..." -ForegroundColor Yellow

    Start-Process "powershell.exe" -ArgumentList @(
        "-NoExit",
        "-ExecutionPolicy", "Bypass",
        "-File", "`"$tempScript`""
    ) -WorkingDirectory $ProjectPath

    Write-Host "Agent $AgentID launched in PowerShell" -ForegroundColor Green
}

# Spawn heartbeat script as hidden background process
$heartbeatScriptPath = Join-Path $PSScriptRoot "agent-heartbeat.ps1"

if (Test-Path $heartbeatScriptPath) {
    # Start heartbeat monitor in hidden window
    $heartbeatProcess = Start-Process -FilePath "powershell.exe" `
        -ArgumentList @(
            "-ExecutionPolicy", "Bypass",
            "-WindowStyle", "Hidden",
            "-File", "`"$heartbeatScriptPath`"",
            "-AgentID", "`"$AgentID`"",
            "-ServerURL", "http://localhost:3000",
            "-IntervalSeconds", "15",
            "-CurrentTask", "initializing"
        ) `
        -WindowStyle Hidden `
        -PassThru

    Write-Host "Heartbeat monitor started (PID: $($heartbeatProcess.Id))" -ForegroundColor Green

    # Store heartbeat PID for tracking
    $heartbeatPidPath = Join-Path "C:\Users\Admin\Documents\VS Projects\CLIAIMONITOR\data\pids" "$AgentID-heartbeat.pid"
    $heartbeatProcess.Id | Out-File -FilePath $heartbeatPidPath -Encoding ASCII -NoNewline
} else {
    Write-Host "WARNING: Heartbeat script not found at $heartbeatScriptPath" -ForegroundColor Yellow
}

# Return the temp script path for cleanup if needed
Write-Output $tempScript
