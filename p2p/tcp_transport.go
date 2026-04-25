package p2p

import (
	"fmt"
	"net"
	"sync"
)

// Represent a node over a 'tcp' connection
type TCPPeer struct {
	// conn => Connection of the represented node
	conn net.Conn
	// if we Dial a connection => outbound == true
	// if we accept and retrieve a  connection => outbound == false
	outbound bool
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		conn:     conn,
		outbound: outbound,
	}
}

// TCPTransport is a data transporting layer that uses tcp sockets
type TCPTransport struct {
	listenAddress string
	listener      net.Listener
	mu            sync.RWMutex      //Mutex to stop race condtion in peer
	peers         map[net.Addr]Peer // HashTable of peers
}

func NewTcpTranport(listenAddr string) *TCPTransport {
	return &TCPTransport{
		listenAddress: listenAddr,
	}
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error

	t.listener, err = net.Listen("tcp", t.listenAddress)
	if err != nil {
		return err
	}
	go t.startAccepLoop()

	return nil
}

func (t *TCPTransport) startAccepLoop() {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			fmt.Printf("Tcp accept error : %s\n", err)
		}
		go t.handleConn(conn)
	}
}

func (t *TCPTransport) handleConn(conn net.Conn) {
	peer := NewTCPPeer(conn, true)
	fmt.Printf(" new incoming connection :  %+v\n ", peer)
}
