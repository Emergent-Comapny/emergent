package blueprints

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

// Blueprint status values.
const (
	StatusDraft      = "draft"
	StatusPublished  = "published"
	StatusDeprecated = "deprecated"
)

// Blueprint is a versioned, immutable-on-publish blueprint definition.
// Manifest is stored as opaque JSON; its contents are parsed and validated
// in a later apply phase. Checksum is a content hash of the JSON manifest
// used for future apply/drift comparison, not verified on read.
type Blueprint struct {
	bun.BaseModel `bun:"table:kb.blueprints,alias:bp"`

	ID          string          `bun:"id,pk,type:uuid,default:uuid_generate_v4()" json:"id"`
	Name        string          `bun:"name,notnull" json:"name"`
	Version     string          `bun:"version,notnull" json:"version"`
	Description string          `bun:"description,notnull,default:''" json:"description"`
	Author      string          `bun:"author,notnull,default:''" json:"author"`
	Status      string          `bun:"status,notnull,default:'draft'" json:"status"`
	Manifest    json.RawMessage `bun:"manifest,type:jsonb" json:"manifest"`
	Checksum    string          `bun:"checksum,notnull,default:''" json:"checksum"`
	CreatedAt   time.Time       `bun:"created_at,notnull" json:"created_at"`
	UpdatedAt   time.Time       `bun:"updated_at,notnull" json:"updated_at"`
}

// CreateBlueprintRequest is the request to create a new blueprint draft.
type CreateBlueprintRequest struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description,omitempty"`
	Author      string          `json:"author,omitempty"`
	Manifest    json.RawMessage `json:"manifest"`
}

// UpdateBlueprintRequest is the request to update a draft blueprint.
// Only non-nil fields are applied; Manifest applies when non-nil.
type UpdateBlueprintRequest struct {
	Description *string         `json:"description,omitempty"`
	Author      *string         `json:"author,omitempty"`
	Manifest    json.RawMessage `json:"manifest"`
}

// NewVersionRequest is the request to clone a blueprint into a new version.
type NewVersionRequest struct {
	Version string `json:"version"`
}

// BlueprintApplication records one project's application of a blueprint version.
// (project_id, blueprint_id) is unique — a re-apply updates the row — so the
// table reflects "currently applied" state, not append-only history. Checksum
// is the applied manifest content hash (empty for a draft apply), kept for
// future drift detection and unapply.
type BlueprintApplication struct {
	bun.BaseModel `bun:"table:kb.blueprint_applications,alias:bpa"`

	ID          string    `bun:"id,pk,type:uuid,default:uuid_generate_v4()" json:"id"`
	BlueprintID string    `bun:"blueprint_id,notnull,type:uuid" json:"blueprintId"`
	ProjectID   string    `bun:"project_id,notnull,type:uuid" json:"projectId"`
	Version     string    `bun:"version,notnull" json:"version"`
	Checksum    string    `bun:"checksum,notnull,default:''" json:"checksum"`
	AppliedBy   *string   `bun:"applied_by,type:uuid" json:"appliedBy,omitempty"`
	Status      string    `bun:"status,notnull,default:'applied'" json:"status"`
	AppliedAt   time.Time `bun:"applied_at,notnull,default:now()" json:"appliedAt"`
	CreatedAt   time.Time `bun:"created_at,notnull,default:now()" json:"createdAt"`
	UpdatedAt   time.Time `bun:"updated_at,notnull,default:now()" json:"updatedAt"`
}

// AppliedBlueprint is the project-scoped view of an applied blueprint, joined
// with its blueprint metadata. Returned by GET /api/blueprints/applied.
type AppliedBlueprint struct {
	BlueprintID string    `json:"blueprintId"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Description string    `json:"description,omitempty"`
	Author      string    `json:"author,omitempty"`
	Checksum    string    `json:"checksum"`
	AppliedAt   time.Time `json:"appliedAt"`
}

// UnapplyCounts reports per-entity-type reversal counts for one blueprint
// unapply. Missing counts entities that were already absent (idempotent);
// Skipped counts entities intentionally not reversed.
type UnapplyCounts struct {
	Removed int `json:"removed"`
	Missing int `json:"missing"`
	Skipped int `json:"skipped"`
}

// UnapplyResult is the response from Unapply.
type UnapplyResult struct {
	BlueprintID      string        `json:"blueprintId"`
	BlueprintName    string        `json:"blueprintName"`
	Version          string        `json:"version"`
	Checksum         string        `json:"checksum"`         // stored (applied) checksum
	Drift            bool          `json:"drift"`            // applied checksum != current blueprint checksum
	AlreadyUnapplied bool          `json:"alreadyUnapplied"` // idempotent short-circuit
	DriftedAgents    []string      `json:"driftedAgents,omitempty"`
	Packs            UnapplyCounts `json:"packs"`
	Agents           UnapplyCounts `json:"agents"`
	Skills           UnapplyCounts `json:"skills"`
	SkippedSkills    []string      `json:"skippedSkills,omitempty"`
	Seed             UnapplyCounts `json:"seed"`
	Status           string        `json:"status"`
}
