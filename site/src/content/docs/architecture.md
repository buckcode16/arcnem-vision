---
title: Architecture
description: How the services, data pipeline, and agent graph system fit together.
---

## System Diagram

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Flutter    │────▶│   Hono API   │────▶│     Inngest     │
│   Client     │     │   (Bun)      │     │   Event Queue   │
│              │     │              │     │                 │
│ GenUI + Gemma│     │ Presigned S3 │     └────────┬────────┘
└─────────────┘     │ better-auth  │              │
                    └──────────────┘              ▼
┌─────────────┐                        ┌──────────────────┐
│    React     │                        │   Go Agents      │
│  Dashboard   │                        │                  │
│              │     ┌──────────┐       │ LangGraph loads  │
│  Workflow    │────▶│ Postgres │◀──────│ graph from DB,   │
│  Builder     │     │ pgvector │       │ executes nodes   │
└─────────────┘     └──────────┘       └────────┬─────────┘
                         ▲                      │
                         │               ┌──────▼─────────┐
                    ┌────┴───┐           │   MCP Server    │
                    │   S3   │           │                 │
                    │Storage │           │ OCR, CLIP,      │
                    └────────┘           │ Descriptions,   │
                                         │ Search, Segment │
                                         └─────────────────┘
```

## The Pipeline

Client captures image → API issues presigned S3 URL → Client uploads directly → API acknowledges and fires Inngest event → Go agent service loads the document's agent graph from Postgres → LangGraph builds and executes the workflow → Worker nodes call LLMs, tool/condition/supervisor nodes route the work → MCP generates OCR, descriptions, embeddings, and segmentations → Everything lands in Postgres with HNSW cosine indexes plus persisted OCR results → Searchable by meaning.

## Agent Graph System

![Workflow Library showing both pipeline and supervisor workflows](/dashboard-workflows.png)

Agent graphs are data, not code. Templates define reusable workflows with nodes, edges, and tools. Instances bind templates to organizations. Four node types:

- **Worker** — ReAct agent with access to MCP tools
- **Tool** — Single MCP tool invocation with input/output mapping
- **Supervisor** — Multi-agent orchestration across workers
- **Condition** — Deterministic branching on state using `contains` / `equals` checks and explicit true/false targets

Every execution is traced step-by-step in `agent_graph_runs` and `agent_graph_run_steps` — state deltas, timing, errors, the full picture. OCR payloads are persisted in `document_ocr_results`, so operators can inspect extracted text and confidence directly in the dashboard.

## Repository Layout

```
arcnem-vision/
├── client/                 Flutter app — GenUI, Gemma, camera, gallery
│   ├── lib/screens/        Auth, camera, dashboard, loading
│   ├── lib/services/       Upload, document, GenUI, intent parsing
│   └── lib/catalog/        Custom widget catalog for AI-generated UI
├── server/                 Bun workspace
│   ├── packages/api/       Hono routes, middleware, auth, S3, Inngest
│   ├── packages/db/        Drizzle schema (23 tables), migrations, seed
│   ├── packages/dashboard/ React admin — workflow builder, doc viewer
│   └── packages/shared/    Env helpers
├── models/                 Go workspace
│   ├── agents/             Inngest handlers, LangGraph execution engine
│   ├── mcp/                MCP server — 7 tools (descriptions, OCR, embeddings, segmentation, search)
│   ├── db/                 GORM gen introspection (schema → Go models)
│   └── shared/             Common env loading
└── docs/                   Deep dives — embeddings, LangChain, LangGraph, GenUI
```

## Service Ports

| Service   | Host Port | Container Port |
|-----------|-----------|----------------|
| Postgres  | 5480      | 5432           |
| Redis     | 6381      | 6379           |
| API       | 3000      | —              |
| Dashboard | 3001      | —              |
| Agents    | 3020      | —              |
| MCP       | 3021      | —              |
