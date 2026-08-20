# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Thureos Compliance** — Plataforma de monitoreo transaccional con motor de reglas dinámicas. Permite ingestar datos de múltiples fuentes (CSV, Excel, JSON, APIs), estructurarlos automáticamente, aplicar reglas configurables y generar dashboards personalizados.

## Architecture

```
monitors-main/
├── frontend/          # Next.js 15 + React 19 + shadcn/ui + Tailwind CSS 4
├── backend/           # Go (Gin/Fiber) REST API + WebSocket
├── linea_grafica/     # Manual de marca y arte del logotipo (normativo)
└── docs/superpowers/  # Specs de diseño y planes de implementación
```

### Frontend (`frontend/`)
- **Framework:** Next.js 15 (App Router) with TypeScript
- **UI:** shadcn/ui components + Tailwind CSS 4, minimalist design
- **State:** Zustand for global state, TanStack Query for server state
- **Charts:** Recharts for dashboards
- **Auth:** JWT del backend, estado de sesión en Zustand (`stores/auth-store.ts`). Roles: admin, compliance, viewer
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
- Queue key: `thureos:queue:rule_eval`, dead: `thureos:queue:dead`

## Build & Development Commands

### Frontend
```bash
cd frontend
npm install              # Install dependencies
npm run dev              # Dev server (http://localhost:3000)
npm run build            # Production build
npm run lint             # ⚠ ESLint NO está configurado: abre un asistente interactivo
npm run test             # ⚠ Vitest instalado pero el proyecto no tiene tests
```

### Backend
```bash
cd backend
go mod tidy              # Sync dependencies
go run cmd/server/main.go  # Run API server (http://localhost:8080)
go build -o bin/server cmd/server/main.go  # Build binary
go test ./...            # ⚠ el backend no tiene tests todavía
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
- **compliance**: Create/edit monitors, rules, dashboards. Cannot manage users
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

> Estado del proyecto y hallazgos abiertos (sin tests, ESLint sin configurar, `gofmt`
> pendiente, sin rate limiting): `docs/superpowers/auditoria-2026-08-20.md`.

## Sistema de marca Thureos

La aplicación es **Thureos Compliance**. «Compliance» es un descriptor funcional,
nunca una marca independiente: no se usa solo.

- Tokens en `frontend/src/styles/tokens/`. **No se editan**: son copias de la fuente
  de verdad (`/Users/slacker/Downloads/tokens/`, con copia idéntica en
  `thureos_monitoreo/packages/design-tokens/`). Cualquier ajuste va en la capa
  puente `@theme inline` de `globals.css`.
- Tema y marca son atributos de `<html>`: `data-theme="dark|light"` y
  `data-brand="compliance"`. Cambiar de producto es cambiar un atributo.
- El puente envuelve los tripletes de shadcn: `--color-primary: hsl(var(--primary))`.
  Sin el `hsl()` Tailwind emite CSS inválido que se ignora **en silencio**.
- **Ningún componente escribe un color literal.** Importa de `@/lib/semantic-colors`
  o usa las utilidades de token (`bg-canvas`, `text-ink-muted`, `bg-risk-high-bg`…).
- Tres ejes de color que no se mezclan: riesgo (`--risk-*`), estado (`--success-*`,
  `--danger-*`…) y categoría (`--chart-1..8`). El acento azul identifica al producto
  y **nunca** comunica severidad.
- Tipografía: Inter 400/500/600/700 para lectura; JetBrains Mono 400/500/600 para
  identificadores, montos, hashes y timestamps.
- El arte del logotipo no se modifica, recolorea, redibuja ni revectoriza.
  Único punto que lo referencia: `components/brand/logo.tsx`.
- Manual normativo: `linea_grafica/Thureos-Manual-de-Marca_1.pdf`.

## Environment Variables

### Frontend (`.env.local`)
```
NEXT_PUBLIC_API_URL=http://localhost:8080/api/v1
```

### Backend (`.env`)
```
# Puertos publicados por docker-compose.yml, no los estándar
MONGO_URI=mongodb://localhost:27019/thureos_compliance
REDIS_URL=redis://localhost:6383
APP_ENV=development
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
