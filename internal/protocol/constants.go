package protocol

const (
	packetSingle = -1
	packetMulti  = -2
)

const (
	HeaderChallenge = 0x41
	HeaderInfo      = 0x49
	HeaderPlayers   = 0x44
	HeaderRules     = 0x45
)

const (
	RequestInfo    = 0x54
	RequestPlayers = 0x55
	RequestRules   = 0x56
)

// PacketKind identifies top-level packet framing.
type PacketKind int

const (
	PacketUnknown PacketKind = iota
	PacketSingle
	PacketMulti
)
