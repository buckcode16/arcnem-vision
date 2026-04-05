---
title: Dashboard Operations
description: Manage projects, devices, API keys, uploads, workflows, and live runs from the dashboard.
---

The dashboard (`server/packages/dashboard`) is the operator control room for Arcnem Vision. It covers configuration, ad-hoc uploads, and live run inspection without redeploying code.

![Dashboard — projects, devices, and API keys](/dashboard-projects.png)

## Tabs at a glance

- **Projects & Devices**: create projects, register devices, assign default workflows, and issue or rotate device API keys.
- **Workflow Library**: create/edit graph workflows.
- **Docs**: browse seeded or live uploads, upload directly from the dashboard, inspect related OCR and segmentation outputs, and queue workflows against any document.
- **Runs**: monitor execution history with live updates as runs start, advance, and finish.

## Projects, devices, and API keys

1. Create a project.
2. Add devices inside that project and choose the saved workflow each device should run by default.
3. Create a device API key when the device is ready to upload.

Notes:

- The generated secret is shown once; afterward the dashboard keeps only the public identifier.
- Existing keys can be renamed, disabled, or deleted without changing the device record.
- Workflow assignment is per device, so one project can mix standard ingestion, OCR review, quality review, and segmentation devices.

## Building workflows

![Workflow Library with document and segmentation workflows](/dashboard-workflows.png)

Open **Workflow Library** and create a new workflow or edit an existing one.

### Node types

| Node type | Purpose | Required config |
|---|---|---|
| `worker` | ReAct-style worker agent | Model, optional system message, optional tools |
| `supervisor` | Orchestrates worker members | Model, `config.members` (worker node keys) |
| `condition` | Deterministic branch on state | `config.source_key`, `operator`, `value`, `true_target`, `false_target`, optional `case_sensitive` |
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

### Condition node routing

Use a `condition` node when the branch can be expressed as a simple state check
instead of an LLM decision.

- `source_key`: the state key to inspect, such as `ocr_text`
- `operator`: `contains` or `equals`
- `value`: the string to compare against
- `true_target` / `false_target`: target node keys or `END`
- `case_sensitive`: optional boolean
- `outputKey`: optional place to store the boolean match result for later steps

Condition nodes do not select a model or tools. Their outgoing edges are
managed: the canvas expects exactly two edges, and they must match the
configured `true_target` and `false_target`.

Before save, the canvas enforces unique node keys, model requirements, one tool per tool node, valid supervisor membership, valid condition routing targets, exactly two managed edges for each condition node, and entry-to-`END` reachability.

Segmentation flows are ordinary workflows. The difference is the tool they call: versioned segmentation models are registered in the database and invoked through MCP. OCR flows work the same way, except the tool is `create_document_ocr` and the result stays attached to the source document as persisted text plus metadata.

## Docs: search, upload, OCR, and segmented results

![Docs tab with newer seeded images](/dashboard-docs.png)

- Search by meaning uses semantic ranking first and falls back to lexical matching if there is no embedding seed.
- **Add From Dashboard** uploads a one-off image into a project without binding it to a device.
- Click any document to choose a different workflow and queue it without changing the source device's saved assignment.

![Selected document with a related segmented result](/dashboard-docs-segmentation-detail.png)

- Related OCR results stay attached to the source document and show the model label, extracted text, average confidence when available, and the raw normalized payload.
- Derived segmented images stay attached to the source document and show the model label plus prompt used to create them.
- Segmentation outputs are stored as real documents, so they can be described, browsed, and reused in later workflows.
- OCR outputs are not separate documents; they are stored as rows in `document_ocr_results` so operators can review text extraction without creating duplicate media objects.

## Runs and realtime updates

![Runs tab with expanded run details](/dashboard-run-detail.png)

- The dashboard subscribes to `/api/realtime/dashboard` via Server-Sent Events.
- **Docs** refresh when documents are created, OCR results are written, descriptions are written, or segmentation results are persisted.
- **Runs** refresh when a run is created, when steps change, and when the run finishes.
- Expand a run to inspect initial state, per-step state deltas, final state, timing, and errors.

## OCR and segmentation workflows

- `create_document_ocr` persists normalized text, the raw OCR payload, and optional average confidence for a document.
- Use a `condition` node when OCR routing can be handled with a deterministic rule like "contains URGENT".
- Use a `supervisor` when OCR needs semantic judgment, such as routing to billing vs operations specialists.
- Versioned models in the `models` table can be marked as segmentation models and called from MCP.
- `create_document_segmentation` stores both the raw result payload and any derived segmented image.
- The seed includes `OCR Keyword Condition Router`, `OCR Review Supervisor`, language segmentation, and semantic segmentation showcase workflows so you can test the dashboard end to end immediately.
