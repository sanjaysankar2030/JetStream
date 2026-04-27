package p2p

import "net"

type RPC struct {
	FromPort net.Addr
	Payload  []byte
}
