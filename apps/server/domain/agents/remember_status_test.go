package agents

import (
	"reflect"
	"strings"
	"testing"
)

// tc builds an AgentRunToolCall with completed status.
func tc(toolName string, input, output map[string]any) *AgentRunToolCall {
	return &AgentRunToolCall{ToolName: toolName, Input: input, Output: output, Status: "completed"}
}

func TestAggregateRememberStatus_SuccessfulCreates(t *testing.T) {
	calls := []*AgentRunToolCall{
		tc("entity-create", nil, map[string]any{
			"success": float64(2),
			"failed":  float64(0),
			"total":   float64(2),
			"results": []any{
				map[string]any{
					"success": true,
					"entity":  map[string]any{"id": "obj-1", "type": "Person"},
					"index":   float64(0),
				},
				map[string]any{
					"success": true,
					"entity":  map[string]any{"id": "obj-2", "type": "Company", "key": "acme"},
					"index":   float64(1),
				},
			},
		}),
	}

	agg := aggregateRememberStatus(calls)

	if agg.ObjectsCreated != 2 {
		t.Errorf("ObjectsCreated = %d, want 2", agg.ObjectsCreated)
	}
	if agg.ObjectsUpdated != 0 || agg.RelationshipsCreated != 0 {
		t.Errorf("unexpected counts: updated=%d rels=%d", agg.ObjectsUpdated, agg.RelationshipsCreated)
	}
	wantIDs := []string{"obj-1", "obj-2"}
	if !reflect.DeepEqual(agg.CreatedObjectIDs, wantIDs) {
		t.Errorf("CreatedObjectIDs = %v, want %v", agg.CreatedObjectIDs, wantIDs)
	}
	wantTypes := []string{"Person", "Company"}
	if !reflect.DeepEqual(agg.DiscoveredTypes, wantTypes) {
		t.Errorf("DiscoveredTypes = %v, want %v", agg.DiscoveredTypes, wantTypes)
	}
}

func TestAggregateRememberStatus_InlineRelationships(t *testing.T) {
	calls := []*AgentRunToolCall{
		tc("entity-create", nil, map[string]any{
			"results": []any{
				map[string]any{
					"success": true,
					"entity": map[string]any{
						"id":   "obj-1",
						"type": "Decision",
						"relationships": []any{
							map[string]any{"id": "rel-1", "type": "about", "target_id": "obj-9"},
						},
					},
				},
			},
		}),
	}

	agg := aggregateRememberStatus(calls)

	if agg.ObjectsCreated != 1 {
		t.Errorf("ObjectsCreated = %d, want 1", agg.ObjectsCreated)
	}
	if agg.RelationshipsCreated != 1 {
		t.Errorf("RelationshipsCreated = %d, want 1", agg.RelationshipsCreated)
	}
	if !reflect.DeepEqual(agg.CreatedRelationshipIDs, []string{"rel-1"}) {
		t.Errorf("CreatedRelationshipIDs = %v, want [rel-1]", agg.CreatedRelationshipIDs)
	}
	if !reflect.DeepEqual(agg.DiscoveredTypes, []string{"Decision", "about"}) {
		t.Errorf("DiscoveredTypes = %v, want [Decision about]", agg.DiscoveredTypes)
	}
}

func TestAggregateRememberStatus_SuccessfulUpdates(t *testing.T) {
	calls := []*AgentRunToolCall{
		tc("entity-update", nil, map[string]any{
			"success": true,
			"entity":  map[string]any{"id": "obj-1", "type": "Person"},
			"message": "Entity updated successfully",
		}),
		tc("entity-update", nil, map[string]any{
			"success": false,
			"error":   "boom",
		}),
	}

	agg := aggregateRememberStatus(calls)

	if agg.ObjectsUpdated != 1 {
		t.Errorf("ObjectsUpdated = %d, want 1", agg.ObjectsUpdated)
	}
	if agg.FailedToolCalls != 0 {
		t.Errorf("FailedToolCalls = %d, want 0 (failed output is not a failed call)", agg.FailedToolCalls)
	}
	if !reflect.DeepEqual(agg.DiscoveredTypes, []string{"Person"}) {
		t.Errorf("DiscoveredTypes = %v, want [Person]", agg.DiscoveredTypes)
	}
}

func TestAggregateRememberStatus_RelationshipCreates(t *testing.T) {
	shapes := []map[string]any{
		{"id": "rel-1", "type": "works_at"},
		{"relationship": map[string]any{"id": "rel-2", "type": "reports_to"}},
		{"relationship_id": "rel-3", "relationship_type": "mentors"},
		{"data": map[string]any{"id": "rel-4"}},
	}

	var calls []*AgentRunToolCall
	for _, out := range shapes {
		calls = append(calls, tc("entity-relationship-create", nil, out))
	}

	agg := aggregateRememberStatus(calls)

	if agg.RelationshipsCreated != 4 {
		t.Errorf("RelationshipsCreated = %d, want 4", agg.RelationshipsCreated)
	}
	want := []string{"rel-1", "rel-2", "rel-3", "rel-4"}
	if !reflect.DeepEqual(agg.CreatedRelationshipIDs, want) {
		t.Errorf("CreatedRelationshipIDs = %v, want %v", agg.CreatedRelationshipIDs, want)
	}
	if !reflect.DeepEqual(agg.DiscoveredTypes, []string{"works_at", "reports_to", "mentors"}) {
		t.Errorf("DiscoveredTypes = %v", agg.DiscoveredTypes)
	}
}

func TestAggregateRememberStatus_MixedSuccessFailure(t *testing.T) {
	calls := []*AgentRunToolCall{
		tc("entity-create", nil, map[string]any{
			"results": []any{
				map[string]any{"success": true, "entity": map[string]any{"id": "ok-1", "type": "Person"}},
				map[string]any{"success": false, "error": "duplicate"},
			},
		}),
		{
			ToolName: "entity-create",
			Output: map[string]any{
				"results": []any{
					map[string]any{"success": true, "entity": map[string]any{"id": "ok-2", "type": "Task"}},
				},
			},
			Status: "failed", // whole call failed (toolErr recorded)
		},
		{
			ToolName: "entity-update",
			Output:   map[string]any{"success": true, "entity": map[string]any{"id": "u-1", "type": "Person"}},
			Status:   "completed",
		},
	}

	agg := aggregateRememberStatus(calls)

	if agg.ObjectsCreated != 1 {
		t.Errorf("ObjectsCreated = %d, want 1 (failed call excluded, failed result excluded)", agg.ObjectsCreated)
	}
	if agg.ObjectsUpdated != 1 {
		t.Errorf("ObjectsUpdated = %d, want 1", agg.ObjectsUpdated)
	}
	if agg.FailedToolCalls != 1 {
		t.Errorf("FailedToolCalls = %d, want 1", agg.FailedToolCalls)
	}
	if !reflect.DeepEqual(agg.CreatedObjectIDs, []string{"ok-1"}) {
		t.Errorf("CreatedObjectIDs = %v, want [ok-1]", agg.CreatedObjectIDs)
	}
	if !strings.Contains(agg.Summary, "1 tool call(s) failed") {
		t.Errorf("Summary = %q, want failure note", agg.Summary)
	}
}

func TestAggregateRememberStatus_ZeroMutations(t *testing.T) {
	calls := []*AgentRunToolCall{
		tc("entity-query", nil, map[string]any{"data": []any{}}),
		tc("search-hybrid", nil, map[string]any{"data": []any{}}),
		tc("recall_notes", nil, map[string]any{"result": "nothing found"}),
	}

	agg := aggregateRememberStatus(calls)

	if agg.ObjectsCreated != 0 || agg.ObjectsUpdated != 0 || agg.RelationshipsCreated != 0 {
		t.Errorf("expected zero counts, got %+v", agg)
	}
	if len(agg.CreatedObjectIDs) != 0 || len(agg.CreatedRelationshipIDs) != 0 {
		t.Errorf("expected no ids, got %v / %v", agg.CreatedObjectIDs, agg.CreatedRelationshipIDs)
	}
	if agg.Summary != "No graph changes were made by this run." {
		t.Errorf("Summary = %q", agg.Summary)
	}
}

func TestAggregateRememberStatus_MalformedOutputSkipped(t *testing.T) {
	calls := []*AgentRunToolCall{
		// results is a string, not an array
		tc("entity-create", nil, map[string]any{"results": "not-an-array", "success": 1}),
		// entity is a string, not a map
		tc("entity-update", nil, map[string]any{"success": true, "entity": "oops"}),
		// relationship data is malformed
		tc("entity-relationship-create", nil, map[string]any{"relationship": "nope"}),
		// completely empty output
		tc("entity-create", nil, map[string]any{}),
		// nil output
		{ToolName: "save_note", Status: "completed", Output: nil},
	}

	agg := aggregateRememberStatus(calls)

	if agg.ObjectsCreated != 0 || agg.ObjectsUpdated != 0 || agg.RelationshipsCreated != 0 {
		t.Errorf("expected malformed calls to be skipped, got %+v", agg)
	}
	if len(agg.CreatedObjectIDs) != 0 || len(agg.CreatedRelationshipIDs) != 0 || len(agg.DiscoveredTypes) != 0 {
		t.Errorf("expected no ids/types from malformed output, got %+v", agg)
	}
}

func TestAggregateRememberStatus_UnknownToolsIgnored(t *testing.T) {
	calls := []*AgentRunToolCall{
		tc("web-fetch", nil, map[string]any{"result": "page content"}),
		tc("trigger_agent", nil, map[string]any{"run_id": "x"}),
		tc("agent-run-status", nil, map[string]any{"status": "completed"}),
	}

	agg := aggregateRememberStatus(calls)

	if agg.ObjectsCreated != 0 || agg.ObjectsUpdated != 0 || agg.RelationshipsCreated != 0 {
		t.Errorf("unknown tools should be ignored, got %+v", agg)
	}
}

func TestAggregateRememberStatus_SaveNote(t *testing.T) {
	calls := []*AgentRunToolCall{
		// New note created: plain-text result with an ID.
		tc("save_note", nil, map[string]any{
			"result": "Note saved (ID: 3f2a1b4c-9d8e-4f7a-b6c5-1e2d3f4a5b6c). Category: fact. Confidence: 0.7.",
		}),
		// Dedup path: merged into an existing Note (entity-update shape).
		tc("save_note", nil, map[string]any{
			"success": true,
			"entity":  map[string]any{"id": "note-2", "type": "Note"},
		}),
	}

	agg := aggregateRememberStatus(calls)

	if agg.ObjectsCreated != 1 {
		t.Errorf("ObjectsCreated = %d, want 1", agg.ObjectsCreated)
	}
	if agg.ObjectsUpdated != 1 {
		t.Errorf("ObjectsUpdated = %d, want 1", agg.ObjectsUpdated)
	}
	if !reflect.DeepEqual(agg.CreatedObjectIDs, []string{"3f2a1b4c-9d8e-4f7a-b6c5-1e2d3f4a5b6c"}) {
		t.Errorf("CreatedObjectIDs = %v", agg.CreatedObjectIDs)
	}
	if !reflect.DeepEqual(agg.DiscoveredTypes, []string{"Note"}) {
		t.Errorf("DiscoveredTypes = %v, want [Note]", agg.DiscoveredTypes)
	}
}

func TestAggregateRememberStatus_ManageNotesActions(t *testing.T) {
	calls := []*AgentRunToolCall{
		// update → counts as update
		tc("manage_notes",
			map[string]any{"action": "update", "note_id": "n-1"},
			map[string]any{"success": true, "entity": map[string]any{"id": "n-1", "type": "Note"}}),
		// promote_to_core → counts as update
		tc("manage_notes",
			map[string]any{"action": "promote_to_core", "note_id": "n-2"},
			map[string]any{"success": true, "entity": map[string]any{"id": "n-2", "type": "Note"}}),
		// list → ignored (read action)
		tc("manage_notes", map[string]any{"action": "list"}, map[string]any{"data": []any{}}),
		// delete → ignored (not a create/update)
		tc("manage_notes", map[string]any{"action": "delete", "note_id": "n-3"}, map[string]any{"success": true}),
		// missing action → ignored
		tc("manage_notes", map[string]any{}, map[string]any{"success": true}),
	}

	agg := aggregateRememberStatus(calls)

	if agg.ObjectsUpdated != 2 {
		t.Errorf("ObjectsUpdated = %d, want 2", agg.ObjectsUpdated)
	}
	if agg.ObjectsCreated != 0 {
		t.Errorf("ObjectsCreated = %d, want 0", agg.ObjectsCreated)
	}
	if !reflect.DeepEqual(agg.DiscoveredTypes, []string{"Note"}) {
		t.Errorf("DiscoveredTypes = %v, want [Note]", agg.DiscoveredTypes)
	}
}

func TestAggregateRememberStatus_NilAndNilCalls(t *testing.T) {
	agg := aggregateRememberStatus(nil)
	if agg.ObjectsCreated != 0 || agg.Summary == "" {
		t.Errorf("nil input should yield zero counts + summary, got %+v", agg)
	}

	agg = aggregateRememberStatus([]*AgentRunToolCall{nil, tc("entity-create", nil, map[string]any{})})
	if agg.ObjectsCreated != 0 {
		t.Errorf("nil calls should be skipped, got %+v", agg)
	}
}

func TestRememberStatusFromRunStatus(t *testing.T) {
	tests := []struct {
		status   AgentRunStatus
		want     string
		terminal bool
	}{
		{RunStatusSuccess, "completed", true},
		{RunStatusError, "failed", true},
		{RunStatusSkipped, "completed", true},
		{RunStatusCancelled, "failed", true},
		{RunStatusQueued, "running", false},
		{RunStatusRunning, "running", false},
		{RunStatusPaused, "running", false},
		{RunStatusCancelling, "running", false},
	}
	for _, tt := range tests {
		got, terminal, _ := rememberStatusFromRunStatus(tt.status)
		if got != tt.want || terminal != tt.terminal {
			t.Errorf("rememberStatusFromRunStatus(%q) = (%q, %v), want (%q, %v)",
				tt.status, got, terminal, tt.want, tt.terminal)
		}
	}
}

func TestGraphMutatingToolAllowlist(t *testing.T) {
	for _, name := range []string{"entity-create", "entity-update", "entity-relationship-create", "save_note", "manage_notes"} {
		if !isGraphMutatingTool(name) {
			t.Errorf("expected %q in allowlist", name)
		}
	}
	for _, name := range []string{"entity-query", "entity-delete", "recall_notes", "web-fetch", "remember"} {
		if isGraphMutatingTool(name) {
			t.Errorf("expected %q NOT in allowlist", name)
		}
	}
}
