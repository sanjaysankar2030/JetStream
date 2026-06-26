// 02:18:55
package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// Content Addressable Storage
// uses the hashed key as the name the name of the files to maintain consitency among peers
// returns a string which is the pathName transformed
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

type PathTransformFunc func(string) PathKey

type PathKey struct {
	PathName string
	Filename string
}

func (p PathKey) FullPath() string {
	return fmt.Sprintf("%s/%s", p.PathName, p.Filename)
}

func DefaultPathTransformFunc(key string) string {
	return key
}

type StoreOpts struct {
	PathTransformFunc PathTransformFunc
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	return &Store{
		StoreOpts: opts,
	}
}

func (s *Store) Read(key string) (io.Reader, error) {
	f, err := s.readStream(key)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, f)

	return buf, err
}

func (s *Store) readStream(key string) (io.ReadCloser, error) {
	pathKey := s.PathTransformFunc(key)
	return os.Open(pathKey.FullPath())
}

// TODO : RemoveAll() removes only the file in the directory
// To remove all the dir we might need to pass in the base directory rather than the FullPath()
// FullPath() returns dir/subdir/filename
func (s *Store) Delete(key string) error {
	fmt.Println("Called the Delete Method")
	pathKey := s.PathTransformFunc(key)
	err := os.RemoveAll(pathKey.FullPath())
	log.Printf("deleted FullPath [ %s ]", pathKey.FullPath())
	return err
}

// Instead of Reader we might pass a peers net.Conn as it has a reader
func (s *Store) writeStream(key string, r io.Reader) error {
	pathKey := s.PathTransformFunc(key)
	if err := os.MkdirAll(pathKey.PathName, os.ModePerm); err != nil {
		return err
	}
	pathAndFullPath := pathKey.FullPath()
	f, err := os.Create(pathAndFullPath)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, r)
	if err != nil {
		return err
	}
	log.Printf("written => (%d) to the disk in %s ", n, pathAndFullPath)
	defer f.Close()
	return nil
}
