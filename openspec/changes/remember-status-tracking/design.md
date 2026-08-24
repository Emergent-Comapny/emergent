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
`entity-create`, `entity-update`, `entity-relationship-create`. Any tool call outside this list is ignored for counting purposes. This list SHALL be defined as a single constant/slice so it's easy to extend when new mutating tools are added.

**Decision (discovered during live testing): follow `queue-reextraction` to its async extraction job.**
The real `remember` flow does NOT call `entity-create` directly: the agent classifies the message into a document and calls `queue-reextraction` (`document_id`, `schema_id` → `{"job_id": "..."}`), which queues an `ObjectExtractionJob`. That job runs asynchronously AFTER the agent run reports `completed` and performs the actual graph mutations. Without following it, `remember-status` under-reports (`objects_created: 0`) for the primary flow. The aggregator therefore also:
- extracts `job_id` from each completed `queue-reextraction` call,
- resolves each job via `ObjectExtractionJobsService.FindByID` (projected through `mcp.ExtractionJobInfo` because `agents` cannot import `extraction` — `agents → extraction → projects → agents` is an import cycle; the finder is injected into the agents handler from the composition root under the Agents feature flag),
- reports `status: running` while any resolved job is pending/processing (remembering isn't done until extraction finishes),
- merges completed jobs' persisted results (`ObjectsCreated`, `RelationshipsCreated`, `DiscoveredTypes`, `CreatedObjectIDs`; `ObjectsMerged` is not persisted, so update counts are tool-call-derived only) into the same totals as the tool-call aggregation,
- surfaces failed/dead-lettered jobs in `error`/`summary`; a failed extraction job marks the overall status `failed` even when the agent run itself completed, because the requested graph mutations never happened.

**Decision: report embedding generation readiness as a separate signal, not part of the completion gate.**
When an `ObjectExtractionJob` completes it enqueues `kb.graph_embedding_jobs` rows for the created objects — a third async stage after (1) agent run and (2) extraction job. Until those finish, the created objects are not semantically searchable via recall/search (confirmed live: recall missed notes while embeddings were still processing). `remember-status` therefore resolves the created objects' embedding-job status (`GraphEmbeddingJobsService.FindByObjectIDs`, projected through `mcp.EmbeddingJobInfo` via an adapter injected like the extraction finder) and reports `embeddings_pending`, `embeddings_failed`, and `embeddings_ready` (true when zero pending) plus a summary note when embeddings are still processing. The overall `status` is deliberately NOT gated on embeddings — "did remember persist the memory" (graph mutation) is complete once extraction finishes, while "is it recallable yet" (embeddings) is an independently retryable concern; conflating them would make `status` depend on an unrelated background queue and flip-flop for callers that only care about persistence. Permanently failed embedding jobs are surfaced via `embeddings_failed` and a summary note without failing the overall status. When the embedding finder is not configured (nil), the fields are omitted and a warning is logged — same graceful degradation as the extraction finder.

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
