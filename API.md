# API Reference: otfabric/s7comm

This document describes the public API exposed by the module and gives practical behavior notes.

## Packages

- client: high-level PLC operations
- model: domain data types and value helpers
- transport: TCP I/O wrapper with context-aware deadlines
- wire: low-level protocol encoding and parsing

## client

```go
import "github.com/otfabric/s7comm/client"
```

### Construction and lifecycle

```go
func New(host string, opts ...Option) *Client
func (c *Client) Connect(ctx context.Context) error
func (c *Client) Close() error
func (c *Client) ConnectionInfo() model.ConnectionInfo
func (c *Client) PDUSize() int
```

Behavior notes:

- Connect performs TCP dial, COTP setup, and S7 setup communication.
- On setup failure, connection state is closed and cleared.
- Request methods are serialized internally for protocol safety.

### Read/write API

```go
func (c *Client) ReadArea(ctx context.Context, addr model.Address) ([]byte, error)
func (c *Client) WriteArea(ctx context.Context, addr model.Address, data []byte) error

func (c *Client) ReadDB(ctx context.Context, dbNum, offset, size int) ([]byte, error)
func (c *Client) WriteDB(ctx context.Context, dbNum, offset int, data []byte) error
func (c *Client) ReadInputs(ctx context.Context, offset, size int) ([]byte, error)
func (c *Client) ReadOutputs(ctx context.Context, offset, size int) ([]byte, error)
func (c *Client) ReadMerkers(ctx context.Context, offset, size int) ([]byte, error)
```

Behavior notes:

- ReadArea chunks requests based on negotiated PDU size.
- WriteArea uses WriteVar with optional rate limiting.

### Discovery API

```go
type DiscoverResult struct {
    IP      string
    Port    int
    IsS7    bool
    Rack    int
    Slot    int
    PDUSize int
    TSAP    string
    Error   string
}

func Discover(ctx context.Context, cidr string, opts ...DiscoverOption) ([]DiscoverResult, error)
func WithDiscoverTimeout(ms int) DiscoverOption
func WithDiscoverParallel(n int) DiscoverOption
func WithDiscoverRackSlotRange(rackMin, rackMax, slotMin, slotMax int) DiscoverOption
func WithDiscoverRateLimit(ms int) DiscoverOption
```

Default discovery settings:

- timeout: 2000 ms
- parallel workers: 10
- rack range: 0..3
- slot range: 0..5

### Rack/Slot Probe API

A host-oriented probe that determines which rack/slot combinations are valid for a specific target IP. Intended for pre-connection topology discovery and troubleshooting.

```go
type RackSlotProbeRequest struct {
    Address     string
    Port        int           // default 102
    RackMin     int           // default 0
    RackMax     int           // default 7
    SlotMin     int           // default 0
    SlotMax     int           // default 31
    Timeout     time.Duration // per-attempt timeout; default 2s
    Parallelism int           // concurrent probes; default 4
    DelayMS     int           // delay between attempts in ms; default 0
    StopOnFirst bool          // stop after first valid candidate

    // optional manual TSAP override (bypasses rack/slot-derived TSAP)
    LocalTSAP  *uint16
    RemoteTSAP *uint16
}

type RackSlotCandidate struct {
    Rack           int
    Slot           int
    ReachableTCP   bool
    ReachableCOTP  bool
    S7SetupOK      bool
    SZLQueryOK     bool
    PDUSize        int
    LocalTSAP      uint16
    RemoteTSAP     uint16
    Classification string // see table below
    Error          string
}

type RackSlotProbeResult struct {
    Address    string
    Candidates []RackSlotCandidate
    Valid      []RackSlotCandidate
}

func ProbeRackSlots(ctx context.Context, req RackSlotProbeRequest) (*RackSlotProbeResult, error)
```

Classification values:

| Value           | Meaning                                              |
|-----------------|------------------------------------------------------|
| `valid-query`   | S7 setup + benign SZL read both succeeded            |
| `valid-connect` | S7 setup succeeded; SZL not attempted or failed      |
| `rejected`      | COTP connected but S7 setup was rejected by the PLC  |
| `cotp-failed`   | TCP reachable, COTP session not accepted             |
| `tcp-only`      | TCP reachable, no recognisable COTP/S7 response      |
| `unreachable`   | TCP connect failed                                   |

Behavior notes:

- A candidate is considered **valid** when `S7SetupOK` is true (Classification `valid-connect` or better).
- `SZLQueryOK` is attempted on each valid-connect candidate using a benign SZL 0x0011 read.
- Remote TSAP is derived from rack/slot (PG convention: `0x03RS`) unless `RemoteTSAP` override is set.
- Parallelism is bounded per-host; candidates are collected in rack-major, slot-minor order.
- Probe is fully non-destructive: only connection/setup and read-only SZL traffic is used.

### Identification, diagnostics, and blocks

```go
func (c *Client) Identify(ctx context.Context) (*model.DeviceInfo, error)
func (c *Client) GetCPUState(ctx context.Context) (model.CPUState, error)
func (c *Client) GetProtectionLevel(ctx context.Context) (model.ProtectionLevel, error)
func (c *Client) ReadDiagBuffer(ctx context.Context) (*model.DiagBuffer, error)

func (c *Client) ListBlocks(ctx context.Context, bt model.BlockType) ([]model.BlockInfo, error)
func (c *Client) ListAllBlocks(ctx context.Context) ([]model.BlockInfo, error)
func (c *Client) GetBlockInfo(ctx context.Context, bt model.BlockType, num int) (*model.BlockInfo, error)
func (c *Client) UploadBlock(ctx context.Context, bt model.BlockType, num int) (*model.BlockData, error)
```

### Client options

```go
type Option func(*options)

type Logger interface {
    Debug(msg string, args ...interface{})
    Info(msg string, args ...interface{})
    Error(msg string, args ...interface{})
}

func WithPort(port int) Option
func WithRackSlot(rack, slot int) Option
func WithTSAP(local, remote uint16) Option
func WithAutoRackSlot(brute bool) Option
func WithTimeout(t time.Duration) Option
func WithRateLimit(d time.Duration) Option
func WithLogger(l Logger) Option
func WithMaxPDU(size int) Option
```

Defaults:

- port: 102
- rack/slot: 0/1
- timeout: 5s
- max PDU request: 480

## model

```go
import "github.com/otfabric/s7comm/model"
```

### Addressing and enums

```go
type Area uint8

const (
    AreaInputs  Area = 0x81
    AreaOutputs Area = 0x82
    AreaMerkers Area = 0x83
    AreaDB      Area = 0x84
    AreaCounter Area = 0x1C
    AreaTimer   Area = 0x1D
)

func (a Area) String() string

type Address struct {
    Area     Area
    DBNumber int
    Start    int
    Size     int
}
```

### Blocks and device metadata

```go
type BlockType uint8
func (b BlockType) String() string

type BlockLang uint8
func (l BlockLang) String() string

type BlockInfo struct { ... }
type BlockData struct { ... }

type DeviceInfo struct { ... }
type ConnectionInfo struct { ... }

type CPUState uint8
func (s CPUState) String() string

type ProtectionLevel uint8
func (p ProtectionLevel) String() string

type DiagEntry struct { ... }
type DiagBuffer struct { ... }

type Fingerprint struct { ... }
type DiscoverResult struct { ... }
type TSAPProfile struct { ... }
```

### Value encoders/decoders

```go
func DecodeBool(data []byte, bit int) bool
func DecodeByte(data []byte) byte
func DecodeWord(data []byte) uint16
func DecodeInt(data []byte) int16
func DecodeDWord(data []byte) uint32
func DecodeDInt(data []byte) int32
func DecodeReal(data []byte) float32
func DecodeString(data []byte) string

func EncodeBool(val bool) []byte
func EncodeByte(val byte) []byte
func EncodeWord(val uint16) []byte
func EncodeInt(val int16) []byte
func EncodeDWord(val uint32) []byte
func EncodeDInt(val int32) []byte
func EncodeReal(val float32) []byte
func EncodeString(val string, maxLen int) []byte
```

Notes:

- Numeric values are big-endian.
- DecodeBool returns false for invalid/negative indexes.
- EncodeString uses S7 string layout and clamps total length to [2, 256].

## transport

```go
import "github.com/otfabric/s7comm/transport"
```

```go
var ErrConnectionNotEstablished = errors.New("connection not established")

type Conn struct { ... }

type Tracer interface {
    Trace(direction string, data []byte)
}

func New(conn net.Conn, timeout time.Duration) *Conn
func (c *Conn) SetTracer(t Tracer)
func (c *Conn) Send(data []byte) error
func (c *Conn) SendContext(ctx context.Context, data []byte) error
func (c *Conn) Receive() ([]byte, error)
func (c *Conn) ReceiveContext(ctx context.Context) ([]byte, error)
func (c *Conn) Close() error
func (c *Conn) LocalAddr() net.Addr
func (c *Conn) RemoteAddr() net.Addr
```

## wire

```go
import "github.com/otfabric/s7comm/wire"
```

### Framing and S7 headers

```go
func EncodeTPKT(payload []byte) []byte
func ParseTPKT(data []byte) (*TPKT, []byte, error)

func EncodeCOTPCR(localTSAP, remoteTSAP uint16) []byte
func EncodeCOTPData() []byte
func ParseCOTP(data []byte) (*COTP, []byte, error)
func BuildTSAP(connType, rack, slot int) uint16

func EncodeS7Header(rosctr byte, pduRef uint16, paramLen, dataLen int) []byte
func ParseS7Header(data []byte) (*S7Header, []byte, error)
```

### PDUs

```go
func EncodeSetupCommRequest(maxAmqCalling, maxAmqCalled, pduSize int) []byte
func ParseSetupCommResponse(data []byte) (*SetupCommResponse, error)

func EncodeS7Any(addr S7AnyAddress) []byte
func EncodeReadVarRequest(pduRef uint16, addrs []S7AnyAddress) []byte
func ParseReadVarResponse(param, data []byte) ([]ReadVarItem, error)
func EncodeWriteVarRequest(pduRef uint16, addr S7AnyAddress, value []byte) []byte
func ParseWriteVarResponse(param, data []byte) error

func EncodeSZLRequest(pduRef, szlID, szlIndex uint16) []byte
func ParseSZLResponse(paramData, data []byte) (*SZLResponse, error)

func EncodeBlockListRequest(pduRef uint16, blockType byte) []byte
func ParseBlockListResponse(szlData []byte) ([]BlockListEntry, error)
func EncodeStartUploadRequest(pduRef uint16, blockType byte, blockNum int) []byte
func ParseStartUploadResponse(param []byte) (string, error)
func EncodeUploadRequest(pduRef uint16, sessionID string) []byte
func EncodeEndUploadRequest(pduRef uint16, sessionID string) []byte
func ParseUploadResponse(param, data []byte) (*UploadChunk, error)
```

### Inspection and errors

```go
func InspectFrame(frame []byte) (*FrameSummary, error)

func NewS7Error(class, code byte) *S7Error
func ReturnCodeError(code byte) error
```

Key sentinel errors include short/invalid TPKT, COTP, and S7 headers and payload length mismatches.
