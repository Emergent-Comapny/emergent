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
