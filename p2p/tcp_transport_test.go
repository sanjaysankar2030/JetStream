package p2p

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTCPTransport(t *testing.T) {
	listenAddr := ":16969"
	opts := TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandShakeFunc: NOPHandShakeFunc,
		Decoder:       DefaultDecoder{},
	}
	tr := NewTCPTransport(opts)
	assert.Equal(t, listenAddr, tr.Addr())

	err := tr.ListenAndAccept()
	assert.NoError(t, err)
	defer tr.Close()

	// Connect a client
	conn, err := net.Dial("tcp", "127.0.0.1"+listenAddr)
	assert.NoError(t, err)
	defer conn.Close()

	// Send an IncomingMessage with 4-byte length prefix
	payload := []byte("ping payload")
	_, err = conn.Write([]byte{IncomingMessage})
	assert.NoError(t, err)

	err = binary.Write(conn, binary.BigEndian, int32(len(payload)))
	assert.NoError(t, err)

	_, err = conn.Write(payload)
	assert.NoError(t, err)

	// Consume message
	select {
	case rpc := <-tr.Consume():
		assert.False(t, rpc.Stream)
		assert.Equal(t, payload, rpc.Payload)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for RPC")
	}
}
