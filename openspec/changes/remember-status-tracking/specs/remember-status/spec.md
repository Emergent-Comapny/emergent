## Purpose

Gives callers of async `remember`/`forget` operations a way to check whether the background run has finished and what graph objects/relationships it created, updated, or failed to create, without needing to poll unrelated run-execution status alone.

## ADDED Requirements

### Requirement: remember-status tool reports run completion state
The system SHALL expose a `remember-status` MCP tool (and equivalent REST route) that, given a `run_id`, returns the current lifecycle status of that run: `running`, `completed`, or `failed`.

#### Scenario: Run still in progress
- **WHEN** a caller invokes `remember-status` with a `run_id` for a run that has not finished
- **THEN** the response SHALL include `"status": "running"`
- **THEN** the response SHALL NOT claim any objects or relationships as final (counts MAY be reported as partial/in-progress or omitted)

#### Scenario: Run completed successfully
- **WHEN** a caller invokes `remember-status` with a `run_id` for a run that finished without error
- **THEN** the response SHALL include `"status": "completed"`

#### Scenario: Run failed
- **WHEN** a caller invokes `remember-status` with a `run_id` for a run that terminated with an error
- **THEN** the response SHALL include `"status": "failed"`
- **THEN** the response SHALL include an `error` field with a human-readable failure reason

#### Scenario: Unknown run_id
- **WHEN** a caller invokes `remember-status` with a `run_id` that does not exist or does not belong to the caller's project
- **THEN** the system SHALL return an error result indicating the run was not found
- **THEN** the system SHALL NOT leak information about runs belonging to other projects

### Requirement: remember-status reports graph mutation summary
When a run has produced at least one graph-mutating tool call, `remember-status` SHALL report a summary of what was created, updated, or attempted, derived from that run's recorded tool calls.

#### Scenario: Objects and relationships created
- **WHEN** a completed run's tool calls include successful `entity-create`, `entity-update`, `entity-relationship-create`, or note-creation calls
- **THEN** the response SHALL include `objects_created` and `objects_updated` counts
- **THEN** the response SHALL include a `relationships_created` count
- **THEN** the response SHALL include a list of created object/relationship identifiers when available from the tool call output

#### Scenario: Discovered types are surfaced
- **WHEN** a completed run created entities or relationships of one or more types
- **THEN** the response SHALL include a `discovered_types` list of the distinct types touched

#### Scenario: No graph mutations occurred
- **WHEN** a completed run made no graph-mutating tool calls (e.g., it determined nothing new was worth saving)
- **THEN** the response SHALL report zero counts rather than omitting the fields
- **THEN** the response SHALL include a `summary` string indicating no changes were made

#### Scenario: Partial failures within a run
- **WHEN** some graph-mutating tool calls within the run failed while others succeeded
- **THEN** the reported counts SHALL reflect only successful mutations
- **THEN** the response MAY include a note that some operations failed, without failing the overall status if the run itself completed

### Requirement: remember-status is scoped to the caller's project
`remember-status` SHALL only report on runs that belong to the authenticated caller's project.

#### Scenario: Cross-project access denied
- **WHEN** a caller supplies a `run_id` belonging to a different project
- **THEN** the system SHALL treat it identically to a not-found run_id
