## 1. Aggregation logic

- [x] 1.1 Define the graph-mutating tool-name allowlist (`entity-create`, `entity-update`, `entity-relationship-create`) as a package-level constant/slice in `apps/server/domain/agents`
- [x] 1.2 Implement `aggregateRememberStatus(runID string, run *AgentRun, toolCalls []*AgentRunToolCall) map[string]any` (or equivalent) that:
  - derives `objects_created`, `objects_updated`, `relationships_created` counts
  - collects `created_object_ids` / `created_relationship_ids`
  - collects `discovered_types`
  - defensively skips tool calls with malformed/missing output fields
- [x] 1.3 Unit tests for the aggregator covering: successful creates, successful updates, relationship creates, mixed success/failure calls, zero-mutation run, malformed output JSON, unknown tool names ignored

## 2. MCP tool

- [x] 2.1 Add `ExecuteRememberStatus(ctx, projectID, args)` handler in `apps/server/domain/agents/mcp_tools.go`:
  - validate `run_id` present
  - call `FindRunByIDForProject` (reuse existing not-found/cross-project error shape from `ExecuteGetRunStatus`)
  - if run still running, return `status: running` with any error/partial info, skip full aggregation or mark counts as partial per spec
  - if completed/failed, fetch tool calls via existing `FindToolCallsByRunID`-style query and run the aggregator
  - assemble final `ToolResult` per spec (status, counts, ids, discovered_types, summary, error)
- [x] 2.2 Register `remember-status` tool definition (name, description, input schema requiring `run_id`) alongside other agent-run tools
- [x] 2.3 Add `remember-status` case to the MCP tool dispatch switch in `apps/server/domain/mcp/service.go` (`delegateAgentTool` or equivalent)
- [x] 2.4 Add `remember-status` to `toolRequiredScope` mapping with the same scope as `agent-run-status`

## 3. REST route (optional parity)

- [x] 3.1 Confirm whether a REST route is required for this change or MCP-only is sufficient for current callers; if required, add `GET /api/agents/runs/:runID/remember-status` handler reusing the same aggregation function
  - NOTE: implemented as `GET /api/projects/:projectId/agent-runs/:runId/remember-status` (project-scoped run group, matching the other run routes) reusing `buildRememberStatus`; the proposed flat `/api/agents/runs/:runID/...` path was not used
- [x] 3.2 Add route registration + OpenAPI/swagger annotation if REST route is added

## 4. Remember/forget response wording

- [x] 4.1 Update `remember`/`forget` MCP tool async-mode response text (`executeRemember`/`executeForget` in `apps/server/domain/mcp/service.go`) to reference `remember-status(run_id)` as the follow-up call

## 5. Tests and docs

- [ ] 5.1 Add e2e test: trigger `remember(mode=async)`, poll `remember-status` until `completed`, assert counts match what was actually created
- [ ] 5.2 Add e2e test: `remember-status` on unknown/cross-project `run_id` returns not-found error
- [x] 5.3 Update MCP README tool table/count to include `remember-status`
- [x] 5.4 Run `go build ./...`, `task lint`, `task test` before marking complete
