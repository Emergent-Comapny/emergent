## Why

`remember` (and `forget`) support `mode=async`, returning a `run_id` immediately and executing as a background agent that calls MCP tools (`entity-create`, `entity-update`, `entity-relationship-create`, etc.) to update the graph. Callers currently have no way to know when that background run finishes, whether it succeeded, or what it actually changed (which objects/relationships were created, merged, or failed). This makes async `remember` a black box — callers must guess a wait time or poll unrelated endpoints (`agent-run-status`) that report execution status but not graph impact.

## What Changes

- Add a `remember-status` MCP tool (and equivalent REST route) that accepts a `run_id` and returns:
  - `status`: `running` | `completed` | `failed`
  - `objects_created`, `objects_updated` (counts, derived from tool calls)
  - `relationships_created` (count)
  - `created_object_ids` / `created_relationship_ids`
  - `discovered_types` (distinct entity/relationship types touched)
  - `summary` (short human-readable line)
  - `error` (if failed)
- Implementation approach: aggregate `kb.agent_run_tool_calls` rows for the given `run_id`, filtering to graph-mutating tool names (`entity-create`, `entity-update`, `entity-relationship-create`) and parsing each call's `output` JSON for created/updated IDs and types. No new tables or schema migrations required — this data is already recorded per tool call.
- Update `remember`/`forget` MCP tool async-mode responses to mention `remember-status(run_id)` as the follow-up call (already partially done — refine wording once the tool exists).
- Out of scope for this change: linking the project journal (`kb.project_journal`) to `run_id` for a full mutation audit trail — deferred to a future change if deeper auditability is needed beyond tool-call aggregation.

## Capabilities

### New Capabilities
- `remember-status`: MCP tool + REST endpoint to check completion status and graph-mutation summary of an async `remember`/`forget` run, by aggregating that run's recorded tool calls.

### Modified Capabilities
(none — `remember`/`forget` response wording is an implementation-detail refinement, not a requirement change)

## Impact

- `apps/server/domain/mcp/service.go`: new `remember-status` case in tool dispatch, new `executeRememberStatus` handler.
- `apps/server/domain/agents/repository.go`: new query to fetch `agent_run_tool_calls` by `run_id` filtered by tool name (may reuse existing `ExecuteGetAgentRunToolCalls` query path).
- `apps/server/domain/chat/handler.go`: no changes required for aggregation itself; may adjust async response text.
- New REST route: `GET /api/agents/runs/{run_id}/remember-status` (or reuse existing agent-run routes with a query flag) — final routing decided in design.
- No DB schema changes.
- No breaking changes — purely additive.
