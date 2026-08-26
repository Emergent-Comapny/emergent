## Context

Memory already owns durable skill storage (`kb.skills`), the runtime injection path (`agent-skill-catalog` spec), and the write/import surfaces (REST, MCP `skill-*` tools, CLI `skills import`, blueprint `skills/` dirs). This change specs the **management** half and closes four gaps. It deliberately stops short of any autonomous learning loop or external marketplace sync (see Non-Goals).

## Decisions

### 1. Provenance lives in `metadata` (jsonb), not new columns

`SkillMetadata` currently holds only `Location`. We extend it with provenance fields instead of adding columns:

```
metadata: {
  location: string?,        // existing
  source: manual|cli|blueprint|agent|marketplace,
  license: string?,          // SPDX id or free text
  version: string?,
  source_url: string?,       // upstream origin
  origin_id: string?,        // upstream identifier (e.g. "hermes/official/security/1password")
  content_hash: string        // SHA-256 of content, server-computed
}
```

Rationale: additive, no migration beyond behavior, flexible for future marketplace fields. `content_hash` is always server-computed (callers may never set it) and recomputed whenever `content` changes — this is the idempotency anchor for future re-import/drift detection.

### 2. Scope model — project > org > global

Three scopes, encoded by which ID field is set. Name uniqueness is global and per-project; a project skill shadows a broader-scoped skill of the same name (no error). This matches existing `FindForAgent` merge behavior and the `agent-skill-catalog` precedence rule.

### 3. Embedding on every write path

The bug: `skill-create`/`skill-update` (MCP) pass `embedding=nil`, so MCP-created skills never surface in semantic retrieval above the 50-skill threshold. Fix: the write path (store `Create`/`Update`) triggers description embedding generation uniformly, with the existing non-fatal behavior preserved.

### 4. MCP write tools are project-scoped

An agent can create/update/delete **project** skills only. Global/org skill management is an operator concern (REST + CLI + blueprints). This bounds agent blast radius.

### 5. AutoLoadSkills semantics

`auto_load_skills = true` triggers prefix matching `{agentName}.{suffix}`. Merge order: explicit `skills` first, then auto-matched, deduplicated by name. This is the hook a future "self-improving agent" change will use: an agent writes `{agentName}.{topic}` and it auto-loads next run — no definition edit required.

## Non-Goals

- **Background learning loop** (nudge counter, background review agent, curator/consolidation, usage telemetry, confidence). Future change builds on this storage/import foundation.
- **Marketplace sync** (network fetch from skills.sh / hermes skills-index / cline catalog / `.well-known/skills/index.json`). Provenance fields here are the enabling prerequisite; the actual fetch/registry integration is a separate future change.
- **Version history / rollback** of skill content — out of scope for v1 management.

## Open Questions

- Content size cap default: proposed 1 MiB (aligns with hermes per-file cap). Confirm against current handler behavior.
- Whether `skill-list` should eventually support server-side keyword filtering; today it returns slim summaries and semantic retrieval is runtime-only.
