package blueprints

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestObjectTypeDefRoundTripPreservesBehaviouralFields verifies that object
// type behavioural keys (labels, embedding, extraction, ui) and relationship
// type properties survive a manifest JSON round-trip. This is the losslessness
// guarantee behind routing schema-carrying packs through blueprint apply.
func TestObjectTypeDefRoundTripPreservesBehaviouralFields(t *testing.T) {
	m := BlueprintManifest{
		Packs: []PackManifest{{
			Name:    "agent-notes",
			Version: "2.0.0",
			ObjectTypes: []ObjectTypeDef{{
				Name:        "Note",
				Label:       "Note",
				Description: "an observation",
				Properties: map[string]any{
					"content": map[string]any{"type": "string", "required": true},
				},
				Labels:     []string{"note", "{category}"},
				Embedding:  map[string]any{"mode": "field", "field": "content"},
				Extraction: map[string]any{"enabled": false},
				UI:         map[string]any{"icon": "📝", "color": "#4F46E5"},
			}},
			RelationshipTypes: []RelationshipTypeDef{{
				Name:       "annotates",
				Label:      "Annotates",
				SourceType: "Note",
				TargetType: "*",
				Properties: map[string]any{"description": "links a note to its target"},
			}},
		}},
	}

	raw, err := json.Marshal(m)
	require.NoError(t, err)

	var back BlueprintManifest
	require.NoError(t, json.Unmarshal(raw, &back))

	ot := back.Packs[0].ObjectTypes[0]
	assert.Equal(t, []string{"note", "{category}"}, ot.Labels, "labels must round-trip")
	assert.Equal(t, "field", ot.Embedding["mode"], "embedding.mode must round-trip")
	assert.Equal(t, "content", ot.Embedding["field"], "embedding.field must round-trip")
	assert.Equal(t, false, ot.Extraction["enabled"], "extraction.enabled:false must survive (not dropped by omitempty)")
	assert.Equal(t, "📝", ot.UI["icon"], "ui.icon must round-trip")
	assert.Equal(t, true, ot.Properties["content"].(map[string]any)["required"], "properties must round-trip")

	rt := back.Packs[0].RelationshipTypes[0]
	assert.Equal(t, "links a note to its target", rt.Properties["description"], "relationship properties must round-trip")
}
