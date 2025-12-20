# MAH Platform Vision: Next-Gen Hosting Infrastructure

## The Big Picture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           MAH PLATFORM                                       │
│  "Give it resources, it provisions and hosts - fast and efficient"          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐ │
│   │   PHASE 1   │    │   PHASE 2   │    │   PHASE 3   │    │   PHASE 4   │ │
│   │  WMH Core   │───▶│  Go CMS     │───▶│  AI Layer   │───▶│  Marketplace│ │
│   │  (cPanel    │    │  (WP        │    │  (AI-       │    │  (Sell      │ │
│   │  killer)    │    │  killer)    │    │  powered)   │    │  anything)  │ │
│   └─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘ │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: WMH Core (Web/Magnolia Hosting - cPanel Replacement)

### Why cPanel Sucks
- Bloated (500MB+ RAM just for the panel)
- Expensive licensing ($15-45/mo per server)
- PHP-heavy, slow
- Over-engineered for simple tasks
- Legacy architecture

### WMH Core Philosophy
- **Single Go binary** - deploy anywhere
- **Sub-50MB memory footprint** for the control panel
- **API-first** - everything is an API call
- **Container-native** - every site in its own container
- **Agent-based** - MAH agents provision and manage

### Core Components

```
WMH Core Architecture
=====================

┌──────────────────────────────────────────────────────────────────┐
│                        WMH Control Plane                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │ API      │  │ Web UI   │  │ CLI      │  │ Agent Interface  │  │
│  │ Server   │  │ (React)  │  │ wmh      │  │ (MCP/NATS)       │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬─────────┘  │
│       └─────────────┴─────────────┴─────────────────┘            │
│                              │                                    │
│  ┌───────────────────────────┴───────────────────────────────┐   │
│  │                    Core Services                           │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │   │
│  │  │ Auth    │ │ Billing │ │ DNS     │ │ SSL/TLS │          │   │
│  │  │ (JWT)   │ │ Stripe  │ │ (BIND/  │ │ (ACME)  │          │   │
│  │  │         │ │ Paddle  │ │ PowerDNS│ │         │          │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘          │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐          │   │
│  │  │ Storage │ │ Backup  │ │ Monitor │ │ Logging │          │   │
│  │  │ (S3/    │ │ (Restic)│ │ (Prom)  │ │ (Loki)  │          │   │
│  │  │ Minio)  │ │         │ │         │ │         │          │   │
│  │  └─────────┘ └─────────┘ └─────────┘ └─────────┘          │   │
│  └───────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌──────────────────────────────────────────────────────────────────┐
│                     Infrastructure Layer                          │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                  Provisioner Agent                        │    │
│  │  "Give me resources, I'll create hosting"                │    │
│  │                                                           │    │
│  │  Inputs:           Outputs:                               │    │
│  │  - Proxmox API     - VMs with WMH node                   │    │
│  │  - DO API          - Containers with apps                │    │
│  │  - Vultr API       - Databases                           │    │
│  │  - AWS/GCP         - Configured services                 │    │
│  │  - Bare metal SSH  - DNS records                         │    │
│  │  - CloudLinux      - SSL certs                           │    │
│  └──────────────────────────────────────────────────────────┘    │
│                              │                                    │
│  ┌──────────────────────────────────────────────────────────┐    │
│  │                    Node Agent (per server)                │    │
│  │  - Container runtime (Podman/Docker)                     │    │
│  │  - Reverse proxy (Caddy - auto SSL)                      │    │
│  │  - Resource limits (cgroups)                             │    │
│  │  - Health monitoring                                      │    │
│  │  - Log shipping                                           │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

### WMH Feature Matrix vs cPanel

| Feature | cPanel | WMH Core | Advantage |
|---------|--------|----------|-----------|
| Memory footprint | 500MB+ | <50MB | 10x lighter |
| Licensing | $15-45/mo | Free/Open | Cost savings |
| Site isolation | WHM accounts | Containers | Better security |
| SSL setup | Manual/AutoSSL | Auto (Caddy) | Zero config |
| Deployment | FTP/cPanel | Git push/CLI | Modern workflow |
| API | Partial | 100% | Automatable |
| Multi-server | Complex | Native | Built-in clustering |
| Container support | None | Native | Modern apps |

### Tech Stack for WMH Core

```go
// Core binary structure
cmd/
  wmh/           // CLI tool
  wmh-server/    // API server
  wmh-node/      // Per-server agent
  wmh-proxy/     // Edge proxy (or use Caddy)

internal/
  api/           // REST/gRPC API
  auth/          // JWT, API keys, OAuth
  billing/       // Stripe integration (already started!)
  dns/           // PowerDNS/BIND management
  ssl/           // ACME/Let's Encrypt
  container/     // Podman/Docker abstraction
  storage/       // S3/Minio/local
  backup/        // Restic wrapper
  provision/     // Multi-cloud provisioner
  monitor/       // Prometheus metrics

pkg/
  wmhctl/        // Client library
```

### Key Differentiators

1. **Container-First**: Every site is a container
   - Isolation by default
   - Easy resource limits
   - Portable (move between nodes)
   - Reproducible environments

2. **Git-Native Deployments**
   ```bash
   git push wmh main  # Deploy like Heroku
   ```

3. **Instant Provisioning**
   ```bash
   wmh site create mysite.com --template=wordpress
   # Site live in <30 seconds
   ```

4. **Smart Resource Allocation**
   - Auto-scale containers
   - Move sites between nodes based on load
   - Hibernate idle sites (save resources)

---

## Phase 2: GoCMS (WordPress Replacement)

### Why WordPress Sucks
- PHP = slow, memory hungry
- Database-heavy (every page = 50+ queries)
- Plugin hell (security nightmares)
- 40MB for "Hello World"
- Needs caching layers to be fast

### GoCMS Philosophy
- **Single binary** - no runtime dependencies
- **Embedded database** - SQLite or optional Postgres
- **File-based content** - Markdown + frontmatter (like Hugo but dynamic)
- **Built-in caching** - in-memory, Redis optional
- **Plugin-safe** - WASM plugins (sandboxed)

### Architecture

```
GoCMS Architecture
==================

┌─────────────────────────────────────────────────────────────┐
│                      GoCMS Binary                            │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                   HTTP Server                        │    │
│  │   - Static file serving (embedded)                  │    │
│  │   - Template rendering (html/template)              │    │
│  │   - API endpoints (REST + GraphQL)                  │    │
│  │   - WebSocket (live preview)                        │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                  │
│  ┌────────────┬───────────┴───────────┬────────────────┐    │
│  │            │                       │                │    │
│  ▼            ▼                       ▼                ▼    │
│  ┌────────┐ ┌────────┐ ┌───────────┐ ┌──────────────┐      │
│  │Content │ │ Theme  │ │  Plugin   │ │    Admin     │      │
│  │Manager │ │ Engine │ │  Runtime  │ │   Dashboard  │      │
│  │        │ │        │ │  (WASM)   │ │   (React)    │      │
│  │-Markdown│ │-Templ  │ │           │ │              │      │
│  │-Assets │ │-TailCSS│ │-Sandboxed │ │-Visual Edit  │      │
│  │-Media  │ │-Blocks │ │-API hooks │ │-SEO tools    │      │
│  └────────┘ └────────┘ └───────────┘ └──────────────┘      │
│       │          │            │              │               │
│       └──────────┴────────────┴──────────────┘               │
│                           │                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │                   Data Layer                         │    │
│  │   SQLite (default) ──or── PostgreSQL (scale)        │    │
│  │   + In-memory cache (ristretto)                     │    │
│  │   + Optional Redis                                   │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘

Memory: ~20MB idle, ~50MB under load
Startup: <100ms
Requests: 10,000+ req/sec (static), 2,000+ req/sec (dynamic)
```

### Content Model

```yaml
# content/posts/hello-world.md
---
title: "Hello World"
date: 2025-01-15
author: admin
tags: [welcome, first-post]
template: post
seo:
  description: "My first GoCMS post"
  image: /media/hello.jpg
---

# Hello World

This is **Markdown** content with full support for:

- Code blocks with syntax highlighting
- Embedded media
- Custom shortcodes
- Dynamic data via {{.Site.Config.Name}}
```

### Theme System

```
themes/
  starter/
    layouts/
      base.html       # Base template
      home.html       # Homepage
      post.html       # Single post
      list.html       # List/archive
      partials/
        header.html
        footer.html
        nav.html
    assets/
      css/
        main.css      # TailwindCSS
      js/
        main.js       # Alpine.js
    theme.yaml        # Theme config
```

### Plugin System (WASM)

```go
// Plugins run in WASM sandbox - can't crash the host
type Plugin interface {
    Name() string
    Init(api PluginAPI) error

    // Hooks
    OnContentSave(content *Content) error
    OnPageRender(page *Page) error
    OnAPIRequest(r *Request) (*Response, error)
}

// Example: SEO plugin
func (p *SEOPlugin) OnPageRender(page *Page) error {
    page.Meta["og:title"] = page.Title
    page.Meta["og:description"] = page.SEO.Description
    return nil
}
```

### WP Migration Path

```bash
# One-command WordPress import
gocms import wordpress --url=https://mysite.com \
  --user=admin --password=xxx \
  --include=posts,pages,media,users
```

---

## Phase 3: AI Layer

### AI Integration Points

```
┌─────────────────────────────────────────────────────────────────┐
│                     MAH AI Layer                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    AI Gateway                            │    │
│  │  Route requests to appropriate AI providers              │    │
│  │                                                          │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐   │    │
│  │  │ Anthropic│ │ OpenAI   │ │ Local    │ │ Custom   │   │    │
│  │  │ Claude   │ │ GPT-4    │ │ Ollama   │ │ Fine-    │   │    │
│  │  │          │ │          │ │ Llama    │ │ tuned    │   │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘   │    │
│  └─────────────────────────────────────────────────────────┘    │
│                              │                                   │
│  ┌───────────────────────────┴───────────────────────────────┐  │
│  │                    AI Services                             │  │
│  │                                                            │  │
│  │  ┌─────────────────┐  ┌─────────────────┐                 │  │
│  │  │ Site Builder AI │  │ Content Writer  │                 │  │
│  │  │ - Generate sites│  │ - Blog posts    │                 │  │
│  │  │ - From prompt   │  │ - SEO content   │                 │  │
│  │  │ - Theme suggest │  │ - Translations  │                 │  │
│  │  └─────────────────┘  └─────────────────┘                 │  │
│  │                                                            │  │
│  │  ┌─────────────────┐  ┌─────────────────┐                 │  │
│  │  │ Code Assistant  │  │ DevOps AI       │                 │  │
│  │  │ - Debug help    │  │ - Auto-scaling  │                 │  │
│  │  │ - Code review   │  │ - Security scan │                 │  │
│  │  │ - Optimization  │  │ - Performance   │                 │  │
│  │  └─────────────────┘  └─────────────────┘                 │  │
│  │                                                            │  │
│  │  ┌─────────────────┐  ┌─────────────────┐                 │  │
│  │  │ Support Bot     │  │ Analytics AI    │                 │  │
│  │  │ - Help desk     │  │ - Insights      │                 │  │
│  │  │ - Docs search   │  │ - Predictions   │                 │  │
│  │  │ - Troubleshoot  │  │ - Anomalies     │                 │  │
│  │  └─────────────────┘  └─────────────────┘                 │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### AI-Powered Site Creation

```bash
# User prompt
wmh ai create-site "A portfolio site for a photographer
named Jane. Modern, minimalist, dark theme.
Should have gallery, about, and contact pages."

# MAH AI:
# 1. Generates site structure
# 2. Creates initial content
# 3. Selects/customizes theme
# 4. Configures SEO
# 5. Site live in 60 seconds
```

### Customer AI Options (Resellable)

```yaml
# ai-plans.yaml
plans:
  - name: ai-basic
    price: $5/mo
    features:
      - 100 AI requests/mo
      - Content generation
      - Basic support bot
    models:
      - claude-3-haiku
      - gpt-3.5-turbo

  - name: ai-pro
    price: $20/mo
    features:
      - 1000 AI requests/mo
      - Site builder AI
      - Code assistant
      - Advanced analytics
    models:
      - claude-3-5-sonnet
      - gpt-4-turbo

  - name: ai-enterprise
    price: $100/mo
    features:
      - Unlimited AI requests
      - All AI features
      - Custom fine-tuning
      - Priority support
    models:
      - claude-opus-4-5
      - gpt-4o
      - Custom models
```

---

## Phase 4: Marketplace & Full Platform

### Revenue Streams

```
┌─────────────────────────────────────────────────────────────────┐
│                    MAH Revenue Model                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. HOSTING (Core Revenue)                                      │
│     ├── Shared hosting: $5-20/mo                                │
│     ├── VPS/Container: $10-100/mo                               │
│     ├── Dedicated: $100-500/mo                                  │
│     └── Enterprise: Custom pricing                              │
│                                                                  │
│  2. AI ADD-ONS (High Margin)                                    │
│     ├── AI Basic: $5/mo (cost: ~$1)                            │
│     ├── AI Pro: $20/mo (cost: ~$5)                             │
│     └── AI Enterprise: $100/mo (cost: ~$30)                    │
│                                                                  │
│  3. MARKETPLACE (Commission)                                    │
│     ├── Themes: 30% commission                                  │
│     ├── Plugins: 30% commission                                 │
│     ├── Templates: 30% commission                               │
│     └── Services: 20% commission                                │
│                                                                  │
│  4. WHITELABEL (B2B)                                           │
│     ├── Reseller hosting                                        │
│     ├── White-label WMH                                         │
│     └── API access plans                                        │
│                                                                  │
│  5. MANAGED SERVICES                                            │
│     ├── Migration service: $50-500 one-time                    │
│     ├── Managed WordPress: $30/mo premium                      │
│     └── Security monitoring: $10/mo                            │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Marketplace Components

```
marketplace/
  themes/
    - starter (free)
    - business-pro ($49)
    - ecommerce ($79)
    - portfolio ($29)

  plugins/
    - seo-pro ($29/yr)
    - ecommerce ($99/yr)
    - forms ($19/yr)
    - analytics ($39/yr)
    - backup-pro ($49/yr)

  templates/
    - landing-pages (10 pack, $19)
    - email-templates (20 pack, $29)
    - blog-layouts (15 pack, $15)

  services/
    - wp-migration ($99)
    - custom-theme ($500+)
    - seo-audit ($199)
    - security-audit ($299)
```

---

## Implementation Roadmap

### Phase 1: WMH Core (Foundation)

```
Wave 1.1 - Core Infrastructure
├── [ ] WMH API server (Go)
├── [ ] Authentication (JWT + API keys)
├── [ ] User/Organization management
├── [ ] SQLite + Postgres support
└── [ ] Basic CLI (wmh)

Wave 1.2 - Container Runtime
├── [ ] Podman/Docker abstraction layer
├── [ ] Container lifecycle management
├── [ ] Resource limits (CPU/RAM/IO)
├── [ ] Container networking
└── [ ] Volume management

Wave 1.3 - Web Serving
├── [ ] Caddy integration (auto-SSL)
├── [ ] Domain management
├── [ ] DNS API (PowerDNS/Cloudflare)
├── [ ] SSL certificate automation
└── [ ] Reverse proxy config generation

Wave 1.4 - Multi-Provider Provisioning
├── [ ] Proxmox API integration
├── [ ] DigitalOcean provisioner
├── [ ] Vultr provisioner
├── [ ] Generic SSH provisioner
├── [ ] CloudLinux integration
└── [ ] Node auto-discovery

Wave 1.5 - Operations
├── [ ] Backup system (Restic)
├── [ ] Monitoring (Prometheus metrics)
├── [ ] Logging (structured JSON)
├── [ ] Alerting rules
└── [ ] Web dashboard (React)
```

### Phase 2: GoCMS

```
Wave 2.1 - Core CMS
├── [ ] Content model (Markdown + frontmatter)
├── [ ] Template engine
├── [ ] Asset pipeline
├── [ ] SQLite storage
└── [ ] Admin API

Wave 2.2 - Admin Dashboard
├── [ ] Visual editor (blocks)
├── [ ] Media manager
├── [ ] User management
├── [ ] SEO tools
└── [ ] Settings UI

Wave 2.3 - Theme System
├── [ ] Theme structure spec
├── [ ] Starter theme
├── [ ] TailwindCSS integration
├── [ ] Live preview
└── [ ] Theme marketplace prep

Wave 2.4 - Plugin System
├── [ ] WASM runtime
├── [ ] Plugin API spec
├── [ ] Core hooks
├── [ ] Example plugins
└── [ ] Security sandbox

Wave 2.5 - Migration Tools
├── [ ] WordPress importer
├── [ ] Ghost importer
├── [ ] Markdown folder import
├── [ ] Media migration
└── [ ] URL redirect mapping
```

### Phase 3: AI Layer

```
Wave 3.1 - AI Gateway
├── [ ] Multi-provider abstraction
├── [ ] Rate limiting & quotas
├── [ ] Usage tracking
├── [ ] Cost calculation
└── [ ] Model routing

Wave 3.2 - AI Services
├── [ ] Content generation API
├── [ ] Site builder AI
├── [ ] Code assistant integration
├── [ ] Support bot
└── [ ] Analytics AI

Wave 3.3 - Customer AI
├── [ ] AI plan management
├── [ ] Customer AI dashboard
├── [ ] Usage billing
├── [ ] Custom model support
└── [ ] Local model option (Ollama)
```

### Phase 4: Marketplace

```
Wave 4.1 - Marketplace Infrastructure
├── [ ] Vendor registration
├── [ ] Product listings
├── [ ] Payment processing
├── [ ] Commission system
└── [ ] Review system

Wave 4.2 - Whitelabel
├── [ ] Reseller accounts
├── [ ] Custom branding
├── [ ] API access tiers
├── [ ] Billing integration
└── [ ] Support escalation
```

---

## Technical Decisions

### Why Go for Everything?

1. **Single binary deployment** - No runtime dependencies
2. **Low memory** - 10-50MB vs 500MB+ for PHP stacks
3. **Fast startup** - <100ms vs seconds
4. **Concurrency** - Goroutines handle thousands of connections
5. **Cross-platform** - Build for any OS/arch
6. **Strong typing** - Fewer runtime errors
7. **Great tooling** - Testing, profiling, built-in

### Container Strategy

```
Site Container Architecture
===========================

┌─────────────────────────────────────────────────────┐
│                   Host Server                        │
│  ┌───────────────────────────────────────────────┐  │
│  │              Caddy (Edge Proxy)                │  │
│  │  - Auto SSL for all domains                   │  │
│  │  - Request routing                            │  │
│  │  - Rate limiting                              │  │
│  │  - Compression                                │  │
│  └─────────────────────┬─────────────────────────┘  │
│                        │                             │
│  ┌─────────┬───────────┼───────────┬─────────────┐  │
│  │         │           │           │             │  │
│  ▼         ▼           ▼           ▼             ▼  │
│ ┌────┐   ┌────┐      ┌────┐     ┌────┐       ┌────┐│
│ │Site│   │Site│      │Site│     │Site│       │Site││
│ │ A  │   │ B  │      │ C  │     │ D  │       │ E  ││
│ │    │   │    │      │    │     │    │       │    ││
│ │PHP │   │Node│      │Go  │     │Py  │       │Go  ││
│ │WP  │   │Next│      │CMS │     │DJ  │       │CMS ││
│ └────┘   └────┘      └────┘     └────┘       └────┘│
│   │         │          │          │            │   │
│   └─────────┴──────────┴──────────┴────────────┘   │
│                        │                            │
│  ┌─────────────────────┴─────────────────────────┐ │
│  │              Shared Services                   │ │
│  │  - PostgreSQL (multi-tenant)                  │ │
│  │  - Redis (sessions, cache)                    │ │
│  │  - Object storage mount                       │ │
│  └───────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

### Database Strategy

```
Tier 1: SQLite (Default)
- Single-site deployments
- Low-traffic sites
- GoCMS default
- Zero config

Tier 2: PostgreSQL (Scale)
- High-traffic sites
- Multi-tenant
- Complex queries
- WMH control plane

Tier 3: Distributed (Enterprise)
- CockroachDB
- TiDB
- Global deployments
```

---

## What We Already Have (CLIAIMONITOR → WMH)

### Reusable Components

| Component | Status | Reuse Plan |
|-----------|--------|------------|
| Agent orchestration | ✅ Working | Captain → WMH Orchestrator |
| MCP protocol | ✅ Working | Agent communication |
| NATS messaging | ✅ Working | Inter-service messaging |
| Billing/Stripe | ✅ Started | Customer billing |
| Task system | ✅ Working | Job queue for provisioning |
| Dashboard | ✅ Working | Evolve to WMH dashboard |
| Memory DB | ✅ Working | Context/state persistence |
| Spawner | ✅ Working | Container spawner base |

### Migration Path

```
CLIAIMONITOR                    WMH
============                    ===
Agent orchestration      →      WMH Orchestrator
Task queue               →      Provisioning queue
Spawner                  →      Container lifecycle
Dashboard                →      WMH Control Panel
Memory DB                →      State management
Billing integration      →      Customer billing
NATS                     →      Inter-node messaging
MCP tools                →      Provisioner tools
```

---

## Competitive Analysis

### vs cPanel/Plesk
- **WMH wins**: Cost, speed, modern architecture
- **They win**: Market presence, feature completeness
- **Strategy**: Target developers and agencies first

### vs Cloudways/RunCloud
- **WMH wins**: Self-hostable, no vendor lock-in
- **They win**: Managed experience
- **Strategy**: Offer both self-hosted and managed

### vs Coolify/CapRover
- **WMH wins**: Integrated CMS, AI features, polished UX
- **They win**: Existing community
- **Strategy**: Better developer experience

### vs WordPress
- **GoCMS wins**: Performance, security, simplicity
- **WP wins**: Ecosystem, market share
- **Strategy**: Easy migration, familiar concepts

---

## Success Metrics

### Phase 1 Success
- [ ] 100 sites hosted on WMH
- [ ] <30 second site provisioning
- [ ] <50MB memory per control plane
- [ ] 99.9% uptime

### Phase 2 Success
- [ ] GoCMS serving 1000 req/sec
- [ ] 50 sites migrated from WordPress
- [ ] 10 themes available
- [ ] 5 plugins in marketplace

### Phase 3 Success
- [ ] 100 customers using AI features
- [ ] $5k MRR from AI add-ons
- [ ] 3 AI providers integrated

### Phase 4 Success
- [ ] $50k MRR total
- [ ] 10 resellers
- [ ] 50 marketplace products
- [ ] 1000 active sites

---

## Open Questions

1. **Naming**: WMH? MagnoliaHost? Something else?
2. **Open source strategy**: Core open, premium closed?
3. **First target market**: Developers? Agencies? SMBs?
4. **GoCMS branding**: Separate brand or WMH integrated?
5. **AI pricing model**: Per-request? Subscription? Both?
6. **Geographic focus**: US first? Global?

---

## Next Steps (Immediate)

1. [ ] Finalize architecture decisions
2. [ ] Create WMH repo structure
3. [ ] Port/adapt CLIAIMONITOR components
4. [ ] Build container lifecycle manager
5. [ ] Implement first provisioner (DigitalOcean)
6. [ ] Basic web dashboard
7. [ ] Deploy first test site

---

*Document version: 0.1*
*Last updated: 2025-12-19*
*Author: Captain (Orchestrator)*
