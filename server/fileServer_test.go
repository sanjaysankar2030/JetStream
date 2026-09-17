package server

import (
	"bytes"
	"io"
	"testing"
	"time"

	"jetstream/crypto"
	"jetstream/p2p"
	"jetstream/storage"

	"github.com/stretchr/testify/assert"
)

func makeTestServer(t *testing.T, listenAddr string, encKey []byte, nodes ...string) *DefaultFileServer {
	tcpTransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandShakeFunc: p2p.NOPHandShakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}
	tcpTransport := p2p.NewTCPTransport(tcpTransportOpts)

	root := t.TempDir()
	opts := FileServerOpts{
		EncKey:            encKey,
		StorageRoot:       root,
		PathTransformFunc: storage.CASPathTransformFunc,
		Transport:         tcpTransport,
		BootstrapNodes:    nodes,
	}

	srv := NewFileServer(opts)
	return srv
}

func TestStoreAndGetAcrossNetwork(t *testing.T) {
	clusterKey := crypto.NewEncryptionKey()

	s1 := makeTestServer(t, ":15001", clusterKey)
	s2 := makeTestServer(t, ":15002", clusterKey, ":15001")
	s3 := makeTestServer(t, ":15003", clusterKey, ":15001", ":15002")

	defer func() {
		s1.Stop()
		s2.Stop()
		s3.Stop()
	}()

	go s1.Start()
	time.Sleep(100 * time.Millisecond)
	go s2.Start()
	time.Sleep(100 * time.Millisecond)
	go s3.Start()
	time.Sleep(300 * time.Millisecond)

	testKey := "distributed_doc.txt"
	testContent := []byte("The quick brown fox jumps over the lazy dog - Distributed File System in Go")

	// Node 3 stores the file
	err := s3.Store(testKey, bytes.NewReader(testContent))
	assert.NoError(t, err)

	// Small pause for network replication
	time.Sleep(300 * time.Millisecond)

	// Node 1 retrieves the file from the network
	r, err := s1.Get(testKey)
	assert.NoError(t, err)

	data, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, string(testContent), string(data))
	if rc, ok := r.(io.Closer); ok {
		rc.Close()
	}

	// Verify local caching on Node 1: Get again should succeed immediately
	hashedKey := crypto.HashKey(testKey)
	assert.True(t, s1.storage.Has(s1.ID, hashedKey))

	r2, err := s1.Get(testKey)
	assert.NoError(t, err)
	data2, err := io.ReadAll(r2)
	assert.NoError(t, err)
	assert.Equal(t, string(testContent), string(data2))
	if rc, ok := r2.(io.Closer); ok {
		rc.Close()
	}
}

func TestNetworkGetFetchFromRemotePeer(t *testing.T) {
	clusterKey := crypto.NewEncryptionKey()

	// Start Node 1 (has no peers yet)
	s1 := makeTestServer(t, ":16001", clusterKey)
	defer s1.Stop()
	go s1.Start()
	time.Sleep(100 * time.Millisecond)

	// Node 1 stores a file while isolated
	fileKey := "isolated_file.txt"
	fileData := []byte("Isolated secret data only on Node 1 initially")
	err := s1.Store(fileKey, bytes.NewReader(fileData))
	assert.NoError(t, err)

	// Start Node 2 and bootstrap to Node 1
	s2 := makeTestServer(t, ":16002", clusterKey, ":16001")
	defer s2.Stop()
	go s2.Start()
	time.Sleep(300 * time.Millisecond)

	// Node 2 does not have the file initially
	hashedKey := crypto.HashKey(fileKey)
	assert.False(t, s2.storage.Has(s2.ID, hashedKey))

	// Node 2 fetches the file from the network (Node 1 responds with encrypted stream)
	r, err := s2.Get(fileKey)
	assert.NoError(t, err)

	readBytes, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, string(fileData), string(readBytes))
	if rc, ok := r.(io.Closer); ok {
		rc.Close()
	}

	// Node 2 now has the file cached locally
	assert.True(t, s2.storage.Has(s2.ID, hashedKey))
}
