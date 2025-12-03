# Bootstrap Kit

This directory contains the Bootstrap Kit for deploying CLIAIMONITOR Captain in infrastructure-poor environments.

## Overview

The Bootstrap Kit enables the Captain to operate with minimal infrastructure, carrying only essential state in a portable JSON file. This allows deployment to customer sites, new networks, or any environment where full CLIAIMONITOR infrastructure isn't immediately available.

## Files

- **state.json** - Template for portable state file
- **bootstrap.ps1** - Windows initialization script
- **bootstrap.sh** - Unix/Linux initialization script

## Usage

### Windows

```powershell
.\bootstrap.ps1 -EnvironmentName "customer-acme" -EnvironmentType "customer"
```

### Linux/Mac

```bash
./bootstrap.sh --env "customer-acme" --type "customer"
```

## State File Structure

The `state.json` file contains:

- **Environment metadata** - Where the Captain is deployed
- **Findings summary** - Aggregate security/architectural findings
- **Active agents** - Currently running Snake/Worker agents
- **Phone home config** - Connection to Magnolia HQ
- **Scale-up triggers** - When to deploy full CLIAIMONITOR

## Modes

### Lightweight Mode
- Just state.json + Captain
- No persistent infrastructure
- Suitable for: Initial reconnaissance, short engagements

### Local Mode
- CLIAIMONITOR running locally
- Full agent coordination
- Suitable for: Multi-agent engagements, extended work

### Connected Mode
- Phone home to Magnolia HQ
- Remote backup and reporting
- Suitable for: Customer deployments, distributed teams

### Full Mode
- Complete infrastructure
- Dashboard, metrics, full persistence
- Suitable for: Production operations, long-term management

## Phone Home

When enabled, the Captain periodically sends:
- Findings summaries (encrypted)
- Agent status updates
- Requests for guidance

And receives:
- Priority overrides
- Task assignments
- Configuration updates

API key must be set via environment variable specified in `phone_home.api_key_env`.

## Scale-Up Detection

The Captain automatically scales up infrastructure when:
- More than 3 agents needed simultaneously
- Multi-day engagement detected
- Customer requests dashboard
- Critical findings require immediate coordination

Scale-up creates local CLIAIMONITOR instance and migrates state seamlessly.
