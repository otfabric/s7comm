# Release v0.3.0

**Date:** 2026-03-13
**Previous release:** v0.2.1

## Summary

- **Strict rack/slot probe mode**: `Strict: true` on `RackSlotProbeRequest` restricts "valid" to candidates that complete both S7 setup and a benign follow-up query (`valid-query`). Without strict, any setup success (`setup-only`, `valid-connect`, or `valid-query`) is valid.
- **Confirmation strategies**: When strict, follow-up is configurable via `Confirm`: `szl` (SZL module ID), `cpu-state` (SZL CPU state), or `any` (try SZL, then CPU state, then protection). Default when `Strict` is true is `ConfirmAny`.
- **New probe types**: `ProbeStage` (tcp, cotp, setup, query), `ProbeStatus` (unreachable, tcp-only, cotp-only, setup-only, valid-connect, valid-query, rejected, timeout, flaky), `ConfirmationKind`, and `Confidence`.
- **Extended `RackSlotCandidate`**: `Stage`, `Status`, `ConfirmedBy`, `Confidence`; legacy fields `ReachableTCP`, `ReachableCOTP`, `S7SetupOK`, `SZLQueryOK`, `Classification` retained and derived from `Status`.
- **Result summary**: `RackSlotProbeResult` now includes `SetupAccepted`, `ConfirmedByQuery`, `TCPOnly`, and `Flaky` for honest reporting. In strict mode only `valid-query` candidates appear in `Valid`.
- **Backward compatibility**: Legacy classification constants and candidate fields preserved; existing callers continue to work.

---
# Release v0.2.1

**Date:** 2026-03-13
**Previous release:** v0.2.0

## Summary

- Fix wrong package namespace.
- Fix linting errors.

---

# Release v0.2.0

**Date:** 2026-03-13
**Previous release:** v0.1.0

## Summary

- Added host-oriented rack/slot probe API (`ProbeRackSlots`) as a first-class public function in the `client` package.
- New types: `RackSlotProbeRequest`, `RackSlotCandidate`, `RackSlotProbeResult` covering the full connection classification model from TCP reachability through S7 setup success.
- Each candidate is classified as `valid-query`, `valid-connect`, `cotp-failed`, `tcp-only`, `unreachable`, or `rejected`, giving operators precise visibility into which protocol stage succeeded or failed.
- Probe supports bounded parallelism, configurable rack/slot ranges, per-attempt delays, stop-on-first mode, and optional manual TSAP override.
- An optional benign SZL query (SZL 0x0011) is attempted on successful candidates to elevate confidence from `valid-connect` to `valid-query`.
- All probe logic is non-destructive: only connection and setup traffic, plus a read-only SZL request where possible.
- Exit semantics and JSON/table output are documented for `s7commctl probe rackslot` CLI consumers.

---

# Release v0.1.0

**Date:** 2026-03-13
**Previous release:** v0.0.0

## Summary

- Initial public release of the S7 communication library for Go, with a modular package layout: `client`, `model`, `transport`, and `wire`.
- Added end-to-end S7 client flows for connect/setup, memory read/write operations, device discovery, SZL-based identification/diagnostics, and block upload/listing support.
- Included low-level protocol encoding/decoding for TPKT, COTP, S7 headers, and key PDU message families.
- Expanded and hardened tests around protocol parsing and edge cases, including connection lifecycle cleanup, discovery boundary handling, enum/string behavior, and model encoding safety.
- Fixed reliability issues discovered during review:
	- connection references are now cleared after close/failed handshake paths;
	- defensive bounds handling for string encoding;
	- safe negative-index handling in boolean decoding.
- Added contributor tooling and automation:
	- self-documented Makefile targets for test, coverage, lint, vet, and formatting;
	- GitHub Actions CI workflow (test/lint/coverage);
	- manual release workflow with semantic bumping and GitHub Release creation.

## Known limitations

- Integration coverage is currently strongest for protocol/unit tests; broader multi-model PLC interoperability validation remains in progress.
- Public API documentation is still concise and will be expanded with additional practical recipes (error handling, retry/rate-limit tuning, and block/SZL workflows).
- Transport and wire package coverage can be increased further with more fixture-based table tests for malformed and vendor-variant frames.

---
