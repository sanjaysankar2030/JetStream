package storage

import (
	"bytes"
	"io"
	"testing"

	"jetstream/crypto"

	"github.com/stretchr/testify/assert"
)

func TestPathTransformFunc(t *testing.T) {
	key := "noods"
	pathKey := CASPathTransformFunc(key)
	expectedFilename := "f87e0e4b4b505750ffb00a1fccc0c132bf24c4ef"
	expectedPathName := "f87e0/e4b4b/50575/0ffb0/0a1fc/cc0c1/32bf2/4c4ef"

	assert.Equal(t, expectedPathName, pathKey.PathName)
	assert.Equal(t, expectedFilename, pathKey.Filename)
}

func TestStoreWriteReadDelete(t *testing.T) {
	opts := StoreOpts{
		Root:              "test_store_root",
		PathTransformFunc: CASPathTransformFunc,
	}
	s := NewStore(opts)
	defer s.Clear()

	nodeID := "node_alpha"
	key := "mySpecialFile"
	content := []byte("hello distributed file system")

	// Verify not present initially
	assert.False(t, s.Has(nodeID, key))

	// Write
	n, err := s.Write(nodeID, key, bytes.NewReader(content))
	assert.NoError(t, err)
	assert.Equal(t, int64(len(content)), n)

	// Has
	assert.True(t, s.Has(nodeID, key))

	// Read
	size, r, err := s.Read(nodeID, key)
	assert.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)

	readBytes, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, content, readBytes)

	if rc, ok := r.(io.Closer); ok {
		rc.Close()
	}

	// Delete
	err = s.Delete(nodeID, key)
	assert.NoError(t, err)
	assert.False(t, s.Has(nodeID, key))
}

func TestStoreWriteDecrypt(t *testing.T) {
	opts := StoreOpts{
		Root:              "test_store_decrypt_root",
		PathTransformFunc: CASPathTransformFunc,
	}
	s := NewStore(opts)
	defer s.Clear()

	nodeID := "node_beta"
	key := "encryptedDoc"
	plaintext := []byte("secret contents that travel encrypted over the wire")
	encKey := crypto.NewEncryptionKey()

	// Encrypt to buffer
	encBuf := new(bytes.Buffer)
	_, err := crypto.CopyEncrypt(encKey, bytes.NewReader(plaintext), encBuf)
	assert.NoError(t, err)

	// WriteDecrypt onto disk
	n, err := s.WriteDecrypt(encKey, nodeID, key, encBuf)
	assert.NoError(t, err)
	assert.Equal(t, int64(len(plaintext)), n)

	// Read decrypted file from disk
	assert.True(t, s.Has(nodeID, key))
	size, r, err := s.Read(nodeID, key)
	assert.NoError(t, err)
	assert.Equal(t, int64(len(plaintext)), size)

	diskBytes, err := io.ReadAll(r)
	assert.NoError(t, err)
	assert.Equal(t, plaintext, diskBytes)

	if rc, ok := r.(io.Closer); ok {
		rc.Close()
	}
}

func TestNodeNamespacingIsolation(t *testing.T) {
	opts := StoreOpts{
		Root:              "test_store_isolation_root",
		PathTransformFunc: CASPathTransformFunc,
	}
	s := NewStore(opts)
	defer s.Clear()

	key := "shared_key"
	node1 := "node_1111"
	node2 := "node_2222"

	content1 := []byte("content for node 1")
	content2 := []byte("content for node 2")

	_, err := s.Write(node1, key, bytes.NewReader(content1))
	assert.NoError(t, err)

	_, err = s.Write(node2, key, bytes.NewReader(content2))
	assert.NoError(t, err)

	assert.True(t, s.Has(node1, key))
	assert.True(t, s.Has(node2, key))

	// Delete node1's file, node2's must remain
	err = s.Delete(node1, key)
	assert.NoError(t, err)

	assert.False(t, s.Has(node1, key))
	assert.True(t, s.Has(node2, key))

	_, r2, err := s.Read(node2, key)
	assert.NoError(t, err)
	b2, _ := io.ReadAll(r2)
	assert.Equal(t, content2, b2)
	if rc, ok := r2.(io.Closer); ok {
		rc.Close()
	}
}
