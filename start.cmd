@echo off
REM CLIAIMONITOR Launcher - Starts the system in WezTerm with proper pane layout
REM Usage: Double-click this file or run from command line
REM
REM This script ensures everything runs inside WezTerm so:
REM - Captain spawns in a split pane below the server
REM - Agents spawn in a grid layout below Captain

REM Check if WezTerm is available
where wezterm.exe >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo ERROR: WezTerm not found in PATH
    echo Please install WezTerm from https://wezfurlong.org/wezterm/
    pause
    exit /b 1
)

REM Get the directory where this script is located
set SCRIPT_DIR=%~dp0

REM Remove trailing backslash if present
if "%SCRIPT_DIR:~-1%"=="\" set SCRIPT_DIR=%SCRIPT_DIR:~0,-1%

REM Check if already inside WezTerm
if defined WEZTERM_PANE (
    echo Already in WezTerm, starting server directly...
    cd /d "%SCRIPT_DIR%"
    bin\cliaimonitor.exe
    exit /b
)

REM Start WezTerm with the server
REM The server will detect WEZTERM_PANE and spawn Captain via split-pane
echo Starting CLIAIMONITOR in WezTerm...
wezterm.exe start --cwd "%SCRIPT_DIR%" -- cmd /k "title CLIAIMONITOR Server && bin\cliaimonitor.exe"
