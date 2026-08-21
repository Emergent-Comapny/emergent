package agents

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/emergent-company/emergent.memory/domain/mcp"
)

// ============================================================================
// remember-status aggregation
// ============================================================================

// graphMutatingToolNames is the allowlist of MCP tools that mutate the
// knowledge graph. remember-status aggregates ONLY these tool calls to compute
// a run's graph-mutation summary. Extend this slice when new mutating tools are
// added — calls to tools outside this list are ignored for counting purposes.
var graphMutatingToolNames = []string{
	"entity-create",
	"entity-update",
	"entity-relationship-create",
	"save_note",
	"manage_notes",
}

var graphMutatingToolSet = func() map[string]bool {
	set := make(map[string]bool, len(graphMutatingToolNames))
	for _, n := range graphMutatingToolNames {
		set[n] = true
	}
	return set
}()

// manageNotesMutatingActions are the manage_notes input actions that count as
// graph mutations. list/delete are read/no-op actions and are ignored.
var manageNotesMutatingActions = map[string]bool{
	"create":          true,
	"update":          true,
	"promote_to_core": true,
}

func isGraphMutatingTool(name string) bool {
	return graphMutatingToolSet[name]
}

// rememberStatusAggregation holds the derived graph-mutation summary for a run.
type rememberStatusAggregation struct {
	ObjectsCreated         int
	ObjectsUpdated         int
	RelationshipsCreated   int
	CreatedObjectIDs       []string
	CreatedRelationshipIDs []string
	DiscoveredTypes        []string
	// FailedToolCalls counts graph-mutating tool calls that did not complete.
	FailedToolCalls int
	// Summary is a short human-readable line describing the run's impact.
	Summary string
}

// aggregateRememberStatus derives objects_created/updated, relationships_created,
// created IDs, and discovered types from a run's recorded tool calls.
//
// Parsing is defensive: tool outputs are free-form JSON, so any call whose
// output is missing or malformed is skipped rather than failing the whole
// aggregation. Calls with status != "completed" are counted as failures and
// excluded from mutation counts.
func aggregateRememberStatus(toolCalls []*AgentRunToolCall) rememberStatusAggregation {
	agg := rememberStatusAggregation{
		CreatedObjectIDs:       []string{},
		CreatedRelationshipIDs: []string{},
		DiscoveredTypes:        []string{},
	}

	for _, tc := range toolCalls {
		if tc == nil || !isGraphMutatingTool(tc.ToolName) {
			continue
		}
		if tc.Status != "completed" {
			agg.FailedToolCalls++
			continue
		}

		switch tc.ToolName {
		case "entity-create":
			agg.parseEntityCreate(tc.Output)
		case "entity-update":
			agg.parseEntityUpdate(tc.Output)
		case "entity-relationship-create":
			agg.parseRelationshipCreate(tc.Output)
		case "save_note":
			agg.parseSaveNote(tc.Output)
		case "manage_notes":
			agg.parseManageNotes(tc)
		}
	}

	agg.buildSummary()
	return agg
}

// parseEntityCreate counts created entities (and any inline relationships
// created with them) from an entity-create tool call output.
func (a *rememberStatusAggregation) parseEntityCreate(output map[string]any) {
	// Batch form: {"success": n, "failed": m, "results": [{"success": true,
	// "entity": {"id": "...", "type": "...", "relationships": [...]}}]}
	if results := mapSlice(output, "results"); results != nil {
		for _, raw := range results {
			item, ok := raw.(map[string]any)
			if !ok || !mapBool(item, "success") {
				continue
			}
			ent := mapMap(item, "entity")
			if ent == nil {
				continue // malformed result — no entity payload, skip
			}
			a.ObjectsCreated++
			a.collectEntity(ent)
			if rels := mapSlice(ent, "relationships"); rels != nil {
				for _, rRaw := range rels {
					r, ok := rRaw.(map[string]any)
					if !ok {
						continue
					}
					a.RelationshipsCreated++
					if rid := mapStr(r, "id"); rid != "" {
						addID(&a.CreatedRelationshipIDs, rid)
					}
					if rt := mapStr(r, "type"); rt != "" {
						addType(&a.DiscoveredTypes, rt)
					}
				}
			}
		}
		return
	}

	// Single-entity fallback: {"success": true, "entity": {...}}
	if ent := mapMap(output, "entity"); ent != nil && mapBool(output, "success") {
		a.ObjectsCreated++
		a.collectEntity(ent)
	}
}

// parseEntityUpdate counts an entity-update tool call output. Calls with a
// missing/malformed entity payload are skipped (defensive best-effort parsing).
func (a *rememberStatusAggregation) parseEntityUpdate(output map[string]any) {
	if !mapBool(output, "success") {
		return
	}
	ent := mapMap(output, "entity")
	if ent == nil {
		return // malformed output — no entity payload, skip
	}
	a.ObjectsUpdated++
	if t := mapStr(ent, "type"); t != "" {
		addType(&a.DiscoveredTypes, t)
	}
}

// parseRelationshipCreate counts an entity-relationship-create tool call output.
// Relationship outputs vary: {"id": "..."} at top level, wrapped under
// "relationship"/"data", or as "relationship_id"/"relationship_type" keys.
func (a *rememberStatusAggregation) parseRelationshipCreate(output map[string]any) {
	rel := output
	if r := mapMap(output, "relationship"); r != nil {
		rel = r
	} else if r := mapMap(output, "data"); r != nil {
		rel = r
	}

	id := mapStr(rel, "id")
	if id == "" {
		id = mapStr(output, "relationship_id")
	}
	if id == "" {
		// No id extractable — only count when the output explicitly signals success.
		if !mapBool(output, "success") && !mapBool(rel, "success") {
			return
		}
	}
	a.RelationshipsCreated++
	if id != "" {
		addID(&a.CreatedRelationshipIDs, id)
	}
	t := mapStr(rel, "type")
	if t == "" {
		t = mapStr(output, "relationship_type")
	}
	if t != "" {
		addType(&a.DiscoveredTypes, t)
	}
}

// parseSaveNote counts a save_note tool call output. Two shapes are produced by
// the underlying tool: a new Note is created (plain text result "Note saved
// (ID: ...)") or an existing Note is merged/updated (entity-update shape).
func (a *rememberStatusAggregation) parseSaveNote(output map[string]any) {
	if ent := mapMap(output, "entity"); ent != nil && mapBool(output, "success") {
		a.ObjectsUpdated++
		if t := mapStr(ent, "type"); t != "" {
			addType(&a.DiscoveredTypes, t)
		}
		return
	}

	text := mapStr(output, "result")
	if text == "" {
		return // unknown shape — nothing to count
	}
	a.ObjectsCreated++
	addType(&a.DiscoveredTypes, "Note")
	if id := noteIDFromText(text); id != "" {
		addID(&a.CreatedObjectIDs, id)
	}
}

// parseManageNotes counts a manage_notes tool call only when its input action is
// a graph mutation (create/update/promote_to_core).
func (a *rememberStatusAggregation) parseManageNotes(tc *AgentRunToolCall) {
	action := mapStr(tc.Input, "action")
	if !manageNotesMutatingActions[action] {
		return
	}

	// update/promote_to_core delegate to the entity-update backend.
	if ent := mapMap(tc.Output, "entity"); ent != nil && mapBool(tc.Output, "success") {
		a.ObjectsUpdated++
		t := mapStr(ent, "type")
		if t == "" {
			t = "Note"
		}
		addType(&a.DiscoveredTypes, t)
		return
	}

	// create (and any other entity-create-shaped output).
	a.parseEntityCreate(tc.Output)
}

// collectEntity records a created entity's id/type from a slim entity payload.
func (a *rememberStatusAggregation) collectEntity(ent map[string]any) {
	if id := mapStr(ent, "id"); id != "" {
		addID(&a.CreatedObjectIDs, id)
	}
	if t := mapStr(ent, "type"); t != "" {
		addType(&a.DiscoveredTypes, t)
	}
}

// buildSummary produces a short human-readable summary line.
func (a *rememberStatusAggregation) buildSummary() {
	var parts []string
	if a.ObjectsCreated > 0 {
		parts = append(parts, fmt.Sprintf("created %d objects", a.ObjectsCreated))
	}
	if a.ObjectsUpdated > 0 {
		parts = append(parts, fmt.Sprintf("updated %d objects", a.ObjectsUpdated))
	}
	if a.RelationshipsCreated > 0 {
		parts = append(parts, fmt.Sprintf("created %d relationships", a.RelationshipsCreated))
	}
	if a.FailedToolCalls > 0 {
		parts = append(parts, fmt.Sprintf("%d tool call(s) failed", a.FailedToolCalls))
	}
	if len(parts) == 0 {
		a.Summary = "No graph changes were made by this run."
		return
	}
	a.Summary = strings.Join(parts, "; ") + "."
}

// toMap renders the aggregation with the JSON field names exposed by
// remember-status. Slices are guaranteed non-nil so they marshal as [].
func (a rememberStatusAggregation) toMap() map[string]any {
	if a.CreatedObjectIDs == nil {
		a.CreatedObjectIDs = []string{}
	}
	if a.CreatedRelationshipIDs == nil {
		a.CreatedRelationshipIDs = []string{}
	}
	if a.DiscoveredTypes == nil {
		a.DiscoveredTypes = []string{}
	}
	return map[string]any{
		"objects_created":          a.ObjectsCreated,
		"objects_updated":          a.ObjectsUpdated,
		"relationships_created":    a.RelationshipsCreated,
		"created_object_ids":       a.CreatedObjectIDs,
		"created_relationship_ids": a.CreatedRelationshipIDs,
		"discovered_types":         a.DiscoveredTypes,
		"summary":                  a.Summary,
	}
}

// Defensive map accessors — tolerate missing/wrong-typed fields.

func mapStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func mapBool(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func mapMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

func mapSlice(m map[string]any, key string) []any {
	if v, ok := m[key].([]any); ok {
		return v
	}
	return nil
}

// addID appends a non-empty id, deduplicating.
func addID(ids *[]string, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	for _, existing := range *ids {
		if existing == id {
			return
		}
	}
	*ids = append(*ids, id)
}

// addType appends a non-empty type, deduplicating.
func addType(types *[]string, t string) {
	t = strings.TrimSpace(t)
	if t == "" {
		return
	}
	for _, existing := range *types {
		if existing == t {
			return
		}
	}
	*types = append(*types, t)
}

var noteIDPattern = regexp.MustCompile(`ID:\s*([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})`)

// noteIDFromText extracts a UUID from save_note's plain-text result, best-effort.
func noteIDFromText(text string) string {
	if m := noteIDPattern.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
}

// rememberStatusFromRunStatus maps a run's lifecycle status to the three-state
// remember-status vocabulary (running/completed/failed).
func rememberStatusFromRunStatus(s AgentRunStatus) (status string, terminal bool, errMsg string) {
	switch s {
	case RunStatusSuccess:
		return "completed", true, ""
	case RunStatusError:
		return "failed", true, ""
	case RunStatusSkipped:
		return "completed", true, ""
	case RunStatusCancelled:
		return "failed", true, "run was cancelled"
	default: // submitted, working, input-required, cancelling
		return "running", false, ""
	}
}

// ============================================================================
// MCP tool handler
// ============================================================================

// ExecuteRememberStatus reports the completion state and graph-mutation summary
// of an async remember/forget run, derived from the run's recorded tool calls.
// Follows the same validate → lookup → respond pattern as ExecuteGetRunStatus.
func (h *MCPToolHandler) ExecuteRememberStatus(ctx context.Context, projectID string, args map[string]any) (*mcp.ToolResult, error) {
	runID, _ := args["run_id"].(string)
	if runID == "" {
		return errResult("run_id is required")
	}

	run, err := h.repo.FindRunByIDForProject(ctx, runID, projectID)
	if err != nil {
		return errResult("failed to get run status: " + err.Error())
	}
	if run == nil {
		return errResult(fmt.Sprintf("agent run not found: %s", runID))
	}

	result := map[string]any{"run_id": run.ID}

	status, terminal, terminalErr := rememberStatusFromRunStatus(run.Status)
	result["status"] = status

	// Still running: report partial counts from tool calls recorded so far.
	if !terminal {
		result["partial"] = true
		if toolCalls, err := h.repo.FindToolCallsByRunID(ctx, runID); err == nil {
			agg := aggregateRememberStatus(toolCalls)
			for k, v := range agg.toMap() {
				result[k] = v
			}
			result["summary"] = "Run is still in progress — counts are partial. " + agg.Summary
		} else {
			result["summary"] = "Run is still in progress — no tool-call data available yet."
		}
		return wrapResult(result)
	}

	toolCalls, err := h.repo.FindToolCallsByRunID(ctx, runID)
	if err != nil {
		return errResult("failed to get run tool calls: " + err.Error())
	}
	agg := aggregateRememberStatus(toolCalls)
	for k, v := range agg.toMap() {
		result[k] = v
	}

	if status == "failed" {
		errMsg := terminalErr
		if run.ErrorMessage != nil && *run.ErrorMessage != "" {
			errMsg = *run.ErrorMessage
		}
		if errMsg == "" {
			errMsg = "run failed"
		}
		result["error"] = errMsg
	}

	return wrapResult(result)
}
