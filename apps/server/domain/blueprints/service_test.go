package blueprints

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/emergent-company/emergent.memory/pkg/apperror"
)

// newServiceFromBun builds a Service with a real blueprints Repository and nil
// cross-domain deps. CRUD/versioning/status paths only use s.repo, and the
// Apply paths under test (deprecated / invalid manifest) return before any
// cross-domain dep is touched, so nil is safe here.
func newServiceFromBun(t *testing.T, db *bun.DB) *Service {
	t.Helper()
	repo := NewRepository(db, testLogger())
	return NewService(repo, nil, nil, nil, nil, testLogger())
}

// TestService_CreateBlueprint_Success verifies a created blueprint is a draft
// with the manifest round-tripping through the jsonb column.
func TestService_CreateBlueprint_Success(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	bp, err := svc.CreateBlueprint(ctx, &CreateBlueprintRequest{
		Name:        uniqueName("svc-create"),
		Version:     "1.0.0",
		Description: "a test blueprint",
		Author:      "tester",
		Manifest:    json.RawMessage(`{"kind":"service"}`),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, bp.ID)
	assert.Equal(t, StatusDraft, bp.Status)
	assert.Equal(t, "", bp.Checksum)

	got, err := svc.repo.GetByID(ctx, bp.ID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind":"service"}`, string(got.Manifest), "manifest must round-trip")
	assert.Equal(t, "a test blueprint", got.Description)
}

// TestService_CreateBlueprint_Conflict verifies duplicate (name, version) is
// rejected with a conflict error.
func TestService_CreateBlueprint_Conflict(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	req := &CreateBlueprintRequest{Name: uniqueName("svc-dup"), Version: "1.0.0", Manifest: json.RawMessage(`{}`)}
	_, err := svc.CreateBlueprint(ctx, req)
	require.NoError(t, err)

	_, err = svc.CreateBlueprint(ctx, req)
	assertAppErrorCode(t, err, apperror.ErrConflict.Code)
}

// TestService_CreateBlueprint_Validation verifies empty name/version are
// rejected with bad_request.
func TestService_CreateBlueprint_Validation(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	_, err := svc.CreateBlueprint(ctx, &CreateBlueprintRequest{Version: "1.0.0", Manifest: json.RawMessage(`{}`)})
	assertAppErrorCode(t, err, apperror.ErrBadRequest.Code)

	_, err = svc.CreateBlueprint(ctx, &CreateBlueprintRequest{Name: uniqueName("no-version"), Manifest: json.RawMessage(`{}`)})
	assertAppErrorCode(t, err, apperror.ErrBadRequest.Code)
}

// TestService_UpdateBlueprint verifies draft updates apply and non-draft
// updates are rejected.
func TestService_UpdateBlueprint(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	bp, err := svc.CreateBlueprint(ctx, &CreateBlueprintRequest{
		Name: uniqueName("svc-update"), Version: "1.0.0", Manifest: json.RawMessage(`{"v":1}`),
	})
	require.NoError(t, err)

	t.Run("draft update applies", func(t *testing.T) {
		desc := "updated"
		author := "author2"
		updated, err := svc.UpdateBlueprint(ctx, bp.ID, &UpdateBlueprintRequest{
			Description: &desc,
			Author:      &author,
			Manifest:    json.RawMessage(`{"v":2}`),
		})
		require.NoError(t, err)
		assert.Equal(t, "updated", updated.Description)
		assert.Equal(t, "author2", updated.Author)
		assert.JSONEq(t, `{"v":2}`, string(updated.Manifest))
		assert.Equal(t, StatusDraft, updated.Status, "status must remain draft")
	})

	t.Run("published blueprint cannot be updated", func(t *testing.T) {
		_, err := svc.PublishBlueprint(ctx, bp.ID)
		require.NoError(t, err)

		desc := "nope"
		_, err = svc.UpdateBlueprint(ctx, bp.ID, &UpdateBlueprintRequest{Description: &desc})
		assertAppErrorCode(t, err, apperror.ErrConflict.Code)
	})
}

// TestService_PublishBlueprint_ComputesSha256Checksum verifies draft→published
// with a non-empty sha256 checksum of the manifest.
func TestService_PublishBlueprint_ComputesSha256Checksum(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	bp, err := svc.CreateBlueprint(ctx, &CreateBlueprintRequest{
		Name: uniqueName("svc-publish"), Version: "1.0.0",
		Manifest: json.RawMessage(`{"kind":"publish-me"}`),
	})
	require.NoError(t, err)

	// Checksum must be sha256 of the ROUND-TRIPPED manifest (jsonb normalizes
	// whitespace, so compute against what the DB actually returns).
	stored, err := svc.repo.GetByID(ctx, bp.ID)
	require.NoError(t, err)

	pub, err := svc.PublishBlueprint(ctx, bp.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusPublished, pub.Status)
	require.NotEmpty(t, pub.Checksum)

	sum := sha256.Sum256(stored.Manifest)
	assert.Equal(t, hex.EncodeToString(sum[:]), pub.Checksum)
}

// TestService_PublishBlueprint_AlreadyPublished_Conflict verifies publishing a
// published blueprint is rejected.
func TestService_PublishBlueprint_AlreadyPublished_Conflict(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	bp, err := svc.CreateBlueprint(ctx, &CreateBlueprintRequest{
		Name: uniqueName("svc-republish"), Version: "1.0.0", Manifest: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	_, err = svc.PublishBlueprint(ctx, bp.ID)
	require.NoError(t, err)

	_, err = svc.PublishBlueprint(ctx, bp.ID)
	assertAppErrorCode(t, err, apperror.ErrConflict.Code)
}

// TestService_NewVersion verifies cloning into a new draft version, and that
// an existing (name, version) is rejected.
func TestService_NewVersion(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	bp, err := svc.CreateBlueprint(ctx, &CreateBlueprintRequest{
		Name:        uniqueName("svc-version"),
		Version:     "1.0.0",
		Description: "orig desc",
		Author:      "orig author",
		Manifest:    json.RawMessage(`{"kind":"clone-me"}`),
	})
	require.NoError(t, err)

	clone, err := svc.NewVersion(ctx, bp.ID, "2.0.0")
	require.NoError(t, err)
	assert.NotEqual(t, bp.ID, clone.ID, "clone must be a NEW row")
	assert.Equal(t, bp.Name, clone.Name)
	assert.Equal(t, "2.0.0", clone.Version)
	assert.Equal(t, StatusDraft, clone.Status)
	assert.Equal(t, "orig desc", clone.Description)
	assert.Equal(t, "orig author", clone.Author)
	assert.JSONEq(t, `{"kind":"clone-me"}`, string(clone.Manifest), "manifest must be copied")

	versions, err := svc.ListVersions(ctx, bp.Name)
	require.NoError(t, err)
	require.Len(t, versions, 2)

	t.Run("existing version conflicts", func(t *testing.T) {
		_, err := svc.NewVersion(ctx, bp.ID, "1.0.0")
		assertAppErrorCode(t, err, apperror.ErrConflict.Code)
	})
}

// TestService_DeprecateBlueprint_IsIdempotent verifies any-status→deprecated
// and that deprecating an already-deprecated blueprint is a no-op success.
func TestService_DeprecateBlueprint_IsIdempotent(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	bp, err := svc.CreateBlueprint(ctx, &CreateBlueprintRequest{
		Name: uniqueName("svc-deprecate"), Version: "1.0.0", Manifest: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	// draft → deprecated directly.
	dep, err := svc.DeprecateBlueprint(ctx, bp.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusDeprecated, dep.Status)

	// deprecated → deprecated is idempotent (no error, same row).
	again, err := svc.DeprecateBlueprint(ctx, bp.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusDeprecated, again.Status)
	assert.Equal(t, dep.ID, again.ID)
}

// TestService_DeleteBlueprint_DraftOnly verifies draft deletion succeeds and
// published deletion is rejected.
func TestService_DeleteBlueprint_DraftOnly(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	req := &CreateBlueprintRequest{Name: uniqueName("svc-delete"), Version: "1.0.0", Manifest: json.RawMessage(`{}`)}

	t.Run("draft deletes", func(t *testing.T) {
		bp, err := svc.CreateBlueprint(ctx, req)
		require.NoError(t, err)
		require.NoError(t, svc.DeleteBlueprint(ctx, bp.ID))
		_, err = svc.repo.GetByID(ctx, bp.ID)
		assertAppErrorCode(t, err, apperror.ErrNotFound.Code)
	})

	t.Run("published cannot be deleted", func(t *testing.T) {
		bp, err := svc.CreateBlueprint(ctx, req)
		require.NoError(t, err)
		_, err = svc.PublishBlueprint(ctx, bp.ID)
		require.NoError(t, err)
		err = svc.DeleteBlueprint(ctx, bp.ID)
		assertAppErrorCode(t, err, apperror.ErrConflict.Code)
	})
}

// TestService_Apply_Deprecated_Conflict verifies Apply rejects deprecated
// blueprints before touching any cross-domain dep.
func TestService_Apply_Deprecated_Conflict(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	bp, err := svc.CreateBlueprint(ctx, &CreateBlueprintRequest{
		Name: uniqueName("svc-apply-dep"), Version: "1.0.0", Manifest: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	_, err = svc.DeprecateBlueprint(ctx, bp.ID)
	require.NoError(t, err)

	_, err = svc.Apply(ctx, bp.ID, uuid.NewString(), uuid.NewString(), ApplyOptions{})
	assertAppErrorCode(t, err, apperror.ErrConflict.Code)
}

// TestService_Apply_InvalidManifest_BadRequest verifies Apply rejects a
// manifest that is valid JSONB but not a BlueprintManifest, before touching
// any cross-domain dep.
func TestService_Apply_InvalidManifest_BadRequest(t *testing.T) {
	db := connectTestDB(t)
	svc := newServiceFromBun(t, db)
	ctx := context.Background()

	// `[]` is valid JSON (storable in the jsonb column) but cannot unmarshal
	// into BlueprintManifest.
	bp, err := svc.CreateBlueprint(ctx, &CreateBlueprintRequest{
		Name: uniqueName("svc-apply-invalid"), Version: "1.0.0", Manifest: json.RawMessage(`[]`),
	})
	require.NoError(t, err)

	_, err = svc.Apply(ctx, bp.ID, uuid.NewString(), uuid.NewString(), ApplyOptions{})
	assertAppErrorCode(t, err, apperror.ErrBadRequest.Code)
}
