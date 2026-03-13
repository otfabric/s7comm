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

	"otfabric/s7comm/client"
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
