package p2p

import (
	"fmt"
	"net"
	// "sync"
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

/*
Close() => conn.Close()
Close() implements the Peer Interface
*/
func (p *TCPPeer) Close() error {
	return p.conn.Close()
}

type TCPTransportOpts struct {
	ListenAddr    string
	HandShakeFunc HandShakeFunc
	Decoder       Decoder
	OnPeer        func(Peer) error
}

// TCPTransport is a data transporting layer that uses tcp sockets
type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener
	rpcch    chan RPC

	// mu    sync.RWMutex      //Mutex to stop race condtion in peer
	// peers map[net.Addr]Peer // HashTable of peers
}

func NewTcpTranport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
		rpcch:            make(chan RPC),
	}
}

// Consumes a channel implements a Tranport interface
func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcch
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.ListenAddr)
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
		} else {
			fmt.Printf(" new incoming connection :  %+v\n ", conn)
			go t.handleConn(conn)
		}
	}
}

func (t *TCPTransport) handleConn(conn net.Conn) {
	var err error

	// Drops the connection at the end of the function execution
	defer func() {
		fmt.Printf("Dropping peer connection %s \n", err)
		conn.Close()
	}()

	peer := NewTCPPeer(conn, true)
	fmt.Println("NewTCPPeer Established")
	// Checking whether the handshake is established
	if err = t.HandShakeFunc(peer); err != nil {
		return
	}
	fmt.Println("HandShake Established")

	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return
		}
	}
	rpc := RPC{}
	for {
		err := t.Decoder.Decode(conn, &rpc)
		if err != nil {
			return
		}
		rpc.FromPort = conn.RemoteAddr()
		t.rpcch <- rpc
	}
	// PS E:\JetStream> make run
	// new incoming connection :  &{conn:{fd:0xc0000a8a08}}
	//	NewTCPPeer Established
	// HandShake Established
	// Doing some logic outside of the TCPTransport
	// Decoding Complete
	// rpc Port after decoding :  <nil>
	// rpc err after decoding :  read tcp [::1]:6969->[::1]:56487: use of closed network connection
	// Dropping peer connection %!s(<nil>)
	// make: *** [run] Interrupt
	// PS E:\JetStream>
}
