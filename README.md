# otfabric/s7comm - Siemens S7 Protocol Library for Go

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![CI](https://github.com/otfabric/s7comm/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/s7comm/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/otfabric/s7comm?style=flat&color=blue)](https://github.com/otfabric/s7comm/releases)

A pure Go implementation of the Siemens S7 communication protocol.

The library provides:

- S7 client connection setup (TPKT + COTP + S7 setup communication)
- Read/write operations for DB, inputs, outputs, and merkers
- Device discovery over CIDR ranges with rack/slot probing
- SZL-based identification and diagnostics helpers
- Block listing, block metadata retrieval, and block upload
- Low-level wire parsing/encoding packages for protocol internals

For complete API details, see [API.md](API.md).

## Install

```sh
go get otfabric/s7comm
```

Requires Go 1.25 or later.

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/otfabric/s7comm/client"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := client.New("192.168.0.10", client.WithRackSlot(0, 1))
	if err := c.Connect(ctx); err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	data, err := c.ReadDB(ctx, 1, 0, 16)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("DB1.DBB0..15 = % X\n", data)
}
```

## Discovery

```go
results, err := client.Discover(ctx, "192.168.0.0/24",
	client.WithDiscoverParallel(20),
	client.WithDiscoverRackSlotRange(0, 3, 0, 5),
)
```

Each result reports IP/port reachability, detected rack/slot, negotiated PDU size, and TSAP.

## Rack/Slot Probe

Probe a target for accepted rack/slot combinations:

```go
result, err := client.ProbeRackSlots(ctx, client.RackSlotProbeRequest{
	Address:     "192.168.0.10",
	Port:        102,
	RackMin:     0,
	RackMax:     7,
	SlotMin:     0,
	SlotMax:     31,
	Timeout:     2 * time.Second,
	Parallelism: 4,
})
```

Each candidate has a `Status` and `Stage`. Without `Strict`, "valid" means setup was accepted (`setup-only`, `valid-connect`, or `valid-query`). With **strict mode** (`Strict: true`), "valid" means only **valid-query**: setup succeeded and a benign follow-up S7 query (e.g. SZL or CPU state) also succeeded. This avoids false positives from permissive gateways or simulators that accept setup but do not map to a real CPU.

Strict mode with default confirmation (try SZL, then CPU state, then protection):

```go
result, err := client.ProbeRackSlots(ctx, client.RackSlotProbeRequest{
	Address: "192.168.0.10",
	Port:    102,
	Strict:  true,  // equivalent to Confirm: client.ConfirmAny
})
```

Use a specific confirmation strategy:

```go
result, err := client.ProbeRackSlots(ctx, client.RackSlotProbeRequest{
	Address:  "192.168.0.10",
	Strict:   true,
	Confirm:  client.ConfirmSZL,  // or ConfirmCPUState, ConfirmAny
})
```

The result exposes summary counts: **SetupAccepted**, **ConfirmedByQuery**, and **TCPOnly**. In strict mode only candidates with `valid-query` are included in `result.Valid`.

| Status           | Meaning                                                         |
|------------------|------------------------------------------------------------------|
| `valid-query`    | Setup ok and follow-up query succeeded (strongest)               |
| `valid-connect`  | Setup ok; follow-up failed or not attempted                     |
| `setup-only`     | Setup ok; no follow-up (non-strict only)                         |
| `cotp-only`      | COTP ok, S7 setup failed                                         |
| `tcp-only`       | TCP ok, COTP failed                                             |
| `unreachable`    | TCP connect failed                                              |
| `rejected`       | Target rejected (S7 error)                                      |

Use `StopOnFirst: true` to stop after the first valid combination; in strict mode that means the first `valid-query`.

For CLI usage see [s7commctl probe rackslot](https://github.com/otfabric/s7commctl):

```sh
s7commctl probe rackslot --ip 192.168.0.10
s7commctl probe rackslot --ip 192.168.0.10 --strict
s7commctl probe rackslot --ip 192.168.0.10 --confirm szl
s7commctl probe rackslot --ip 192.168.0.10 --strict --first-confirmed
```

## Package Structure

- `client` - High-level client API (connect, read/write, SZL, discovery, blocks)
- `model` - Data models, areas, type decoders/encoders, device fingerprint structures
- `transport` - Transport connection wrapper with timeout/context handling
- `wire` - Low-level protocol encode/decode for TPKT, COTP, and S7 PDUs

## Development

```sh
make check
```

Useful targets:

- `make test`
- `make coverage`
- `make lint`
- `make lint-ci`
