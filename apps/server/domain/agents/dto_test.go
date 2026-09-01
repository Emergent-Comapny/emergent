package agents

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAgentDefinitionToSummaryDTOIncludesSkills ensures the lightweight list DTO
// carries the bound skill names so clients can compute per-skill usage without
// fetching each definition in full.
func TestAgentDefinitionToSummaryDTOIncludesSkills(t *testing.T) {
	d := &AgentDefinition{
		ID:        "ad-1",
		ProjectID: "proj-1",
		Name:      "diane",
		Tools:     []string{"memory", "skill"},
		Skills:    []string{"diane.meetings", "research"},
	}

	s := d.ToSummaryDTO()

	require.Equal(t, "ad-1", s.ID)
	require.Equal(t, 2, s.ToolCount)
	require.Equal(t, []string{"diane.meetings", "research"}, s.Skills)
}

// TestAgentToDTOIncludesAgentDefinitionID ensures the response DTO exposes the
// linked agent definition so clients can read back a scheduled agent's
// definition after create/update. The field was previously absent from
// AgentDTO, so the gateway could not preselect the saved definition.
func TestAgentToDTOIncludesAgentDefinitionID(t *testing.T) {
	defID := "ad-123"
	a := &Agent{
		ID:                "agent-1",
		Name:              "Test",
		AgentDefinitionID: &defID,
	}

	dto := a.ToDTO()

	require.NotNil(t, dto.AgentDefinitionID)
	require.Equal(t, "ad-123", *dto.AgentDefinitionID)
}
