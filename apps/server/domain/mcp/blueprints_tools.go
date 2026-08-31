package mcp

import (
	"context"
)

// BlueprintToolHandler is the interface for executing blueprint-related MCP
// tools. Implemented by the blueprints domain to avoid circular imports
// (blueprints → mcp). Injected into the mcp Service via SetBlueprintToolHandler.
type BlueprintToolHandler interface {
	ExecuteBlueprintCreate(ctx context.Context, projectID string, args map[string]any) (*ToolResult, error)
	ExecuteBlueprintList(ctx context.Context, projectID string, args map[string]any) (*ToolResult, error)
	ExecuteBlueprintGet(ctx context.Context, projectID string, args map[string]any) (*ToolResult, error)
	ExecuteBlueprintPublish(ctx context.Context, projectID string, args map[string]any) (*ToolResult, error)
	ExecuteBlueprintApply(ctx context.Context, projectID string, args map[string]any) (*ToolResult, error)
	ExecuteBlueprintUnapply(ctx context.Context, projectID string, args map[string]any) (*ToolResult, error)
	ExecuteBlueprintVersions(ctx context.Context, projectID string, args map[string]any) (*ToolResult, error)
	ExecuteBlueprintListApplied(ctx context.Context, projectID string, args map[string]any) (*ToolResult, error)
	ExecuteBlueprintNewVersion(ctx context.Context, projectID string, args map[string]any) (*ToolResult, error)
	ExecuteBlueprintUpdate(ctx context.Context, projectID string, args map[string]any) (*ToolResult, error)
}

// ============================================================================
// Blueprint Tool Definitions
// ============================================================================

func blueprintsToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:          "blueprint-create",
			RequiredScope: "schema:write",
			Description:   "Create a new blueprint draft. Returns the created blueprint with its id, name, version, and manifest. Publish it with blueprint-publish before applying it to a project.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"name": {
						Type:        "string",
						Description: "Blueprint name (e.g. 'team-onboarding')",
					},
					"version": {
						Type:        "string",
						Description: "Blueprint version (e.g. '1.0.0')",
					},
					"description": {
						Type:        "string",
						Description: "Human-readable description of the blueprint",
					},
					"author": {
						Type:        "string",
						Description: "Author of the blueprint",
					},
					"manifest": {
						Type:        "object",
						Description: "Blueprint manifest as a JSON object or a JSON-encoded string. Describes schema packs, agents, skills, and seed graph objects/relationships to materialize on apply.",
					},
				},
				Required: []string{"name", "version"},
			},
		},
		{
			Name:          "blueprint-list",
			RequiredScope: "schema:read",
			Description:   "List blueprints, optionally filtered by name. Returns id, name, version, status, and checksum for each blueprint.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"name": {
						Type:        "string",
						Description: "Optional filter — only return blueprints with this name",
					},
				},
				Required: []string{},
			},
		},
		{
			Name:          "blueprint-get",
			RequiredScope: "schema:read",
			Description:   "Get a single blueprint by id. Returns the full blueprint including its manifest.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"id": {
						Type:        "string",
						Description: "UUID of the blueprint to retrieve",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:          "blueprint-publish",
			RequiredScope: "schema:write",
			Description:   "Publish a draft blueprint, computing a sha256 checksum over its manifest. Only published (or draft) blueprints can be applied. Returns the published blueprint.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"id": {
						Type:        "string",
						Description: "UUID of the draft blueprint to publish",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:          "blueprint-apply",
			RequiredScope: "schema:write",
			Description:   "Apply a blueprint to the current project, materializing its manifest: schema packs (create-or-skip, always assigned), agent definitions (create-or-update by name), global skills (create-or-update by name), and seed graph objects/relationships. Idempotent — re-applying converges. Returns per-type created/updated/skipped counts.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"id": {
						Type:        "string",
						Description: "UUID of the blueprint to apply",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:          "blueprint-unapply",
			RequiredScope: "schema:write",
			Description:   "Reverse a previous blueprint apply for the current project: removes agent definitions owned by the blueprint and pack assignments owned by it, leaving the global pack. Skills and seed graph objects are skipped (shared/global scope). Idempotent. Returns per-type removal counts.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"id": {
						Type:        "string",
						Description: "UUID of the blueprint to unapply",
					},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:          "blueprint-versions",
			RequiredScope: "schema:read",
			Description:   "List all versions of a blueprint name, newest first. Returns the full blueprint for each version.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"name": {
						Type:        "string",
						Description: "Blueprint name to list versions for",
					},
				},
				Required: []string{"name"},
			},
		},
		{
			Name:          "blueprint-list-applied",
			RequiredScope: "schema:read",
			Description:   "List the blueprints currently applied to the current project. Returns blueprint id, name, version, and applied-at timestamp for each.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
				Required:   []string{},
			},
		},
		{
			Name:          "blueprint-new-version",
			RequiredScope: "schema:write",
			Description:   "Clone an existing blueprint into a new version as a draft. The new version copies the source manifest; publish it (blueprint-publish) before it can be applied.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"id": {
						Type:        "string",
						Description: "UUID of the blueprint to clone from",
					},
					"version": {
						Type:        "string",
						Description: "New semantic version, e.g. '1.1.0'",
					},
				},
				Required: []string{"id", "version"},
			},
		},
		{
			Name:          "blueprint-update",
			RequiredScope: "schema:write",
			Description:   "Update a draft blueprint's description, author, and/or manifest. Only drafts can be updated — published blueprints are immutable; use blueprint-new-version to change a published blueprint.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"id": {
						Type:        "string",
						Description: "UUID of the draft blueprint to update",
					},
					"description": {
						Type:        "string",
						Description: "New description (optional)",
					},
					"author": {
						Type:        "string",
						Description: "New author (optional)",
					},
					"manifest": {
						Type:        "object",
						Description: "New manifest as a JSON object or a JSON-encoded string (optional). Describes schema packs, agents, skills, and seed graph objects/relationships to materialize on apply.",
					},
				},
				Required: []string{"id"},
			},
		},
	}
}
