# Arcnem Vision Dashboard

React dashboard for managing projects, devices, API keys, workflows, uploaded documents, and live run inspection.

## What this package does

- **Projects & Devices tab**: create projects, register devices, assign default workflows, and issue or rotate device API keys.
- **Workflow Library tab**: create/edit graph workflows with a visual canvas.
- **Docs tab**: browse uploads, run semantic search, upload directly from the dashboard, inspect related segmented outputs, and queue workflows against any document.
- **Runs tab**: inspect execution history and per-step state changes with realtime refresh.

The dashboard runs as a TanStack Start app and talks to the API server for data and mutations.

## Local development

The recommended way to run the dashboard is via `tilt up` from the repository root — it starts all services including the dashboard with hot reload. See the [root README](../../../README.md#quickstart) for details.

To run the dashboard standalone:

```bash
cd server && bun i
cd server/packages/dashboard
cp .env.example .env
bun run dev
```

Dev server runs on `http://localhost:3001`.

## Required environment variables

`server/packages/dashboard/.env.example`:

- `API_URL`: API base URL (default local value: `http://localhost:3000`)
- `DATABASE_URL`: local Postgres connection string
- `DASHBOARD_SESSION_TOKEN`: optional local debug session token
- `REDIS_URL`: Redis connection string for realtime dashboard updates

## Workflow editor notes

- Node types: `worker`, `supervisor`, `tool`
- Worker/supervisor nodes require a model
- Workers can have multiple tools
- Tool nodes require exactly one tool and support input/output mapping
- Graph validation enforces unique node keys, valid edges, and entry-to-`END` reachability

## Document operations notes

- Documents search is wired to `query` on `/api/dashboard/documents`
- If a matching description is found, the API returns nearest semantic matches using embedding distance
- If no semantic seed is found, the API falls back to lexical `ILIKE` matching
- Dashboard uploads use `/api/dashboard/documents/uploads/presign` and `/api/dashboard/documents/uploads/ack`
- Related segmentation outputs are fetched from `/api/dashboard/documents/:id/segmentations`
- Selecting a document lets operators queue any saved workflow against it with `/api/dashboard/documents/:id/run`

## Realtime notes

- The dashboard subscribes to `/api/realtime/dashboard` with Server-Sent Events
- Documents refresh on document creation, description updates, and segmentation creation
- Runs refresh on run creation, step changes, and run completion

## Build

```bash
bun run build
```
