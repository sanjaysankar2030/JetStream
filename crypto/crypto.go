package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// NewEncryptionKey generates a random 32-byte AES-256 key.
func NewEncryptionKey() []byte {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		panic(fmt.Sprintf("crypto: failed to generate encryption key: %v", err))
	}
	return key
}

// GenerateID generates a 64-character hex string from 32 random bytes.
func GenerateID() string {
	buf := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		panic(fmt.Sprintf("crypto: failed to generate node ID: %v", err))
	}
	return hex.EncodeToString(buf)
}

// HashKey computes the SHA-256 hash of the key (replacing broken MD5).
func HashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// CopyEncrypt encrypts src using AES-CTR and writes IV (16 bytes) + ciphertext to dst.
// Returns the total bytes written to dst (16 bytes IV + encrypted data size).
func CopyEncrypt(key []byte, src io.Reader, dst io.Writer) (int64, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("crypto: failed to create cipher: %w", err)
	}

	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return 0, fmt.Errorf("crypto: failed to read IV: %w", err)
	}

	n, err := dst.Write(iv)
	if err != nil {
		return int64(n), fmt.Errorf("crypto: failed to write IV: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	writer := &cipher.StreamWriter{S: stream, W: dst}

	written, err := io.Copy(writer, src)
	if err != nil {
		return int64(n) + written, fmt.Errorf("crypto: encrypt copy failed: %w", err)
	}

	return int64(n) + written, nil
}

// CopyDecrypt reads the 16-byte IV from src and decrypts remaining ciphertext using AES-CTR to dst.
// Returns the total decrypted bytes written to dst.
func CopyDecrypt(key []byte, src io.Reader, dst io.Writer) (int64, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, fmt.Errorf("crypto: failed to create cipher: %w", err)
	}

	iv := make([]byte, block.BlockSize())
	if _, err := io.ReadFull(src, iv); err != nil {
		return 0, fmt.Errorf("crypto: failed to read IV: %w", err)
	}

	stream := cipher.NewCTR(block, iv)
	reader := &cipher.StreamReader{S: stream, R: src}

	written, err := io.Copy(dst, reader)
	if err != nil {
		return written, fmt.Errorf("crypto: decrypt copy failed: %w", err)
	}

	return written, nil
}
