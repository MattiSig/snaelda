# Decision 0003: Block Registry Contract Ownership

## Status

Accepted

## Decision

Keep the Go `internal/siteconfig` block registry as the canonical contract for
block type/version metadata, default props, and validation rules. React owns
the block renderer components and editor field components, but it consumes the
contract from Go rather than redefining schemas locally.

The shared proof mechanism is a checked-in fixture at
`internal/siteconfig/testdata/block_registry_contract.json`. Go tests must keep
that fixture synchronized with the registry definitions, and frontend tests must
consume the same fixture to prove the React renderer and editor can handle every
Go-defined block contract.

## Context

The implementation plan requires one shared block contract across generation,
persistence, editing, preview, and publish. The current architecture already
has Go validating and persisting block data while React renders and edits it,
but the boundary was implicit. Without an explicit ownership rule, the frontend
could drift from the backend registry by adding unsupported field controls,
omitting renderer support for a new block type, or assuming default props that
Go rejects.

Using Go as the contract source keeps validation close to persistence and
publish-time guarantees. React still owns presentation concerns, but those
concerns must be checked against the exact contract payload the API exposes.

## Consequences

- New block types or versions must be added to the Go registry first.
- React renderer/editor support is required before a new Go-owned block contract
  is considered complete.
- The shared fixture and its tests are the regression barrier for contract
  drift.
- React should not introduce a second source of truth for block validation
  schemas.
