package p2p

// Peer is a representation of a 'remote node'
// i.e another person or a process
type Peer interface {
}

// Transport is anything that handles the communication between nodes in the network
type Transport interface {
	ListenAndAccept() error
}
