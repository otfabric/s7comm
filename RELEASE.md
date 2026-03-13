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
