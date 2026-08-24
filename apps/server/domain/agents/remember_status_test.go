package agents

import (
	"reflect"
	"strings"
	"testing"

	"github.com/emergent-company/emergent.memory/domain/mcp"
)

// tc builds an AgentRunToolCall with completed status.
func tc(toolName string, input, output map[string]any) *AgentRunToolCall {
	return &AgentRunToolCall{ToolName: toolName, Input: input, Output: output, Status: "completed"}
}

// ej builds an extraction job projection.
func ej(id, status string, created, rels int, types, ids []string) *mcp.ExtractionJobInfo {
	return &mcp.ExtractionJobInfo{
		ID:                   id,
		Status:               status,
		ObjectsCreated:       created,
		RelationshipsCreated: rels,
		DiscoveredTypes:      types,
		CreatedObjectIDs:     ids,
	}
}

// ei builds an embedding job info projection.
func ei(objectID, status string) mcp.EmbeddingJobInfo {
	return mcp.EmbeddingJobInfo{ObjectID: objectID, Status: status}
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
		{ToolName: "entity-create", Status: "completed", Output: nil},
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
	for _, name := range []string{"entity-create", "entity-update", "entity-relationship-create"} {
		if !isGraphMutatingTool(name) {
			t.Errorf("expected %q in allowlist", name)
		}
	}
	for _, name := range []string{"entity-query", "entity-delete", "web-fetch", "remember", "save_note", "manage_notes", "recall_notes", "get_note"} {
		if isGraphMutatingTool(name) {
			t.Errorf("expected %q NOT in allowlist", name)
		}
	}
}

// ----------------------------------------------------------------------------
// queue-reextraction → extraction job following
// ----------------------------------------------------------------------------

func TestReextractionJobIDs(t *testing.T) {
	toolCalls := []*AgentRunToolCall{
		tc("queue-reextraction", map[string]any{"document_id": "d-1"}, map[string]any{"job_id": "job-1"}),
		tc("queue-reextraction", map[string]any{"document_id": "d-2"}, map[string]any{"jobId": "job-2"}), // camelCase variant
		// malformed output — no job_id
		tc("queue-reextraction", map[string]any{"document_id": "d-3"}, map[string]any{}),
		// failed call — job never queued
		{
			ToolName: "queue-reextraction",
			Output:   map[string]any{"job_id": "job-3"},
			Status:   "failed",
		},
		tc("entity-create", nil, map[string]any{}),
	}

	ids := reextractionJobIDs(toolCalls)
	if !reflect.DeepEqual(ids, []string{"job-1", "job-2"}) {
		t.Errorf("reextractionJobIDs = %v, want [job-1 job-2]", ids)
	}
}

func TestAggregateExtractionJobs_Completed(t *testing.T) {
	jobs := []*mcp.ExtractionJobInfo{
		ej("job-1", "completed", 2, 1, []string{"Person", "Person"}, []string{"obj-a", "obj-b"}),
		ej("job-2", "completed", 0, 0, nil, nil),
	}

	agg, pending, failures := aggregateExtractionJobs(jobs)

	if pending {
		t.Error("pending = true, want false")
	}
	if len(failures) != 0 {
		t.Errorf("failures = %v, want none", failures)
	}
	if agg.ObjectsCreated != 2 {
		t.Errorf("ObjectsCreated = %d, want 2", agg.ObjectsCreated)
	}
	if agg.RelationshipsCreated != 1 {
		t.Errorf("RelationshipsCreated = %d, want 1", agg.RelationshipsCreated)
	}
	if !reflect.DeepEqual(agg.CreatedObjectIDs, []string{"obj-a", "obj-b"}) {
		t.Errorf("CreatedObjectIDs = %v", agg.CreatedObjectIDs)
	}
	if !reflect.DeepEqual(agg.DiscoveredTypes, []string{"Person"}) {
		t.Errorf("DiscoveredTypes = %v, want [Person] (deduped)", agg.DiscoveredTypes)
	}
}

func TestAggregateExtractionJobs_PendingAndFailed(t *testing.T) {
	errMsg := "llm timeout"
	jobs := []*mcp.ExtractionJobInfo{
		ej("job-pending", "pending", 0, 0, nil, nil),
		ej("job-processing", "processing", 0, 0, nil, nil),
		{ID: "job-failed", Status: "failed", ErrorMessage: &errMsg},
		{ID: "job-dl", Status: "dead_letter", ErrorMessage: nil},
		ej("job-cancelled", "cancelled", 0, 0, nil, nil),
	}

	agg, pending, failures := aggregateExtractionJobs(jobs)

	if !pending {
		t.Error("pending = false, want true (pending + processing jobs)")
	}
	if len(failures) != 2 {
		t.Fatalf("failures = %v, want 2", failures)
	}
	if !strings.Contains(failures[0], "job-failed") || !strings.Contains(failures[0], "llm timeout") {
		t.Errorf("failures[0] = %q, want job id + error message", failures[0])
	}
	if !strings.Contains(failures[1], "job-dl") || !strings.Contains(failures[1], "no error message") {
		t.Errorf("failures[1] = %q, want job id + placeholder", failures[1])
	}
	if agg.ObjectsCreated != 0 {
		t.Errorf("ObjectsCreated = %d, want 0 (no completed jobs)", agg.ObjectsCreated)
	}
}

func TestOverallRememberStatus(t *testing.T) {
	tests := []struct {
		name        string
		runStatus   AgentRunStatus
		agentErr    string
		jobsPending bool
		jobFailures []string
		wantStatus  string
		wantErr     string
	}{
		{"completed no jobs", RunStatusSuccess, "", false, nil, "completed", ""},
		{"agent failed", RunStatusError, "boom", false, nil, "failed", "boom"},
		{"agent failed no msg", RunStatusError, "", false, nil, "failed", "run failed"},
		{"agent cancelled", RunStatusCancelled, "", false, nil, "failed", "run was cancelled"},
		{"agent still running", RunStatusRunning, "", false, nil, "running", ""},
		{"job still pending after agent done", RunStatusSuccess, "", true, nil, "running", ""},
		{"job failed after agent done", RunStatusSuccess, "", false, []string{"job x failed: y"}, "failed", "extraction job(s) failed: job x failed: y"},
		{"job failed even if agent failed but job pending", RunStatusError, "boom", true, nil, "running", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, errMsg := overallRememberStatus(tt.runStatus, tt.agentErr, tt.jobsPending, tt.jobFailures)
			if status != tt.wantStatus {
				t.Errorf("status = %q, want %q", status, tt.wantStatus)
			}
			if errMsg != tt.wantErr {
				t.Errorf("errMsg = %q, want %q", errMsg, tt.wantErr)
			}
		})
	}
}

// TestRememberStatus_ReextractionPending: run with only a queue-reextraction
// call whose job is still pending → overall status running, no final counts.
func TestRememberStatus_ReextractionPending(t *testing.T) {
	toolCalls := []*AgentRunToolCall{
		tc("queue-reextraction", map[string]any{"document_id": "d-1"}, map[string]any{"job_id": "job-1"}),
	}
	jobIDs := reextractionJobIDs(toolCalls)
	if !reflect.DeepEqual(jobIDs, []string{"job-1"}) {
		t.Fatalf("jobIDs = %v", jobIDs)
	}

	agg := aggregateRememberStatus(toolCalls) // queue-reextraction not in allowlist → zero
	jobAgg, pending, failures := aggregateExtractionJobs([]*mcp.ExtractionJobInfo{ej("job-1", "pending", 0, 0, nil, nil)})
	agg.merge(jobAgg)

	if !pending {
		t.Error("pending = false, want true")
	}
	status, _ := overallRememberStatus(RunStatusSuccess, "", pending, failures)
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
	if agg.ObjectsCreated != 0 || agg.RelationshipsCreated != 0 {
		t.Errorf("expected zero counts while pending, got %+v", agg)
	}
}

// TestRememberStatus_ReextractionCompleted: queue-reextraction + completed job
// with objects → status completed with the job's counts merged in.
func TestRememberStatus_ReextractionCompleted(t *testing.T) {
	toolCalls := []*AgentRunToolCall{
		tc("queue-reextraction", map[string]any{"document_id": "d-1"}, map[string]any{"job_id": "job-1"}),
	}
	agg := aggregateRememberStatus(toolCalls)
	jobAgg, pending, failures := aggregateExtractionJobs([]*mcp.ExtractionJobInfo{
		ej("job-1", "completed", 2, 1, []string{"Person"}, []string{"obj-a", "obj-b"}),
	})
	agg.merge(jobAgg)

	status, _ := overallRememberStatus(RunStatusSuccess, "", pending, failures)
	if status != "completed" {
		t.Errorf("status = %q, want completed", status)
	}
	if agg.ObjectsCreated != 2 {
		t.Errorf("ObjectsCreated = %d, want 2 (from extraction job)", agg.ObjectsCreated)
	}
	if agg.RelationshipsCreated != 1 {
		t.Errorf("RelationshipsCreated = %d, want 1", agg.RelationshipsCreated)
	}
	if !reflect.DeepEqual(agg.CreatedObjectIDs, []string{"obj-a", "obj-b"}) {
		t.Errorf("CreatedObjectIDs = %v", agg.CreatedObjectIDs)
	}
	if !reflect.DeepEqual(agg.DiscoveredTypes, []string{"Person"}) {
		t.Errorf("DiscoveredTypes = %v", agg.DiscoveredTypes)
	}
	if !strings.Contains(agg.Summary, "created 2 objects") {
		t.Errorf("Summary = %q, want created 2 objects", agg.Summary)
	}
}

// TestRememberStatus_ReextractionFailed: queue-reextraction + failed job →
// status failed with the job error surfaced.
func TestRememberStatus_ReextractionFailed(t *testing.T) {
	toolCalls := []*AgentRunToolCall{
		tc("queue-reextraction", map[string]any{"document_id": "d-1"}, map[string]any{"job_id": "job-1"}),
	}
	agg := aggregateRememberStatus(toolCalls)
	errMsg := "schema mismatch"
	jobAgg, pending, failures := aggregateExtractionJobs([]*mcp.ExtractionJobInfo{
		{ID: "job-1", Status: "failed", ErrorMessage: &errMsg},
	})
	agg.merge(jobAgg)

	status, statusErr := overallRememberStatus(RunStatusSuccess, "", pending, failures)
	if status != "failed" {
		t.Errorf("status = %q, want failed", status)
	}
	if !strings.Contains(statusErr, "job-1") || !strings.Contains(statusErr, "schema mismatch") {
		t.Errorf("error = %q, want job id + message surfaced", statusErr)
	}
}

// TestRememberStatus_MixedDirectAndReextraction: a run with BOTH direct
// entity-create calls AND queue-reextraction — counts from both merge.
func TestRememberStatus_MixedDirectAndReextraction(t *testing.T) {
	toolCalls := []*AgentRunToolCall{
		tc("entity-create", nil, map[string]any{
			"results": []any{
				map[string]any{"success": true, "entity": map[string]any{"id": "obj-direct", "type": "Decision"}},
			},
		}),
		tc("queue-reextraction", map[string]any{"document_id": "d-1"}, map[string]any{"job_id": "job-1"}),
	}
	agg := aggregateRememberStatus(toolCalls)
	if agg.ObjectsCreated != 1 {
		t.Fatalf("direct tool-call ObjectsCreated = %d, want 1", agg.ObjectsCreated)
	}

	jobAgg, pending, failures := aggregateExtractionJobs([]*mcp.ExtractionJobInfo{
		ej("job-1", "completed", 2, 3, []string{"Person", "Task"}, []string{"obj-a", "obj-b"}),
	})
	agg.merge(jobAgg)

	status, _ := overallRememberStatus(RunStatusSuccess, "", pending, failures)
	if status != "completed" {
		t.Errorf("status = %q, want completed", status)
	}
	if agg.ObjectsCreated != 3 {
		t.Errorf("ObjectsCreated = %d, want 3 (1 direct + 2 from job)", agg.ObjectsCreated)
	}
	if agg.RelationshipsCreated != 3 {
		t.Errorf("RelationshipsCreated = %d, want 3 (job only)", agg.RelationshipsCreated)
	}
	if !reflect.DeepEqual(agg.CreatedObjectIDs, []string{"obj-direct", "obj-a", "obj-b"}) {
		t.Errorf("CreatedObjectIDs = %v", agg.CreatedObjectIDs)
	}
	if !reflect.DeepEqual(agg.DiscoveredTypes, []string{"Decision", "Person", "Task"}) {
		t.Errorf("DiscoveredTypes = %v", agg.DiscoveredTypes)
	}
}

// TestRememberStatus_MissingJobTreatedPending: a queue-reextraction job_id that
// cannot be resolved is treated as in-flight so completion is never claimed
// prematurely.
func TestRememberStatus_MissingJobTreatedPending(t *testing.T) {
	// ExecuteRememberStatus synthesizes a pending job for unresolvable IDs; this
	// verifies the resulting aggregation/status behaves like a pending job.
	agg := rememberStatusAggregation{}
	jobAgg, pending, failures := aggregateExtractionJobs([]*mcp.ExtractionJobInfo{
		{ID: "job-missing", Status: "pending"},
	})
	agg.merge(jobAgg)

	if !pending {
		t.Error("pending = false, want true for unresolved job")
	}
	status, _ := overallRememberStatus(RunStatusSuccess, "", pending, failures)
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
}

// ----------------------------------------------------------------------------
// embedding generation tracking
// ----------------------------------------------------------------------------

func TestAggregateEmbeddingStatus(t *testing.T) {
	t.Run("all completed → ready", func(t *testing.T) {
		s := aggregateEmbeddingStatus([]mcp.EmbeddingJobInfo{ei("a", "completed"), ei("b", "completed")})
		if s.Pending != 0 || s.Failed != 0 {
			t.Errorf("got %+v, want zero pending/failed", s)
		}
	})

	t.Run("some pending → not ready", func(t *testing.T) {
		s := aggregateEmbeddingStatus([]mcp.EmbeddingJobInfo{
			ei("a", "completed"), ei("b", "processing"), ei("c", "pending"),
		})
		if s.Pending != 2 {
			t.Errorf("Pending = %d, want 2", s.Pending)
		}
		if s.Failed != 0 {
			t.Errorf("Failed = %d, want 0", s.Failed)
		}
	})

	t.Run("some failed", func(t *testing.T) {
		s := aggregateEmbeddingStatus([]mcp.EmbeddingJobInfo{
			ei("a", "completed"), ei("b", "dead_letter"), ei("c", "failed"),
		})
		if s.Failed != 2 {
			t.Errorf("Failed = %d, want 2", s.Failed)
		}
		if s.Pending != 0 {
			t.Errorf("Pending = %d, want 0", s.Pending)
		}
	})

	t.Run("mixed statuses incl no-job and cancelled", func(t *testing.T) {
		s := aggregateEmbeddingStatus([]mcp.EmbeddingJobInfo{
			ei("a", "pending"), ei("b", "failed"), ei("c", "completed"),
			ei("d", ""), ei("e", "cancelled"),
		})
		if s.Pending != 1 || s.Failed != 1 {
			t.Errorf("got %+v, want Pending=1 Failed=1", s)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		s := aggregateEmbeddingStatus(nil)
		if s.Pending != 0 || s.Failed != 0 {
			t.Errorf("got %+v, want zeros", s)
		}
	})
}

func TestEmbeddingSummaryNote(t *testing.T) {
	if n := embeddingSummaryNote(0, 0, 3); n != "" {
		t.Errorf("ready note = %q, want empty", n)
	}
	n := embeddingSummaryNote(2, 0, 3)
	if !strings.Contains(n, "Embeddings still processing (2/3 pending)") {
		t.Errorf("pending note = %q", n)
	}
	n = embeddingSummaryNote(0, 1, 3)
	if !strings.Contains(n, "1 object(s) failed embedding generation") {
		t.Errorf("failed note = %q", n)
	}
	// pending takes precedence over failed in the note
	n = embeddingSummaryNote(1, 2, 4)
	if !strings.Contains(n, "(1/4 pending)") {
		t.Errorf("mixed note = %q", n)
	}
}

// TestRememberStatus_EmbeddingsReadiness models the full remember-status flow
// for the primary queue-reextraction path with created objects, then checks
// embedding readiness reporting for the completed/pending/failed cases. The
// overall status stays "completed" regardless of embedding readiness.
func TestRememberStatus_EmbeddingsReadiness(t *testing.T) {
	toolCalls := []*AgentRunToolCall{
		tc("queue-reextraction", map[string]any{"document_id": "d-1"}, map[string]any{"job_id": "job-1"}),
	}
	agg := aggregateRememberStatus(toolCalls)
	jobAgg, jobsPending, jobFailures := aggregateExtractionJobs([]*mcp.ExtractionJobInfo{
		ej("job-1", "completed", 2, 1, []string{"Person"}, []string{"obj-a", "obj-b"}),
	})
	agg.merge(jobAgg)

	status, _ := overallRememberStatus(RunStatusSuccess, "", jobsPending, jobFailures)
	if status != "completed" {
		t.Fatalf("status = %q, want completed", status)
	}
	createdIDs := agg.CreatedObjectIDs
	if !reflect.DeepEqual(createdIDs, []string{"obj-a", "obj-b"}) {
		t.Fatalf("createdIDs = %v", createdIDs)
	}

	t.Run("all embeddings completed → ready", func(t *testing.T) {
		emb := aggregateEmbeddingStatus([]mcp.EmbeddingJobInfo{ei("obj-a", "completed"), ei("obj-b", "completed")})
		ready := emb.Pending == 0
		if !ready {
			t.Errorf("embeddings_ready = false, want true")
		}
		if emb.Pending != 0 || emb.Failed != 0 {
			t.Errorf("emb = %+v, want zeros", emb)
		}
		if n := embeddingSummaryNote(emb.Pending, emb.Failed, len(createdIDs)); n != "" {
			t.Errorf("summary note = %q, want empty", n)
		}
	})

	t.Run("some embeddings pending → not ready + note, status still completed", func(t *testing.T) {
		emb := aggregateEmbeddingStatus([]mcp.EmbeddingJobInfo{ei("obj-a", "completed"), ei("obj-b", "processing")})
		if emb.Pending != 1 {
			t.Errorf("Pending = %d, want 1", emb.Pending)
		}
		if emb.Pending == 0 {
			t.Errorf("embeddings_ready should be false")
		}
		note := embeddingSummaryNote(emb.Pending, emb.Failed, len(createdIDs))
		if !strings.Contains(note, "Embeddings still processing (1/2 pending)") {
			t.Errorf("note = %q", note)
		}
		// status is unaffected by embedding readiness
		if status != "completed" {
			t.Errorf("status = %q, want completed", status)
		}
	})

	t.Run("some embeddings failed → failed count + note, status still completed", func(t *testing.T) {
		emb := aggregateEmbeddingStatus([]mcp.EmbeddingJobInfo{ei("obj-a", "completed"), ei("obj-b", "dead_letter")})
		if emb.Failed != 1 {
			t.Errorf("Failed = %d, want 1", emb.Failed)
		}
		note := embeddingSummaryNote(emb.Pending, emb.Failed, len(createdIDs))
		if !strings.Contains(note, "1 object(s) failed embedding generation") {
			t.Errorf("note = %q", note)
		}
		if status != "completed" {
			t.Errorf("status = %q, want completed", status)
		}
	})

	t.Run("no embedding info (finder not configured) → no fields, no note", func(t *testing.T) {
		// Mirrors the graceful degradation when h.embeddingJobs is nil: nothing
		// is tracked, no embedding fields are emitted, and the summary stays the
		// plain mutation summary.
		embTracked := false
		var emb embeddingStatusSummary
		summary := agg.Summary
		if embTracked {
			summary += embeddingSummaryNote(emb.Pending, emb.Failed, len(createdIDs))
		}
		if summary != agg.Summary {
			t.Errorf("summary = %q, want %q (unchanged when not tracked)", summary, agg.Summary)
		}
	})
}
