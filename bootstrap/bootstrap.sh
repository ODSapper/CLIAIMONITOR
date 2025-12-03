#!/bin/bash

# CLIAIMONITOR Bootstrap Kit - Unix/Linux/Mac
# Initializes Captain with minimal state for infrastructure-poor deployments

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${CYAN}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Usage information
usage() {
    cat << EOF
CLIAIMONITOR Bootstrap Kit

Usage: $0 --env <name> --type <type> [options]

Required:
  --env <name>          Environment name (e.g., "ACME Corp")
  --type <type>         Environment type (customer|internal|test)

Optional:
  --env-id <id>         Environment ID (auto-generated if not provided)
  --captain-id <id>     Captain ID (auto-generated if not provided)
  --data-dir <path>     Data directory (default: ./bootstrap-data)
  --phone-home          Enable phone home to Magnolia HQ
  --phone-home-url <url> HQ endpoint URL
  --clone               Clone CLIAIMONITOR if not present
  --help                Show this help message

Examples:
  $0 --env "ACME Corp" --type customer
  $0 --env "Internal Dev" --type internal --phone-home

EOF
    exit 1
}

# Parse arguments
ENV_NAME=""
ENV_TYPE=""
ENV_ID=""
CAPTAIN_ID=""
DATA_DIR="./bootstrap-data"
PHONE_HOME=false
PHONE_HOME_URL="https://magnolia-hq.example.com/api/v1/reports"
CLONE_CLI=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --env)
            ENV_NAME="$2"
            shift 2
            ;;
        --type)
            ENV_TYPE="$2"
            shift 2
            ;;
        --env-id)
            ENV_ID="$2"
            shift 2
            ;;
        --captain-id)
            CAPTAIN_ID="$2"
            shift 2
            ;;
        --data-dir)
            DATA_DIR="$2"
            shift 2
            ;;
        --phone-home)
            PHONE_HOME=true
            shift
            ;;
        --phone-home-url)
            PHONE_HOME_URL="$2"
            shift 2
            ;;
        --clone)
            CLONE_CLI=true
            shift
            ;;
        --help)
            usage
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate required arguments
if [ -z "$ENV_NAME" ] || [ -z "$ENV_TYPE" ]; then
    log_error "Missing required arguments"
    usage
fi

# Validate environment type
if [[ ! "$ENV_TYPE" =~ ^(customer|internal|test)$ ]]; then
    log_error "Invalid environment type: $ENV_TYPE (must be customer, internal, or test)"
    exit 1
fi

log_info "CLIAIMONITOR Bootstrap Kit - Initializing Captain"
log_info "=================================================="

# Generate IDs if not provided
if [ -z "$ENV_ID" ]; then
    # Generate short UUID (first 8 chars)
    if command -v uuidgen &> /dev/null; then
        ENV_ID="env-$(uuidgen | cut -d'-' -f1)"
    else
        # Fallback to random string
        ENV_ID="env-$(head /dev/urandom | tr -dc a-z0-9 | head -c 8)"
    fi
    log_info "Generated Environment ID: $ENV_ID"
fi

if [ -z "$CAPTAIN_ID" ]; then
    if command -v uuidgen &> /dev/null; then
        CAPTAIN_ID="captain-$(uuidgen | cut -d'-' -f1)"
    else
        CAPTAIN_ID="captain-$(head /dev/urandom | tr -dc a-z0-9 | head -c 8)"
    fi
    log_info "Generated Captain ID: $CAPTAIN_ID"
fi

# Create directory structure
log_info "Creating directory structure..."
mkdir -p "$DATA_DIR/bootstrap"
mkdir -p "$DATA_DIR/docs/recon"
mkdir -p "$DATA_DIR/logs"
log_success "Created directory structure"

# Get current timestamp in ISO 8601 format
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Create state.json
log_info "Creating bootstrap state file..."
STATE_FILE="$DATA_DIR/bootstrap/state.json"

cat > "$STATE_FILE" << EOF
{
  "version": "1.0",
  "captain_id": "$CAPTAIN_ID",
  "environment": {
    "id": "$ENV_ID",
    "name": "$ENV_NAME",
    "type": "$ENV_TYPE",
    "first_contact": "$TIMESTAMP"
  },
  "mode": "lightweight",
  "findings_summary": {
    "critical": 0,
    "high": 0,
    "medium": 0,
    "low": 0
  },
  "active_agents": [],
  "pending_decisions": [],
  "phone_home": {
    "enabled": $PHONE_HOME,
    "endpoint": "$PHONE_HOME_URL",
    "last_sync": null,
    "api_key_env": "MAGNOLIA_API_KEY"
  },
  "scale_up": {
    "triggered": false,
    "reason": null,
    "cliaimonitor_port": null
  }
}
EOF

log_success "Created state file: $STATE_FILE"

# Create recon documentation structure
log_info "Creating recon documentation structure..."

cat > "$DATA_DIR/docs/recon/architecture.md" << 'EOF'
# Architecture Analysis

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
EOF

cat > "$DATA_DIR/docs/recon/vulnerabilities.md" << 'EOF'
# Security Vulnerabilities

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
EOF

cat > "$DATA_DIR/docs/recon/dependencies.md" << 'EOF'
# Dependency Analysis

## Languages Detected
- (Pending reconnaissance)

## Frameworks Detected
- (Pending reconnaissance)

## Dependency Health
- (Pending reconnaissance)

---
*Last updated by Snake reconnaissance*
EOF

cat > "$DATA_DIR/docs/recon/infrastructure.md" << 'EOF'
# Infrastructure Assessment

## Deployment Configuration
- (Pending reconnaissance)

## Services Discovered
- (Pending reconnaissance)

## Network Topology
- (Pending reconnaissance)

---
*Last updated by Snake reconnaissance*
EOF

log_success "Created recon documentation"

# Check for CLIAIMONITOR
log_info "Checking for CLIAIMONITOR..."
CLIAIMONITOR_FOUND=false
CLIAIMONITOR_PATH=""

if [ -f "./cliaimonitor" ]; then
    CLIAIMONITOR_PATH="$(pwd)/cliaimonitor"
    CLIAIMONITOR_FOUND=true
elif [ -f "../cliaimonitor" ]; then
    CLIAIMONITOR_PATH="$(cd .. && pwd)/cliaimonitor"
    CLIAIMONITOR_FOUND=true
elif command -v cliaimonitor &> /dev/null; then
    CLIAIMONITOR_PATH=$(which cliaimonitor)
    CLIAIMONITOR_FOUND=true
fi

if [ "$CLIAIMONITOR_FOUND" = true ]; then
    log_success "CLIAIMONITOR found at: $CLIAIMONITOR_PATH"
else
    log_warning "CLIAIMONITOR not found"
    if [ "$CLONE_CLI" = true ]; then
        log_info "Attempting to clone CLIAIMONITOR repository..."
        log_warning "Clone functionality not yet implemented - please install CLIAIMONITOR manually"
    else
        log_info "You can install CLIAIMONITOR later if scale-up is needed"
    fi
fi

# Validate phone home configuration
if [ "$PHONE_HOME" = true ]; then
    log_info "Validating phone home configuration..."
    if [ -z "$MAGNOLIA_API_KEY" ]; then
        log_warning "MAGNOLIA_API_KEY environment variable not set"
        log_warning "Phone home is enabled but will fail until API key is configured"
    else
        log_success "MAGNOLIA_API_KEY is configured"
    fi
fi

# Create quick reference file
log_info "Creating quick reference..."
cat > "$DATA_DIR/QUICK_START.md" << EOF
# CLIAIMONITOR Bootstrap Quick Reference

## Environment
- Name: $ENV_NAME
- Type: $ENV_TYPE
- ID: $ENV_ID

## Captain
- ID: $CAPTAIN_ID
- Mode: lightweight
- Phone Home: $PHONE_HOME

## Files
- State: $STATE_FILE
- Recon Docs: $DATA_DIR/docs/recon/
- Logs: $DATA_DIR/logs/

## Next Steps

1. **Start reconnaissance**:
   If CLIAIMONITOR is installed, spawn a Snake agent:
   \`\`\`bash
   cliaimonitor spawn snake --env "$ENV_ID" --mission "initial_recon"
   \`\`\`

2. **Check status**:
   \`\`\`bash
   cliaimonitor bootstrap status
   \`\`\`

3. **Phone home** (if enabled):
   \`\`\`bash
   cliaimonitor bootstrap phone-home
   \`\`\`

4. **Scale up** (when needed):
   \`\`\`bash
   cliaimonitor bootstrap scale-up
   \`\`\`

## Phone Home Endpoint
$PHONE_HOME_URL

## Documentation
See bootstrap/README.md for full documentation
EOF

log_success "Created: $DATA_DIR/QUICK_START.md"

# Make scripts executable if they exist
if [ -f "$DATA_DIR/bootstrap/bootstrap.sh" ]; then
    chmod +x "$DATA_DIR/bootstrap/bootstrap.sh"
fi

# Summary
echo ""
log_info "=================================================="
log_success "Bootstrap Complete!"
log_info "=================================================="
echo ""
log_info "Environment: $ENV_NAME ($ENV_TYPE)"
log_info "Captain ID: $CAPTAIN_ID"
log_info "Data Directory: $DATA_DIR"
log_info "Mode: lightweight"
if [ "$PHONE_HOME" = true ]; then
    log_info "Phone Home: ENABLED"
else
    log_info "Phone Home: DISABLED"
fi
echo ""
log_info "Next: Review $DATA_DIR/QUICK_START.md for next steps"
echo ""
