package modelconfig

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/emergent-company/emergent.memory/domain/provider"
	"github.com/emergent-company/emergent.memory/pkg/adk"
	"github.com/emergent-company/emergent.memory/pkg/auth"
	"github.com/emergent-company/emergent.memory/pkg/embeddings"
)

// ADKModelResolverAdapter adapts modelconfig.Service to adk.ModelResolver.
// This breaks the import cycle: pkg/adk cannot import domain/modelconfig,
// so domain/modelconfig provides this adapter and registers it via fx.
type ADKModelResolverAdapter struct {
	svc *Service
}

// NewADKModelResolverAdapter creates a new adapter.
func NewADKModelResolverAdapter(svc *Service) adk.ModelResolver {
	return &ADKModelResolverAdapter{svc: svc}
}

// ResolveGenerativeModelByID implements adk.ModelResolver.
func (a *ADKModelResolverAdapter) ResolveGenerativeModelByID(ctx context.Context, projectIDStr string) (string, string, error) {
	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return "", "", fmt.Errorf("modelconfig adapter: invalid project id %q: %w", projectIDStr, err)
	}
	model, source, err := a.svc.ResolveGenerativeModel(ctx, projectID)
	if err != nil {
		return "", "", err
	}
	return model, string(source), nil
}

// EmbeddingResolverAdapter adapts modelconfig.Service + provider.CredentialService to
// embeddings.EmbeddingResolver. It resolves the embedding model via modelconfig
// (project config only, no guessing) and fetches credentials for the provider
// indicated by the model's provider prefix (e.g. "google/gemini-embedding-001").
//
// This replaces the old provider.EmbeddingCredentialAdapter which used ResolveAny
// and could return the wrong provider (e.g. DeepSeek) for embedding calls.
type EmbeddingResolverAdapter struct {
	svc     *Service
	credsvc *provider.CredentialService
}

// NewEmbeddingResolverAdapter creates the adapter.
func NewEmbeddingResolverAdapter(svc *Service, credsvc *provider.CredentialService) embeddings.EmbeddingResolver {
	return &EmbeddingResolverAdapter{svc: svc, credsvc: credsvc}
}

// ResolveEmbedding implements embeddings.EmbeddingResolver.
// It reads the embedding model from project_model_config, parses the provider
// prefix, fetches credentials for that specific provider, and returns a
// ResolvedEmbeddingCredential. Returns an error if no embedding model is
// configured — no silent fallback.
func (a *EmbeddingResolverAdapter) ResolveEmbedding(ctx context.Context) (*embeddings.ResolvedEmbeddingCredential, error) {
	projectIDStr := auth.ProjectIDFromContext(ctx)
	if projectIDStr == "" {
		return nil, nil // no project context — caller will use static client
	}

	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		return nil, fmt.Errorf("embedding resolver: invalid project id %q: %w", projectIDStr, err)
	}

	model, _, err := a.svc.ResolveEmbeddingModel(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("embedding resolver: failed to resolve embedding model: %w", err)
	}

	// Fast path: model set via project_model_config (projects set-models).
	if model != "" {
		parts := strings.SplitN(model, "/", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("embedding resolver: invalid model name %q — must be 'provider/model-name'", model)
		}
		cred, err := a.credsvc.ResolveFor(ctx, parts[0])
		if err != nil {
			return nil, fmt.Errorf("embedding resolver: failed to get credentials for provider %q: %w", parts[0], err)
		}
		if cred == nil {
			return nil, fmt.Errorf("no credentials configured for provider %q — run 'memory provider configure-project %s --api-key ...'", parts[0], parts[0])
		}
		return a.buildEmbeddingCredential(cred, parts[1]), nil
	}

	// Fallback: no project_model_config — use the provider credential's
	// embedding model (set via 'memory provider configure-project <provider>
	// --embedding-model <model>'). ResolveAnyEmbedding skips providers without
	// an embedding model, so it never short-circuits on a generative-only
	// provider (e.g. DeepSeek).
	if cred, err := a.credsvc.ResolveAnyEmbedding(ctx); err != nil {
		return nil, fmt.Errorf("embedding resolver: failed to resolve embedding credential: %w", err)
	} else if cred != nil && cred.EmbeddingModel != "" {
		emb := cred.EmbeddingModel
		if _, bare, ok := strings.Cut(emb, "/"); ok {
			emb = bare
		}
		return a.buildEmbeddingCredential(cred, emb), nil
	}

	return nil, fmt.Errorf("no embedding model configured for project %s — run 'memory projects set-models --embedding provider/model-name' or 'memory provider configure-project <provider> --embedding-model <model>'", projectIDStr)
}

// buildEmbeddingCredential maps a resolved provider credential to the
// embeddings.ResolvedEmbeddingCredential shape, using the given bare model name.
func (a *EmbeddingResolverAdapter) buildEmbeddingCredential(cred *provider.ResolvedCredential, bareModel string) *embeddings.ResolvedEmbeddingCredential {
	return &embeddings.ResolvedEmbeddingCredential{
		IsGoogleAI:         cred.Provider == provider.ProviderGoogleAI,
		APIKey:             cred.APIKey,
		IsVertexAI:         cred.Provider == provider.ProviderVertexAI,
		GCPProject:         cred.GCPProject,
		Location:           cred.Location,
		ServiceAccountJSON: cred.ServiceAccountJSON,
		EmbeddingModel:     bareModel,
		BaseURL:            cred.BaseURL,
		Source:             string(cred.Source),
	}
}
