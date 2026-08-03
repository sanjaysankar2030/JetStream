package p2p

import "net"

// Peer is a representation of a 'remote node'
// i.e another person or a process
type Peer interface {
	net.Conn
	ReturnAddr() net.Addr
	Send([]byte) error
	Close() error
}

// Transport is anything that handles the communication between nodes in the network
type Transport interface {
	Dial(string) error
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
}
