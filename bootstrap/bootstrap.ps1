#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Bootstrap CLIAIMONITOR Captain in infrastructure-poor environment

.DESCRIPTION
    Initializes Captain with minimal state for deployment to customer sites,
    new networks, or any environment without full CLIAIMONITOR infrastructure.

.PARAMETER EnvironmentName
    Name of the environment (e.g., "ACME Corp", "Customer Alpha")

.PARAMETER EnvironmentType
    Type of environment: customer, internal, test

.PARAMETER EnvironmentID
    Unique ID for environment (auto-generated if not provided)

.PARAMETER CaptainID
    Unique ID for this Captain instance (auto-generated if not provided)

.PARAMETER DataDir
    Directory for bootstrap data (default: ./bootstrap-data)

.PARAMETER EnablePhoneHome
    Enable phone home to Magnolia HQ (requires MAGNOLIA_API_KEY env var)

.PARAMETER PhoneHomeEndpoint
    Magnolia HQ endpoint URL (default: https://magnolia-hq.example.com/api/v1/reports)

.PARAMETER CloneCLIAIMonitor
    Clone CLIAIMONITOR repository if not present

.EXAMPLE
    .\bootstrap.ps1 -EnvironmentName "ACME Corp" -EnvironmentType customer

.EXAMPLE
    .\bootstrap.ps1 -EnvironmentName "Internal Dev" -EnvironmentType internal -EnablePhoneHome
#>

param(
    [Parameter(Mandatory=$true)]
    [string]$EnvironmentName,

    [Parameter(Mandatory=$true)]
    [ValidateSet("customer", "internal", "test")]
    [string]$EnvironmentType,

    [Parameter(Mandatory=$false)]
    [string]$EnvironmentID = "",

    [Parameter(Mandatory=$false)]
    [string]$CaptainID = "",

    [Parameter(Mandatory=$false)]
    [string]$DataDir = ".\bootstrap-data",

    [Parameter(Mandatory=$false)]
    [switch]$EnablePhoneHome,

    [Parameter(Mandatory=$false)]
    [string]$PhoneHomeEndpoint = "https://magnolia-hq.example.com/api/v1/reports",

    [Parameter(Mandatory=$false)]
    [switch]$CloneCLIAIMonitor
)

# Color output helpers
function Write-Success {
    param([string]$Message)
    Write-Host "[SUCCESS] $Message" -ForegroundColor Green
}

function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Cyan
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARNING] $Message" -ForegroundColor Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

Write-Info "CLIAIMONITOR Bootstrap Kit - Initializing Captain"
Write-Info "=================================================="

# Generate IDs if not provided
if ($EnvironmentID -eq "") {
    $EnvironmentID = "env-" + [guid]::NewGuid().ToString().Substring(0, 8)
    Write-Info "Generated Environment ID: $EnvironmentID"
}

if ($CaptainID -eq "") {
    $CaptainID = "captain-" + [guid]::NewGuid().ToString().Substring(0, 8)
    Write-Info "Generated Captain ID: $CaptainID"
}

# Create directory structure
Write-Info "Creating directory structure..."
$directories = @(
    $DataDir,
    "$DataDir\bootstrap",
    "$DataDir\docs\recon",
    "$DataDir\logs"
)

foreach ($dir in $directories) {
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        Write-Success "Created: $dir"
    } else {
        Write-Info "Already exists: $dir"
    }
}

# Create state.json
Write-Info "Creating bootstrap state file..."
$statePath = "$DataDir\bootstrap\state.json"

$state = @{
    version = "1.0"
    captain_id = $CaptainID
    environment = @{
        id = $EnvironmentID
        name = $EnvironmentName
        type = $EnvironmentType
        first_contact = (Get-Date).ToString("o")
    }
    mode = "lightweight"
    findings_summary = @{
        critical = 0
        high = 0
        medium = 0
        low = 0
    }
    active_agents = @()
    pending_decisions = @()
    phone_home = @{
        enabled = $EnablePhoneHome.IsPresent
        endpoint = $PhoneHomeEndpoint
        last_sync = $null
        api_key_env = "MAGNOLIA_API_KEY"
    }
    scale_up = @{
        triggered = $false
        reason = $null
        cliaimonitor_port = $null
    }
}

$state | ConvertTo-Json -Depth 10 | Set-Content -Path $statePath -Encoding UTF8
Write-Success "Created state file: $statePath"

# Create recon directory structure
Write-Info "Creating recon documentation structure..."
$reconDocs = @{
    "architecture.md" = @"
# Architecture Analysis - $EnvironmentName

## Overview
This document tracks architectural findings from reconnaissance.

## Components Discovered
- (Snake agents will populate this section)

## Patterns Identified
- (Snake agents will populate this section)

## Recommendations
- (Snake agents will populate this section)

---
*Last updated by Snake reconnaissance*
"@
    "vulnerabilities.md" = @"
# Security Vulnerabilities - $EnvironmentName

## Critical Findings
- (None yet)

## High Priority Findings
- (None yet)

## Medium Priority Findings
- (None yet)

## Low Priority Findings
- (None yet)

---
*Last updated by Snake reconnaissance*
"@
    "dependencies.md" = @"
# Dependency Analysis - $EnvironmentName

## Languages Detected
- (Pending reconnaissance)

## Frameworks Detected
- (Pending reconnaissance)

## Dependency Health
- (Pending reconnaissance)

---
*Last updated by Snake reconnaissance*
"@
    "infrastructure.md" = @"
# Infrastructure Assessment - $EnvironmentName

## Deployment Configuration
- (Pending reconnaissance)

## Services Discovered
- (Pending reconnaissance)

## Network Topology
- (Pending reconnaissance)

---
*Last updated by Snake reconnaissance*
"@
}

foreach ($doc in $reconDocs.Keys) {
    $docPath = "$DataDir\docs\recon\$doc"
    $reconDocs[$doc] | Set-Content -Path $docPath -Encoding UTF8
    Write-Success "Created: $docPath"
}

# Check for CLIAIMONITOR
Write-Info "Checking for CLIAIMONITOR..."
$cliaimonitorFound = $false
$cliaimonitorPath = $null

# Check current directory
if (Test-Path ".\cliaimonitor.exe") {
    $cliaimonitorPath = Resolve-Path ".\cliaimonitor.exe"
    $cliaimonitorFound = $true
} elseif (Test-Path "..\cliaimonitor.exe") {
    $cliaimonitorPath = Resolve-Path "..\cliaimonitor.exe"
    $cliaimonitorFound = $true
} else {
    # Check PATH
    $pathCLI = Get-Command cliaimonitor -ErrorAction SilentlyContinue
    if ($pathCLI) {
        $cliaimonitorPath = $pathCLI.Source
        $cliaimonitorFound = $true
    }
}

if ($cliaimonitorFound) {
    Write-Success "CLIAIMONITOR found at: $cliaimonitorPath"
} else {
    Write-Warning "CLIAIMONITOR not found"
    if ($CloneCLIAIMonitor) {
        Write-Info "Attempting to clone CLIAIMONITOR repository..."
        # This would require git to be installed
        Write-Warning "Clone functionality not yet implemented - please install CLIAIMONITOR manually"
    } else {
        Write-Info "You can install CLIAIMONITOR later if scale-up is needed"
    }
}

# Validate phone home configuration
if ($EnablePhoneHome) {
    Write-Info "Validating phone home configuration..."
    $apiKey = $env:MAGNOLIA_API_KEY
    if (-not $apiKey) {
        Write-Warning "MAGNOLIA_API_KEY environment variable not set"
        Write-Warning "Phone home is enabled but will fail until API key is configured"
    } else {
        Write-Success "MAGNOLIA_API_KEY is configured"
    }
}

# Create quick reference file
Write-Info "Creating quick reference..."
$quickRef = @"
# CLIAIMONITOR Bootstrap Quick Reference

## Environment
- Name: $EnvironmentName
- Type: $EnvironmentType
- ID: $EnvironmentID

## Captain
- ID: $CaptainID
- Mode: lightweight
- Phone Home: $($EnablePhoneHome.IsPresent)

## Files
- State: $statePath
- Recon Docs: $DataDir\docs\recon\
- Logs: $DataDir\logs\

## Next Steps

1. **Start reconnaissance**:
   If CLIAIMONITOR is installed, spawn a Snake agent:
   ``````
   cliaimonitor spawn snake --env "$EnvironmentID" --mission "initial_recon"
   ``````

2. **Check status**:
   ``````
   cliaimonitor bootstrap status
   ``````

3. **Phone home** (if enabled):
   ``````
   cliaimonitor bootstrap phone-home
   ``````

4. **Scale up** (when needed):
   ``````
   cliaimonitor bootstrap scale-up
   ``````

## Phone Home Endpoint
$PhoneHomeEndpoint

## Documentation
See bootstrap/README.md for full documentation
"@

$quickRef | Set-Content -Path "$DataDir\QUICK_START.md" -Encoding UTF8
Write-Success "Created: $DataDir\QUICK_START.md"

# Summary
Write-Info ""
Write-Info "=================================================="
Write-Success "Bootstrap Complete!"
Write-Info "=================================================="
Write-Info ""
Write-Info "Environment: $EnvironmentName ($EnvironmentType)"
Write-Info "Captain ID: $CaptainID"
Write-Info "Data Directory: $DataDir"
Write-Info "Mode: lightweight"
if ($EnablePhoneHome) {
    Write-Info "Phone Home: ENABLED"
} else {
    Write-Info "Phone Home: DISABLED"
}
Write-Info ""
Write-Info "Next: Review $DataDir\QUICK_START.md for next steps"
Write-Info ""
