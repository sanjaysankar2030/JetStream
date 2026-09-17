package crypto

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptionRoundTrip(t *testing.T) {
	key := NewEncryptionKey()
	assert.Equal(t, 32, len(key))

	payloads := [][]byte{
		[]byte("hello distributed world"),
		[]byte(""),
		bytes.Repeat([]byte("A"), 1024*64), // 64 KB
	}

	for _, original := range payloads {
		src := bytes.NewReader(original)
		encryptedBuf := new(bytes.Buffer)

		nEnc, err := CopyEncrypt(key, src, encryptedBuf)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(original)+16), nEnc)

		decryptedBuf := new(bytes.Buffer)
		nDec, err := CopyDecrypt(key, encryptedBuf, decryptedBuf)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(original)), nDec)

		assert.Equal(t, original, decryptedBuf.Bytes())
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()
	assert.Equal(t, 64, len(id1))
	assert.Equal(t, 64, len(id2))
	assert.NotEqual(t, id1, id2)
}

func TestHashKey(t *testing.T) {
	h1 := HashKey("my-file-key")
	h2 := HashKey("my-file-key")
	h3 := HashKey("different-key")

	assert.Equal(t, 64, len(h1)) // SHA-256 hex string length is 64
	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
}

func TestInvalidKey(t *testing.T) {
	shortKey := []byte("too-short")
	_, err := CopyEncrypt(shortKey, bytes.NewReader([]byte("test")), io.Discard)
	assert.Error(t, err)

	_, err = CopyDecrypt(shortKey, bytes.NewReader([]byte("test")), io.Discard)
	assert.Error(t, err)
}
