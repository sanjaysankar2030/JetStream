package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

func CASPathTrasformFunc(key string) PathKey {
	hash := sha1.Sum([]byte(key))
	hashStr := hex.EncodeToString(hash[:])
	blocksize := 5
	sliceLen := len(hashStr) / blocksize
	paths := make([]string, sliceLen)
	for i := range sliceLen {
		// we are splitting the individual data blocks and using i as a pointer
		from, to := i*blocksize, (i*blocksize)+blocksize
		paths[i] = hashStr[from:to]
	}
	return PathKey{
		PathName: strings.Join(paths, "/"),
		Filename: hashStr,
	}
}
