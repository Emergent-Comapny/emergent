## Context

`remember`/`forget` async mode runs the request as an agent (`agents.ExecuteRequest`), which executes via the standard agent executor and records every tool invocation to `kb.agent_run_tool_calls` (columns: `run_id`, `tool_name`, `input` jsonb, `output` jsonb, `status`, `step_number`). The existing `agent-run-status` tool (`ExecuteGetRunStatus`) already reports `run.Status` (`running`/`completed`/`failed`) and `run.ErrorMessage`/`run.Summary` from `kb.agent_runs`. The existing `agent-run-tool-calls` tool (`ExecuteGetAgentRunToolCalls`) already fetches all tool calls for a run. See proposal.md for motivation.

## Goals / Non-Goals

**Goals:**
- Reuse existing run-status lookup (`FindRunByIDForProject`) for the `status`/`error` fields — no duplication of that logic.
- Derive graph-mutation counts purely from already-persisted `agent_run_tool_calls` rows — zero schema changes, zero new background jobs.
- Keep the aggregation logic isolated in one function so it can be unit-tested against synthetic tool-call fixtures.

**Non-Goals:**
- Linking `kb.project_journal` entries to `run_id` for a full audit trail (deferred; tool-call aggregation is sufficient for a "what did this run do" summary).
- Real-time/streaming status updates (this is a pull-based status check, matching the existing `agent-run-status` pattern).
- Retrying or resuming failed runs — this tool is read-only reporting.

## Decisions

**Decision: Aggregate from `agent_run_tool_calls`, not a new results table.**
Alternative considered: have the remember-executing agent write a structured `ObjectExtractionResults`-style summary row at the end of its run (mirroring the extraction-job pattern). Rejected for now — remember doesn't go through the extraction-job queue, it's a live agent tool-calling loop, and adding a new completion-summary write path duplicates data already captured per-tool-call. Aggregating existing rows is simpler, requires no new write path, and works retroactively for already-completed runs.

**Decision: Filter tool calls by a fixed allowlist of graph-mutating tool names.**
`entity-create`, `entity-update`, `entity-relationship-create`, `save_note`, `manage_notes` (only when `input.action` is `create`/`update`/`promote_to_core`). Any tool call outside this list is ignored for counting purposes. This list SHALL be defined as a single constant/slice so it's easy to extend when new mutating tools are added.

**Decision: Parse counts from each call's `output` JSON, tolerating shape variance.**
Each tool's output already contains enough info (e.g., `entity-create` returns the created object's `id`/`type`; `entity-relationship-create` returns relationship id). The aggregator SHALL defensively extract fields (missing/malformed output → skip that call, don't fail the whole aggregation) since tool outputs are free-form JSON and not guaranteed to have every field the aggregator expects.

**Decision: Reuse `run_id` project-scoping check already used by `agent-run-status`/`agent-run-tool-calls`.**
Call `FindRunByIDForProject(ctx, runID, projectID)` first; if nil, return the same not-found error shape as the existing tools, for consistency and to avoid cross-project leaks.

**Decision: Expose as both an MCP tool and REST route, following the existing agent-run tool pattern.**
Add `remember-status` to the MCP tool dispatch table (naming matches `area-action` convention: `remember` was already accepted as an area for `remember`/`forget`; this is a status/read action on that area). REST route mirrors existing `agent-run-status`-style routes rather than inventing new conventions.

## Risks / Trade-offs

- [Tool output schemas vary per tool and may change over time] → Aggregator uses defensive/best-effort JSON field extraction; unit tests cover each known tool's output shape; unknown/malformed output is skipped rather than causing a failure.
- [Large runs with many tool calls could make aggregation slow] → `agent_run_tool_calls` queries are already indexed by `run_id` (existing `agent-run-tool-calls` tool has the same access pattern); no new performance concern introduced.
- [No audit trail linking to project journal] → Explicitly deferred (Non-Goals); if deeper auditability is needed later, a follow-up change can add `run_id` to journal metadata.
