package agents

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for the per-agent default tool policy (ToolPolicyDefault), the
// effectiveToolPolicy resolution, argument redaction, and hasToolPolicy.

func TestEffectiveToolPolicy_ExplicitOverridesDefault(t *testing.T) {
	d := &AgentDefinition{
		DefaultToolPolicy: ToolPolicyDefaultDeny,
		ToolPolicies: map[string]ToolPolicy{
			"safe-tool": {Confirm: true},
		},
	}
	p, ok := d.effectiveToolPolicy("safe-tool")
	assert.True(t, ok, "explicit entry must be honored over the default")
	assert.True(t, p.Confirm)
	assert.False(t, p.Disabled)
}

func TestEffectiveToolPolicy_DefaultDeny(t *testing.T) {
	d := &AgentDefinition{DefaultToolPolicy: ToolPolicyDefaultDeny}
	p, ok := d.effectiveToolPolicy("unlisted")
	assert.True(t, ok)
	assert.True(t, p.Disabled, "default deny must hard-block unlisted tools")
}

func TestEffectiveToolPolicy_DefaultAsk(t *testing.T) {
	d := &AgentDefinition{DefaultToolPolicy: ToolPolicyDefaultAsk}
	p, ok := d.effectiveToolPolicy("unlisted")
	assert.True(t, ok)
	assert.True(t, p.Confirm, "default ask must require confirmation for unlisted tools")
}

func TestEffectiveToolPolicy_DefaultAllow(t *testing.T) {
	d := &AgentDefinition{DefaultToolPolicy: ToolPolicyDefaultAllow}
	_, ok := d.effectiveToolPolicy("unlisted")
	assert.False(t, ok, "default allow must apply no policy")
}

func TestEffectiveToolPolicy_ZeroValueBehavesAsAllow(t *testing.T) {
	d := &AgentDefinition{}
	_, ok := d.effectiveToolPolicy("unlisted")
	assert.False(t, ok, "zero-value default must behave as allow")
}

func TestDefaultToolPolicy_JSONRoundTrip(t *testing.T) {
	d := &AgentDefinition{
		Name:              "x",
		DefaultToolPolicy: ToolPolicyDefaultAsk,
		ToolPolicies: map[string]ToolPolicy{
			"note": {Confirm: true, Message: "ok?"},
		},
	}
	data, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"defaultToolPolicy":"ask"`)

	var back AgentDefinition
	require.NoError(t, json.Unmarshal(data, &back))
	assert.Equal(t, ToolPolicyDefaultAsk, back.DefaultToolPolicy)
}

func TestDefaultToolPolicy_MissingFieldBehavesAsAllow(t *testing.T) {
	// Old rows without the field must not break: they resolve to allow.
	raw := `{"name":"x","toolPolicies":{"note":{"confirm":true}}}`
	var d AgentDefinition
	require.NoError(t, json.Unmarshal([]byte(raw), &d))
	_, ok := d.effectiveToolPolicy("unlisted")
	assert.False(t, ok, "absent defaultToolPolicy must behave as allow")
}

func TestRedactToolArgs_RedactsValues(t *testing.T) {
	out := redactToolArgs(map[string]any{
		"title":  "secret title",
		"count":  42,
		"flag":   true,
		"nested": map[string]any{"token": "abc"},
		"list":   []any{"a"},
		"nil":    nil,
	})
	assert.Equal(t, "string", out["title"])
	assert.Equal(t, "number", out["count"])
	assert.Equal(t, "boolean", out["flag"])
	assert.Equal(t, "object", out["nested"])
	assert.Equal(t, "array", out["list"])
	assert.Equal(t, "null", out["nil"])
}

func TestRedactToolArgs_Empty(t *testing.T) {
	assert.Empty(t, redactToolArgs(nil))
	assert.Empty(t, redactToolArgs(map[string]any{}))
}

func TestHasToolPolicy(t *testing.T) {
	assert.True(t, hasToolPolicy(&AgentDefinition{ToolPolicies: map[string]ToolPolicy{"x": {}}}))
	assert.True(t, hasToolPolicy(&AgentDefinition{DefaultToolPolicy: ToolPolicyDefaultDeny}))
	assert.True(t, hasToolPolicy(&AgentDefinition{DefaultToolPolicy: ToolPolicyDefaultAsk}))
	assert.False(t, hasToolPolicy(&AgentDefinition{DefaultToolPolicy: ToolPolicyDefaultAllow}))
	assert.False(t, hasToolPolicy(&AgentDefinition{}))
}
