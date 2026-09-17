package p2p

const (
	IncomingMessage byte = 0x1
	IncomingStream  byte = 0x2
)

// RPC represents a message or raw stream notification from a peer.
type RPC struct {
	From    string // remote address of the sender
	Payload []byte // raw gob-encoded Message bytes (when Stream == false)
	Stream  bool   // true if this RPC signals an incoming raw binary stream
}
