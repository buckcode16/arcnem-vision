---
title: Dashboard Operations
description: Manage projects, devices, API keys, uploads, workflows, and live runs from the dashboard.
---

The dashboard (`server/packages/dashboard`) is the operator control room for Arcnem Vision. It covers configuration, ad-hoc uploads, and live run inspection without redeploying code.

![Dashboard — projects, devices, and API keys](/dashboard-projects.png)

## Tabs at a glance

- **Projects & Devices**: create projects, register devices, assign default workflows, and issue or rotate device API keys.
- **Workflow Library**: create/edit graph workflows.
- **Docs**: browse seeded or live uploads, upload directly from the dashboard, inspect related segmented outputs, and queue workflows against any document.
- **Runs**: monitor execution history with live updates as runs start, advance, and finish.

## Projects, devices, and API keys

1. Create a project.
2. Add devices inside that project and choose the saved workflow each device should run by default.
3. Create a device API key when the device is ready to upload.

Notes:

- The generated secret is shown once; afterward the dashboard keeps only the public identifier.
- Existing keys can be renamed, disabled, or deleted without changing the device record.
- Workflow assignment is per device, so one project can mix standard ingestion, quality review, and segmentation devices.

## Building workflows

![Workflow Library with document and segmentation workflows](/dashboard-workflows.png)

Open **Workflow Library** and create a new workflow or edit an existing one.

### Node types

| Node type | Purpose | Required config |
|---|---|---|
| `worker` | ReAct-style worker agent | Model, optional system message, optional tools |
| `supervisor` | Orchestrates worker members | Model, `config.members` (worker node keys) |
| `tool` | Single tool invocation node | Exactly one tool, optional IO mapping |

### Assigning tools to workers

1. Select a `worker` node.
2. In **Assigned tools**, toggle one or more tools.
3. Save the workflow.

Workers can hold multiple tool assignments.

### Tool node mappings

For `tool` nodes, map tool schema fields to graph state keys:

- `input_mapping`: graph state -> tool input
- `output_mapping`: tool output -> graph state

Literal input values can be passed with `_const:` (for example `_const:image/png`).

Before save, the canvas enforces unique node keys, model requirements, one tool per tool node, valid supervisor membership, and entry-to-`END` reachability.

Segmentation flows are ordinary workflows. The difference is the tool they call: versioned segmentation models are registered in the database and invoked through MCP.

## Docs: search, upload, and segmented results

![Docs tab with newer seeded images](/dashboard-docs.png)

- Search by meaning uses semantic ranking first and falls back to lexical matching if there is no embedding seed.
- **Add From Dashboard** uploads a one-off image into a project without binding it to a device.
- Click any document to choose a different workflow and queue it without changing the source device's saved assignment.

![Selected document with a related segmented result](/dashboard-docs-segmentation-detail.png)

- Derived segmented images stay attached to the source document and show the model label plus prompt used to create them.
- Segmentation outputs are stored as real documents, so they can be described, browsed, and reused in later workflows.

## Runs and realtime updates

![Runs tab with expanded run details](/dashboard-run-detail.png)

- The dashboard subscribes to `/api/realtime/dashboard` via Server-Sent Events.
- **Docs** refresh when documents are created, descriptions are written, or segmentation results are persisted.
- **Runs** refresh when a run is created, when steps change, and when the run finishes.
- Expand a run to inspect initial state, per-step state deltas, final state, timing, and errors.

## Segmentation workflows

- Versioned models in the `models` table can be marked as segmentation models and called from MCP.
- `create_document_segmentation` stores both the raw result payload and any derived segmented image.
- The seed includes language segmentation and semantic segmentation showcase workflows so you can test the dashboard end to end immediately.
