package p2p

import "net"

type Message struct {
	FromPort net.Addr
	Payload  []byte
}
