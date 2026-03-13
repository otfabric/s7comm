package model

// Fingerprint contains detailed device fingerprint information
type Fingerprint struct {
	CPUType       string
	CPUGeneration int
	OrderNumber   string
	FirmwareVer   string
	ModuleName    string
	PDUSize       int
	MaxAmqCalling int
	MaxAmqCalled  int
	DetectedRack  int
	DetectedSlot  int
	AcceptedTSAP  uint16
	Protection    ProtectionLevel

	// Capability flags
	SupportsRead   bool
	SupportsWrite  bool
	SupportsSZL    bool
	SupportsUpload bool
	SupportsPI     bool
}

// DiscoverResult represents a discovery scan result
type DiscoverResult struct {
	IP      string
	Port    int
	IsS7    bool
	Rack    int
	Slot    int
	PDUSize int
	CPUType string
	Error   string
}

// TSAPProfile represents a TSAP probing result
type TSAPProfile struct {
	LocalTSAP  uint16
	RemoteTSAP uint16
	Success    bool
	PDUSize    int
}
