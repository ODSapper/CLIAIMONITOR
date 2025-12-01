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

    [Parameter(Mandatory=$true)]
    [string]$MCPConfigPath,

    [Parameter(Mandatory=$true)]
    [string]$SystemPromptPath
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

if (-not (Test-Path $MCPConfigPath)) {
    Write-Error "MCP config path does not exist: $MCPConfigPath"
    exit 1
}

# Create a launcher script that will run in the new terminal
$launcherScript = @"
`$Host.UI.RawUI.WindowTitle = '$AgentID'
Write-Host ''
Write-Host '  ================================================' -ForegroundColor Cyan
Write-Host '    CLIAIMONITOR Agent: $AgentID' -ForegroundColor Green
Write-Host '  ================================================' -ForegroundColor Cyan
Write-Host ''
Write-Host '  Role:    $Role' -ForegroundColor Yellow
Write-Host '  Model:   $Model' -ForegroundColor Yellow
Write-Host '  Project: $ProjectPath' -ForegroundColor Yellow
Write-Host ''
Write-Host '  Starting Claude Code...' -ForegroundColor Cyan
Write-Host ''

Set-Location -Path '$ProjectPath'

# Read the system prompt from file
`$promptContent = Get-Content -Path '$SystemPromptPath' -Raw

# Launch Claude with the prompt
claude --model '$Model' --mcp-config '$MCPConfigPath' --append-system-prompt `$promptContent
"@

# Save launcher script to temp file
$tempScript = Join-Path $env:TEMP "cliaimonitor-$AgentID-launcher.ps1"
$launcherScript | Out-File -FilePath $tempScript -Encoding UTF8

# Check if Windows Terminal is available
$wtPath = Get-Command "wt.exe" -ErrorAction SilentlyContinue

if ($wtPath) {
    # Launch in Windows Terminal with new tab
    # Quote paths with spaces for proper parsing
    $wtArgs = @(
        "new-tab",
        "--title", "`"$AgentID`"",
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

# Return the temp script path for cleanup if needed
Write-Output $tempScript
