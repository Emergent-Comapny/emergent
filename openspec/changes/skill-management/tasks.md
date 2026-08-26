## 1. Provenance metadata

- [ ] 1.1 Extend `SkillMetadata` in `apps/server/domain/skills/entity.go` with `Source`, `License`, `Version`, `SourceURL`, `OriginID`, `ContentHash` fields (jsonb, additive, all omitempty)
- [ ] 1.2 In `store.go`, compute `content_hash` (SHA-256 of `content`) on Create and Update; never trust caller-provided hash
- [ ] 1.3 Recompute `content_hash` whenever `content` changes; preserve all other provenance fields verbatim on update
- [ ] 1.4 Unit test: create records provenance + hash; content update recomputes hash; provenance untouched on non-content update

## 2. Embedding on every write path

- [ ] 2.1 Move description-embedding generation into the store write path (Create/Update) so REST, MCP, CLI, and blueprint all share it
- [ ] 2.2 Verify `skill-create`/`skill-update` (MCP) now produce a non-null `description_embedding`; keep embedding failure non-fatal
- [ ] 2.3 Unit test: MCP create populates embedding; embedding service error → skill still created with null embedding

## 3. Validation

- [ ] 3.1 Enforce name pattern `^[a-z0-9]+(-[a-z0-9]+)*$` and 1–64 length at a single validation point shared by REST + MCP
- [ ] 3.2 Enforce non-empty `description` and `content` on create
- [ ] 3.3 Add content size cap (configurable, default 1 MiB)
- [ ] 3.4 Unit tests: invalid name, empty description, empty content, oversized content all rejected

## 4. AutoLoadSkills spec conformance

- [ ] 4.1 Verify `buildSkillsSystemPrompt` prefix matching (`{agentName}.{suffix}`) matches spec; add tests for prefix match, explicit-first dedup, and off-state
- [ ] 4.2 Confirm `auto_load_skills` is not silently ignored when the agent also has explicit `skills`

## 5. CLI / blueprint import provenance

- [ ] 5.1 `memory skills import --builtin` / `--discover`: set `source = cli`, and `license`/`version` from SKILL.md frontmatter when present
- [ ] 5.2 Blueprint `skills/` apply: set `source = blueprint` and populate provenance from the skill file metadata
- [ ] 5.3 Unit/integration test: import is idempotent (re-run matches by name, no duplicates)

## 6. Tests & verification

- [ ] 6.1 Add REST handler tests covering merged project list, superadmin-gated global create, partial PATCH
- [ ] 6.2 Add MCP tool tests: project-scoped create, name resolution in `skill-get`, write-scope rejection
- [ ] 6.3 `go build ./...` clean; run `task lint` or equivalent
- [ ] 6.4 Run existing skills + agent-definition test suites; no regressions
