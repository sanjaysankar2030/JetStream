package p2p

import "net"

// Peer represents a connected remote node.
type Peer interface {
	net.Conn
	Send([]byte) error
	CloseStream()
}

// Transport manages connections — listen, dial, and consume messages.
type Transport interface {
	Addr() string
	Dial(string) error
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
}
