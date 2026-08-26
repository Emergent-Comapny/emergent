# 40 — MCP Graph Response Contract

Status: proposed (spec-first, pre-implementation)
Scope: MCP tool responses for graph operations only (`apps/server/domain/mcp`).

## 1. Problem

MCP graph tools return the same DTOs the REST API serves browsers. Those DTOs
carry ~28 fields per object for internal/telemetry/provenance concerns. An LLM
agent reading a tool result needs ~6. Verbose responses burn tokens and dilute
signal.

Concrete offenders, measured against current code:

| Tool | Response type | Fields per item |
|---|---|---|
| `search-hybrid` / `search-semantic` / `search-similar` | `graph.SearchResponse` → `SearchResultItem.object = GraphObjectResponse` | 28 + 4 score fields |
| `graph-traverse` | `graph.TraverseGraphResponse` | nodes w/ full `properties` + `paths[][]`, ~15 top-level meta fields |
| `relationship-list` / `relationship-update` / `relationship-delete` | `graph.GraphRelationshipResponse` | 17 |
| `entity-history` | `ObjectHistoryResponse` → full `GraphObjectResponse` | 28 |

Two structural leaks make it worse:

1. **Dual-ID emission.** `GraphObjectResponse.MarshalJSON` and
   `GraphRelationshipResponse.MarshalJSON` emit both legacy (`id`,
   `canonical_id`) and new (`version_id`, `entity_id`) names — 4 UUID strings
   for 2 concepts. Every object, every relationship.
2. **Indented JSON.** `Service.wrapResult` uses `json.MarshalIndent(…, "", "  ")`.
   Indentation adds ~2 whitespace chars per line for zero signal.

Already-good paths (do not regress): `entity-query` (has `fields` projection,
`include_relationships`, `filters`), `entity-edges-get` (slim `EdgeInfo`),
batch `entity-create` (slimEntity).

## 2. Principles

1. **Default = slim.** Read tools return only agent-essential fields.
2. **One opt-in, one knob per concern.** Not per-tool ad hoc flags.
3. **Single ID per object.** `id` = canonical id (entity id). Version id is
   opt-in via `verbose`.
4. **REST API unchanged.** Slimming happens at the MCP mapping layer, not in
   shared DTOs.
5. **Compact JSON for MCP text blocks.**

## 3. Canonical slim shapes

### 3.1 Entity (canonical id = `id`)

```jsonc
{
  "id":        "<canonical_id>",   // string, UUID
  "type":      "Person",
  "key":       "alice",            // omitempty
  "name":      "Alice",            // derived from properties.name, omitempty
  "properties": { /* full, or projected by fields[] */ },
  "labels":    ["core"]            // omitempty
}
```

### 3.2 Relationship

```jsonc
{
  "id":         "<canonical_id>",
  "type":       "works_at",
  "src_id":     "<canonical_id>",
  "dst_id":     "<canonical_id>",
  "label":      "…",               // omitempty
  "weight":     1.0,               // omitempty, default-visible
  "properties": {}                 // omitempty
}
```

### 3.3 Search result item

```jsonc
{
  "object": { /* slim entity §3.1 */ },
  "score": 0.87                     // float, ranking only
}
```

### 3.4 Traverse node / edge

```jsonc
// node
{ "id": "<canonical_id>", "type": "Person", "key": "alice", "depth": 1,
  "labels": ["core"], "properties": {} /* by field_strategy */ }

// edge
{ "id": "<canonical_id>", "type": "works_at", "src_id": "…", "dst_id": "…" }
```

## 4. Opt-in parameters

Uniform across all graph read tools. Unknown params already warn in
`entity-query`; replicate that behavior.

### 4.1 `fields: []string`
Property projection from the `properties` blob. Semantics identical to
existing `entity-query.fields`:
- Only the named property keys are returned.
- `name` is always prepended (for display) unless already listed.
- `id`, `type`, `key`, `labels`, `created_at` are **always** returned and must
  **not** be listed in `fields`.
- Empty / omitted = all properties.

Apply to: `search-hybrid`, `search-semantic`, `graph-traverse`,
`relationship-list`, `entity-history`.

### 4.2 `verbose: bool` (default `false`)
Opt-in for non-typical info. `false` → nothing extra. `true` → the following,
per response kind:

| Kind | `verbose=true` adds |
|---|---|
| Entity | `version`, `version_id`, `status`, `namespace`, `branch_id`, `branch_name`, `schema_version`, `created_at`, `updated_at`, `deleted_at`, `change_summary`, `content_hash`, `external_source`, `external_id`, `external_url`, `external_parent_id`, `synced_at`, `external_updated_at`, `revision_count`, `relationship_count`, `supersedes_id` |
| Relationship | `version`, `version_id`, `branch_id`, `created_at`, `deleted_at`, `change_summary`, `inverse_relationship` |
| Search | per-item `lexical_score`, `vector_score`, `vector_dist`; top-level `meta { elapsed_ms, channel_stats }` |
| Traverse | `max_depth_reached`, `total_nodes`, `has_next_page`, `has_previous_page`, `next_cursor`, `previous_cursor`, `approx_position_start`, `approx_position_end`, `page_direction`, `query_time_ms`, `result_count`, per-node `paths` |
| relationship-list | `pagination` cursor/offset detail beyond `has_more` |

`weight` is default-visible (omitempty), not verbose. First-class edge
semantic (strength), symmetric with `label`, and free when absent — most packs
don't set it, so it costs nothing in practice while keeping `verbose` reserved
for provenance/meta.

### 4.3 `field_strategy: "full" | "compact" | "minimal"`
Controls node/object property depth in graph-shaped responses. The field is
already declared on `graph.TraverseGraphRequest` (`dto.go:696`) but **never
consumed** by the graph service. Implement the selection at the MCP mapping
layer (cheapest; no graph-service change):

| Strategy | Entity properties returned |
|---|---|
| `minimal` | none (id, type, key, depth, labels only) |
| `compact` (default) | none; `name` derived and included |
| `full` | entire `properties` blob |

`field_strategy` and `fields` compose: `fields` projects the blob, then
`field_strategy` decides whether the (projected) blob is emitted at all.
`minimal`/`compact` ignore `fields`.

`field_strategy` governs object/edge **properties only**. `score` is always
present in `search-*` results regardless of strategy — it is the ranking
signal, not a property, and dropping it would make search unusable.

Apply to: `graph-traverse` (primary), `search-hybrid`, `search-semantic`
(default `compact` — agents get id/type/key/name/score, drop full properties
unless asked).

## 5. Dual-ID collapse (MCP layer only)

MCP mapping emits a **single** `id` (canonical id). Legacy/new alias pairs
(`id`/`canonical_id`, `version_id`/`entity_id`) are dropped from MCP output.
`version_id` available only under `verbose`.

Rationale: agents key on identity; two UUIDs per object wastes ~72 bytes/object.
Backward-compat risk is low — MCP consumers are agents, not the UI/REST clients.
Keep `canonical_id` in the input surface where tools accept a version vs
canonical id ambiguity (e.g. `entity-history`), but collapse in outputs.

## 6. Serialization

`wrapResult` gains a compact path (or `wrapResultCompact`) using
`json.Marshal` (no indent) for list-heavy tools. Indentation retained only for
the REST/`tools/list` schema surfaces if any. Target: `search-*`,
`graph-traverse`, `relationship-list`, `entity-query`, `entity-search`,
`entity-history`.

## 7. Tool-by-tool contract

| Tool | Default response | New params | Notes |
|---|---|---|---|
| `entity-query` | unchanged (§3.1 `Entity`) | `fields`†, `verbose` | already slim |
| `entity-search` | unchanged | `fields`, `verbose` | |
| `entity-edges-get` | unchanged | `verbose` | already slim |
| `entity-history` | `versions[]` slim entity | `fields`, `verbose` | collapse dual IDs |
| `entity-create` (batch) | unchanged slimEntity | — | |
| `entity-create` (single) | slim (§3.1) + `relationships[]` | `verbose` | drop `status`/`labels`/`version`/`created_at` from default |
| `entity-update` | slim (§3.1) | `verbose` | |
| `entity-delete` / `entity-restore` | `{success, entity_id}` | — | |
| `search-hybrid` | `{data: [§3.3], total, has_more}` | `fields`, `verbose`, `field_strategy` | |
| `search-semantic` | same as hybrid | same | |
| `search-similar` | `{similar_entities: [slim+score], total}` | `verbose`, `field_strategy` | |
| `graph-traverse` | `{nodes:[§3.4], edges:[§3.4], truncated}` | `field_strategy`, `fields`, `verbose` | |
| `relationship-list` | `{data:[§3.2], total, has_more}` | `fields`, `verbose` | |
| `relationship-create` | `{success, relationship: §3.2}` | `verbose` | |
| `relationship-update` | `{success, relationship: §3.2}` | `verbose` | |
| `relationship-delete` | `{success, relationship_id}` | — | drop full echo |
| `tag-list` | `{tags, total}` | — | unchanged |
| `graph-branch-*` | unchanged | — | |
| `graph-branch-merge` | summary counts only | `verbose` (objects[]/relationships[]) | see §8 |

† `fields` already exists on `entity-query`; nothing to add there.

## 8. Branch merge

`graph-branch-merge` `BranchMergeResponse` returns `objects[]` and
`relationships[]` with full per-object summaries. Default: keep aggregate counts
(`total_objects`, `added_count`, `conflict_count`, `resolved_count`,
`unchanged_count`, `deleted_count`, `relationships_*`). Move `objects[]` and
`relationships[]` behind `verbose`, and cap the verbose lists to the first
`limit` (default 100) with `truncated` flag.

## 9. Backward compatibility

- New params are additive (`fields`, `verbose`, `field_strategy`). Default
  behavior changes response shape (slimmer) — a breaking change for any agent
  relying on `project_id`/`org_id`/timestamps in MCP output. Accept as intended;
  document in tool `description` strings.
- `entity-create`/`entity-update`/`relationship-*` default responses shrink.
  Keep `success` + `id`-bearing echo so agents can chain operations.
- REST API (`/api/...`) untouched throughout.

## 10. Non-goals

- No change to REST DTOs or `graph` domain structs.
- No change to `tools/list` schemas beyond adding the new param properties.
- No removal of fields from the underlying DB or storage.
- No implementation of `FieldStrategy` inside the graph service (unless a
  later perf pass justifies it).

## 11. Resolved decisions

1. **`score` always present** in `search-*`, independent of `field_strategy`
   (see §4.3). Never omitted.
2. **`weight` default-visible** (omitempty), not verbose (see §4.2).
3. **Per-call `verbose` only.** No global default via project/org tool config
   for now. Revisit only if telemetry shows agents repeatedly passing
   `verbose=true`; the three-tier config resolver plumbing already exists, so
   the later change is cheap (YAGNI).

## 12. Acceptance criteria (for implementation phase)

- [ ] `search-hybrid` with no new params returns slim items, no dual IDs, no
      score breakdown.
- [ ] `graph-traverse` default returns nodes+edges+`truncated` only.
- [ ] `verbose=true` reproduces today's full field set (no signal loss).
- [ ] `fields` projection works on all listed read tools.
- [ ] `field_strategy=minimal|compact|full` respected on `graph-traverse` + search.
- [ ] `wrapResult` emits compact JSON for list tools.
- [ ] Unit tests assert default-slim shapes and opt-in expansion.
- [ ] `go build ./...`, `task lint`, e2e smoke pass.
