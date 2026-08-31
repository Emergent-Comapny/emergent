package blueprints

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/emergent-company/emergent.memory/domain/agents"
	"github.com/emergent-company/emergent.memory/domain/graph"
	"github.com/emergent-company/emergent.memory/domain/schemas"
	"github.com/emergent-company/emergent.memory/domain/skills"
	"github.com/emergent-company/emergent.memory/pkg/apperror"
	"github.com/emergent-company/emergent.memory/pkg/logger"
)

// Service handles business logic for blueprints
type Service struct {
	repo        *Repository
	schemasSvc  *schemas.Service
	schemasRepo *schemas.Repository
	skillsRepo  *skills.Repository
	graphSvc    *graph.Service
	agentRepo   *agents.Repository // optional; set via SetAgentRepo
	log         *slog.Logger
}

// NewService creates a new blueprints service
func NewService(repo *Repository, schemasSvc *schemas.Service, schemasRepo *schemas.Repository, skillsRepo *skills.Repository, graphSvc *graph.Service, log *slog.Logger) *Service {
	return &Service{
		repo:        repo,
		schemasSvc:  schemasSvc,
		schemasRepo: schemasRepo,
		skillsRepo:  skillsRepo,
		graphSvc:    graphSvc,
		log:         log.With(logger.Scope("blueprints.svc")),
	}
}

// SetAgentRepo injects the agents repository (optional dependency, wired only
// when the agents feature is enabled). When unset, the agents step of Apply is
// skipped with a warning.
func (s *Service) SetAgentRepo(ar *agents.Repository) {
	s.agentRepo = ar
}

// CreateBlueprint validates the request and creates a new blueprint draft.
func (s *Service) CreateBlueprint(ctx context.Context, req *CreateBlueprintRequest) (*Blueprint, error) {
	if req.Name == "" {
		return nil, apperror.ErrBadRequest.WithMessage("name is required")
	}
	if req.Version == "" {
		return nil, apperror.ErrBadRequest.WithMessage("version is required")
	}

	exists, err := s.repo.ExistsByNameVersion(ctx, req.Name, req.Version)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.ErrConflict.WithMessage("blueprint with name and version already exists")
	}

	now := time.Now()
	bp := &Blueprint{
		Name:        req.Name,
		Version:     req.Version,
		Description: req.Description,
		Author:      req.Author,
		Status:      StatusDraft,
		Manifest:    req.Manifest,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if bp.Manifest == nil {
		bp.Manifest = json.RawMessage("{}")
	}

	if err := s.repo.Create(ctx, bp); err != nil {
		return nil, err
	}
	return bp, nil
}

// GetBlueprint returns a blueprint by ID.
func (s *Service) GetBlueprint(ctx context.Context, id string) (*Blueprint, error) {
	return s.repo.GetByID(ctx, id)
}

// ListBlueprints returns blueprints, optionally filtered by name.
func (s *Service) ListBlueprints(ctx context.Context, nameFilter string) ([]Blueprint, error) {
	return s.repo.List(ctx, nameFilter)
}

// ListVersions returns all versions of a blueprint name.
func (s *Service) ListVersions(ctx context.Context, name string) ([]Blueprint, error) {
	return s.repo.ListVersionsByName(ctx, name)
}

// UpdateBlueprint updates a draft blueprint's description, author, and manifest.
func (s *Service) UpdateBlueprint(ctx context.Context, id string, req *UpdateBlueprintRequest) (*Blueprint, error) {
	bp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if bp.Status != StatusDraft {
		return nil, apperror.ErrConflict.WithMessage("only draft blueprints can be updated")
	}

	if req.Description != nil {
		bp.Description = *req.Description
	}
	if req.Author != nil {
		bp.Author = *req.Author
	}
	if req.Manifest != nil {
		bp.Manifest = req.Manifest
	}

	if err := s.repo.Update(ctx, bp); err != nil {
		return nil, err
	}
	return bp, nil
}

// PublishBlueprint transitions a draft to published, computing a sha256
// checksum over the raw JSON manifest bytes. The checksum is a content hash
// of the manifest for future apply/drift comparison; it is not verified on
// read (tamper detection is out of scope for this phase).
func (s *Service) PublishBlueprint(ctx context.Context, id string) (*Blueprint, error) {
	bp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if bp.Status != StatusDraft {
		return nil, apperror.ErrConflict.WithMessage("only draft blueprints can be published")
	}

	manifestBytes := bp.Manifest
	if len(manifestBytes) == 0 {
		manifestBytes = json.RawMessage("{}")
	}
	sum := sha256.Sum256(manifestBytes)
	checksum := hex.EncodeToString(sum[:])

	if err := s.repo.UpdateStatus(ctx, id, StatusPublished, checksum); err != nil {
		return nil, err
	}
	bp.Status = StatusPublished
	bp.Checksum = checksum
	return bp, nil
}

// DeprecateBlueprint transitions any blueprint to deprecated.
func (s *Service) DeprecateBlueprint(ctx context.Context, id string) (*Blueprint, error) {
	bp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if bp.Status == StatusDeprecated {
		return bp, nil
	}

	if err := s.repo.UpdateStatus(ctx, id, StatusDeprecated, bp.Checksum); err != nil {
		return nil, err
	}
	bp.Status = StatusDeprecated
	return bp, nil
}

// NewVersion clones a blueprint into a new row with the given version, status draft.
func (s *Service) NewVersion(ctx context.Context, id, newVersion string) (*Blueprint, error) {
	if newVersion == "" {
		return nil, apperror.ErrBadRequest.WithMessage("version is required")
	}

	source, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	exists, err := s.repo.ExistsByNameVersion(ctx, source.Name, newVersion)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperror.ErrConflict.WithMessage("blueprint with name and version already exists")
	}

	now := time.Now()
	clone := &Blueprint{
		Name:        source.Name,
		Version:     newVersion,
		Description: source.Description,
		Author:      source.Author,
		Status:      StatusDraft,
		Manifest:    source.Manifest,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if clone.Manifest == nil {
		clone.Manifest = json.RawMessage("{}")
	}

	if err := s.repo.Create(ctx, clone); err != nil {
		return nil, err
	}
	return clone, nil
}

// DeleteBlueprint deletes a blueprint, draft only.
func (s *Service) DeleteBlueprint(ctx context.Context, id string) error {
	bp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if bp.Status != StatusDraft {
		return apperror.ErrConflict.WithMessage("only draft blueprints can be deleted")
	}
	return s.repo.Delete(ctx, id)
}

// ListApplied returns the blueprints applied to a project.
func (s *Service) ListApplied(ctx context.Context, projectID string) ([]AppliedBlueprint, error) {
	return s.repo.ListApplied(ctx, projectID)
}

// recordApplication upserts the project's application record for a blueprint.
// It is the provenance write behind future drift detection and unapply; called
// only after a fully successful apply (packs/agents/skills/seed).
func (s *Service) recordApplication(ctx context.Context, bp *Blueprint, projectID, userID string) error {
	var appliedBy *string
	if userID != "" {
		appliedBy = &userID
	}
	if err := s.repo.RecordApplication(ctx, &BlueprintApplication{
		BlueprintID: bp.ID,
		ProjectID:   projectID,
		Version:     bp.Version,
		Checksum:    bp.Checksum,
		AppliedBy:   appliedBy,
		Status:      "applied",
		AppliedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}); err != nil {
		return err
	}
	// An upgrade applies a new version of the same name; supersede the previous
	// application so ListApplied shows only the latest.
	return s.repo.SupersedeApplications(ctx, projectID, bp.ID, bp.Name)
}

// Unapply reverses an apply for a project: it hard-deletes the blueprint's
// agents (by name, project-scoped) and soft-removes the pack assignments
// (leaving the global pack), then marks the application record unapplied.
// Skills are skipped (global scope, shared across projects) and seed graph
// objects are skipped (no per-object provenance). NOT atomic — each step is
// idempotent (find-then-delete-by-id; already-absent counts as missing), so a
// retry after a mid-unapply crash converges. The record is marked unapplied
// last so GET /applied still lists it until the unapply completes.
func (s *Service) Unapply(ctx context.Context, blueprintID, projectID string) (*UnapplyResult, error) {
	bp, err := s.repo.GetByID(ctx, blueprintID)
	if err != nil {
		return nil, err
	}

	app, err := s.repo.GetApplication(ctx, projectID, blueprintID)
	if err != nil {
		return nil, err
	}

	result := &UnapplyResult{
		BlueprintID:   bp.ID,
		BlueprintName: bp.Name,
		Version:       bp.Version,
		Checksum:      app.Checksum,
		Status:        "unapplied",
	}

	if app.Status == "unapplied" {
		result.AlreadyUnapplied = true
		return result, nil
	}

	// Drift: applied checksum differs from the current blueprint checksum.
	result.Drift = app.Checksum != bp.Checksum

	var manifest BlueprintManifest
	if err := json.Unmarshal(bp.Manifest, &manifest); err != nil {
		return nil, apperror.ErrBadRequest.WithMessage("blueprint manifest is not valid JSON: " + err.Error())
	}

	// Agents: delete only definitions this blueprint created (source_blueprint_id
	// == this blueprint). Pre-existing/manual (nil) and other-blueprint-owned
	// definitions are skipped. Flag drift on owned definitions edited after apply.
	if s.agentRepo != nil {
		for _, am := range manifest.Agents {
			def, err := s.agentRepo.FindDefinitionByName(ctx, projectID, am.Name)
			if err != nil {
				return nil, err
			}
			if def == nil {
				result.Agents.Missing++
				continue
			}
			if def.SourceBlueprintID == nil || *def.SourceBlueprintID != blueprintID {
				result.Agents.Skipped++
				continue
			}
			if def.UpdatedAt.After(app.AppliedAt) {
				result.DriftedAgents = append(result.DriftedAgents, am.Name)
			}
			if err := s.agentRepo.DeleteDefinition(ctx, def.ID); err != nil {
				return nil, err
			}
			result.Agents.Removed++
		}
	} else if len(manifest.Agents) > 0 {
		result.Agents.Skipped = len(manifest.Agents)
	}

	// Packs: remove only assignments this blueprint owns and that no other
	// applied blueprint still claims. Shared and manual assignments are skipped.
	if len(manifest.Packs) > 0 {
		for _, p := range manifest.Packs {
			pack, err := s.schemasRepo.GetPackByNameVersion(ctx, p.Name, p.Version)
			if err != nil {
				if isNotFound(err) {
					result.Packs.Missing++
					continue
				}
				return nil, err
			}
			// Release this blueprint's claim first (idempotent).
			if err := s.repo.DeletePackClaim(ctx, projectID, blueprintID, pack.ID); err != nil {
				return nil, err
			}
			assignment, err := s.schemasRepo.GetActiveAssignment(ctx, projectID, pack.ID)
			if err != nil {
				return nil, err
			}
			if assignment == nil {
				result.Packs.Missing++
				continue
			}
			removed, err := s.schemasRepo.DeleteAssignmentIfOwned(ctx, projectID, pack.ID, blueprintID)
			if err != nil {
				return nil, err
			}
			if removed {
				result.Packs.Removed++
			} else {
				result.Packs.Skipped++
			}
		}
	}

	// Skills: skipped (global scope, shared across projects/blueprints).
	for _, sk := range manifest.Skills {
		result.Skills.Skipped++
		result.SkippedSkills = append(result.SkippedSkills, sk.Name)
	}

	// Seed: skipped (no per-object provenance to attribute).
	if manifest.Seed != nil {
		result.Seed.Skipped = len(manifest.Seed.Objects) + len(manifest.Seed.Relationships)
	}

	// Mark the application record unapplied last (retry converges). The
	// applied_at guard detects a concurrent re-apply and returns conflict.
	if err := s.repo.MarkUnapplied(ctx, projectID, blueprintID, app.AppliedAt); err != nil {
		return nil, err
	}

	return result, nil
}
