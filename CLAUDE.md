# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**DataWatch** — Plataforma de monitoreo transaccional con motor de reglas dinámicas. Permite ingestar datos de múltiples fuentes (CSV, Excel, JSON, APIs), estructurarlos automáticamente, aplicar reglas configurables y generar dashboards personalizados.

## Architecture

```
monitoring/
├── frontend/          # Next.js 15 + React 19 + shadcn/ui + Tailwind CSS 4
├── backend/           # Go (Gin/Fiber) REST API + WebSocket
├── docker/            # Docker Compose (MongoDB, Redis, backend, frontend)
└── docs/              # Architecture decisions, API specs
```

### Frontend (`frontend/`)
- **Framework:** Next.js 15 (App Router) with TypeScript
- **UI:** shadcn/ui components + Tailwind CSS 4, minimalist design
- **State:** Zustand for global state, TanStack Query for server state
- **Charts:** Recharts for dashboards
- **Auth:** NextAuth.js with role-based access (admin, analyst, viewer)
- **Theming:** Light/dark mode with CSS variables

### Backend (`backend/`)
- **Language:** Go 1.22+
- **Router:** Fiber v2
- **Database:** MongoDB (flexible schema for varied data structures)
- **Cache:** Redis (session management, rule evaluation cache)
- **Auth:** JWT tokens with role claims
- **File processing:** Ingestion pipeline for CSV, XLSX, JSON
- **Rules engine:** Dynamic rule evaluation with Claude API for AI-assisted rule generation

### Data Flow
```
Data Sources (CSV/Excel/JSON/API) → Ingestion Pipeline → Schema Detection →
MongoDB Storage → Redis Queue → Worker Pool → Rules Engine → Alerts
```

### Async Rule Evaluation (Queue + Workers)
Rule evaluation is decoupled from data ingestion via a Redis queue:
- Upload endpoint enqueues an `EvalJob` and returns immediately
- Worker goroutines (`internal/services/worker.go`) poll the queue with `BRPOP`
- Failed evaluations retry up to 3 times, then move to dead queue
- Workers are started in `main.go` only when Redis is available
- Queue key: `datawatch:queue:rule_eval`, dead: `datawatch:queue:dead`

## Build & Development Commands

### Frontend
```bash
cd frontend
npm install              # Install dependencies
npm run dev              # Dev server (http://localhost:3000)
npm run build            # Production build
npm run lint             # ESLint
npm run test             # Vitest unit tests
npm run test -- --run    # Run tests once (CI mode)
```

### Backend
```bash
cd backend
go mod tidy              # Sync dependencies
go run cmd/server/main.go  # Run API server (http://localhost:8080)
go build -o bin/server cmd/server/main.go  # Build binary
go test ./...            # Run all tests
go test ./internal/rules/...  # Run specific package tests
go test -v -run TestRuleName ./internal/rules/  # Single test
golangci-lint run        # Linting
```

### Infrastructure
```bash
docker compose up -d     # Start MongoDB + Redis
docker compose down      # Stop services
docker compose logs -f   # Follow logs
```

## Key Architecture Patterns

### Schema Detection Engine
Data uploaded in any format passes through a schema detector that infers field types and creates a MongoDB collection schema dynamically. Each "monitor" has its own collection and schema definition stored in a `schemas` collection.

### Rules Engine
Rules are stored as JSON documents with conditions, thresholds, and actions. Structure:
```json
{
  "monitor_id": "...",
  "name": "High value alert",
  "conditions": [{"field": "amount", "operator": "gt", "value": 10000}],
  "actions": ["alert", "flag"],
  "severity": "high"
}
```
Rules support: comparison operators, regex, aggregate functions, time windows, and composite (AND/OR) logic.

### Role-Based Access
Three roles with hierarchical permissions:
- **admin**: Full access — manage users, monitors, rules, system config
- **analyst**: Create/edit monitors, rules, dashboards. Cannot manage users
- **viewer**: Read-only access to assigned dashboards

### Dashboard System
Dashboards are composed of configurable widgets tied to specific monitors and rules. Each widget queries aggregated data and renders via Recharts. Dashboards are shareable and role-restricted.

## Coding Conventions

### Frontend
- Use `"use client"` only when necessary (interactivity, hooks)
- Prefer Server Components for data fetching
- All API calls go through `/lib/api/` client module
- shadcn/ui components live in `components/ui/`, custom in `components/`
- Use `cn()` utility for conditional classNames

### Backend
- Follow standard Go project layout (`cmd/`, `internal/`, `pkg/`)
- Handlers in `internal/handlers/`, business logic in `internal/services/`
- MongoDB operations in `internal/repository/`
- All errors wrapped with context: `fmt.Errorf("operation: %w", err)`
- Configuration via environment variables loaded in `internal/config/`

## Environment Variables

### Frontend (`.env.local`)
```
NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
NEXTAUTH_SECRET=<secret>
NEXTAUTH_URL=http://localhost:3000
```

### Backend (`.env`)
```
MONGO_URI=mongodb://localhost:27017/datawatch
REDIS_URL=redis://localhost:6379
JWT_SECRET=<secret>
PORT=8080
```

## MongoDB Collections
- `users` — User accounts and roles
- `monitors` — Monitor definitions (name, source type, schedule)
- `schemas` — Detected schemas per monitor
- `data_<monitor_id>` — Dynamic collections per monitor for ingested data
- `rules` — Rule definitions linked to monitors
- `alerts` — Triggered alerts from rule evaluation
- `dashboards` — Dashboard configurations and widget layouts
