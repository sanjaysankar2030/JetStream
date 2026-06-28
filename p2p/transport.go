package p2p

// Peer is a representation of a 'remote node'
// i.e another person or a process
type Peer interface {
	ConnClose() error
}

// Transport is anything that handles the communication between nodes in the network
type Transport interface {
	Dial(string) error
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
}
