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
