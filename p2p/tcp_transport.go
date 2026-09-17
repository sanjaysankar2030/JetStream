package p2p

import (
	"errors"
	"io"
	"log"
	"net"
)

// TCPPeer represents a remote node over a TCP connection.
type TCPPeer struct {
	net.Conn
	outbound      bool
	closeStreamCh chan struct{}
}

// NewTCPPeer creates a new TCPPeer wrapping net.Conn.
func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:          conn,
		outbound:      outbound,
		closeStreamCh: make(chan struct{}, 1),
	}
}

// Send sends bytes directly over the TCP connection.
func (p *TCPPeer) Send(b []byte) error {
	_, err := p.Conn.Write(b)
	return err
}

// CloseStream signals that reading the incoming raw binary stream is finished.
func (p *TCPPeer) CloseStream() {
	select {
	case p.closeStreamCh <- struct{}{}:
	default:
	}
}

// ReturnAddr returns the remote network address of this peer.
func (p *TCPPeer) ReturnAddr() net.Addr {
	return p.Conn.RemoteAddr()
}

// TCPTransportOpts holds configuration for TCPTransport.
type TCPTransportOpts struct {
	ListenAddr       string
	HandShakeFunc    HandShakeFunc
	Decoder          Decoder
	OnPeer           func(Peer) error
	OnPeerDisconnect func(Peer)
}

// TCPTransport implements the Transport interface using raw TCP sockets.
type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener
	rpcch    chan RPC
}

// NewTCPTransport creates a new TCPTransport.
func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	if opts.Decoder == nil {
		opts.Decoder = DefaultDecoder{}
	}
	if opts.HandShakeFunc == nil {
		opts.HandShakeFunc = NOPHandShakeFunc
	}
	return &TCPTransport{
		TCPTransportOpts: opts,
		rpcch:            make(chan RPC, 1024),
	}
}

// NewTcpTranport is an alias for backward compatibility.
var NewTcpTranport = NewTCPTransport

// Addr returns the listening address.
func (t *TCPTransport) Addr() string {
	return t.ListenAddr
}

// Consume returns the receive-only channel of RPC messages.
func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcch
}

// Close closes the underlying TCP listener.
func (t *TCPTransport) Close() error {
	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
}

// ListenAndAccept begins listening on the configured address and accepts connections.
func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.ListenAddr)
	if err != nil {
		return err
	}
	go t.startAcceptLoop()
	return nil
}

func (t *TCPTransport) startAcceptLoop() {
	for {
		conn, err := t.listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			log.Printf("TCP accept error: %v\n", err)
			continue
		}
		go t.handleConn(conn, false)
	}
}

// Dial dials a remote peer at addr.
func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	go t.handleConn(conn, true)
	return nil
}

func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {
	peer := NewTCPPeer(conn, outbound)

	defer func() {
		if t.OnPeerDisconnect != nil {
			t.OnPeerDisconnect(peer)
		}
		conn.Close()
	}()

	if err := t.HandShakeFunc(peer); err != nil {
		return
	}

	if t.OnPeer != nil {
		if err := t.OnPeer(peer); err != nil {
			return
		}
	}

	for {
		var msgType [1]byte
		if _, err := io.ReadFull(conn, msgType[:]); err != nil {
			return
		}

		switch msgType[0] {
		case IncomingMessage:
			rpc := RPC{From: conn.RemoteAddr().String(), Stream: false}
			if err := t.Decoder.Decode(conn, &rpc); err != nil {
				return
			}
			t.rpcch <- rpc

		case IncomingStream:
			rpc := RPC{From: conn.RemoteAddr().String(), Stream: true}
			t.rpcch <- rpc
			// Block until CloseStream() is called by the stream consumer
			<-peer.closeStreamCh

		default:
			log.Printf("unknown message type: 0x%x\n", msgType[0])
			return
		}
	}
}
