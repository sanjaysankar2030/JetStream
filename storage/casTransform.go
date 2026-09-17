package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// CASPathTransformFunc derives a content-addressed storage path for a given key.
// It computes the SHA-1 of the key, splits the 40-char hex string into 5-char blocks,
// joins them with '/' as the directory path, and uses the full hash as the filename.
func CASPathTransformFunc(key string) PathKey {
	hash := sha1.Sum([]byte(key))
	hashStr := hex.EncodeToString(hash[:])
	blocksize := 5
	sliceLen := len(hashStr) / blocksize
	paths := make([]string, sliceLen)
	for i := range sliceLen {
		from, to := i*blocksize, (i*blocksize)+blocksize
		paths[i] = hashStr[from:to]
	}
	return PathKey{
		PathName: strings.Join(paths, "/"),
		Filename: hashStr,
	}
}

// CASPathTrasformFunc is an alias for backward compatibility.
var CASPathTrasformFunc = CASPathTransformFunc
