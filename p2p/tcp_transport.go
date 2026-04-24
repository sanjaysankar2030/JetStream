package p2p

import (
	"fmt"
	"net"
	"sync"
)

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
	fmt.Printf("%+v\n", conn)
}
