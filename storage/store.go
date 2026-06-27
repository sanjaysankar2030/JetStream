package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const defaultRoot string = "stream"

type PathTransformFunc func(string) PathKey

type PathKey struct {
	PathName string
	Filename string
}

func (p PathKey) FullPath() string {
	return fmt.Sprintf("%s/%s", p.PathName, p.Filename)
}

func (p PathKey) FirstPathName() string {
	path := strings.Split(p.PathName, "/")
	if len(path) == 0 {
		return ""
	}
	return path[0]
}

func DefaultPathTransformFunc(key string) PathKey {
	return PathKey{
		PathName: key,
		Filename: key,
	}
}

type StoreOpts struct {
	Root              string
	PathTransformFunc PathTransformFunc
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}
	if len(opts.Root) == 0 {
		opts.Root = defaultRoot
	}
	return &Store{
		StoreOpts: opts,
	}
}
func (s *Store) Write(key string, r io.Reader) error {
	return s.writeStream(key, r)
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
	return os.Open(s.Root + "/" + pathKey.FullPath())
}

func (s *Store) Delete(key string) error {
	fmt.Println("Called the Delete Method")
	pathKey := s.PathTransformFunc(key)
	defer func() {
		log.Printf("deleted [%s ]this from the folder", s.Root+"/"+pathKey.FirstPathName())
	}()
	return os.RemoveAll(s.Root + "/" + pathKey.FirstPathName())
}

func (s *Store) Has(key string) bool {
	fmt.Println("Called the Has Method -> *Store")
	pathKey := s.PathTransformFunc(key)
	_, err := os.Stat(s.Root + "/" + pathKey.FullPath())
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func (s *Store) Clear() error {
	return os.RemoveAll(s.Root)
}

// Instead of Reader we might pass a peers net.Conn as it has a reader
func (s *Store) writeStream(key string, r io.Reader) error {
	pathKey := s.PathTransformFunc(key)
	if err := os.MkdirAll(s.Root+"/"+pathKey.PathName, os.ModePerm); err != nil {
		return err
	}
	pathAndFullPath := s.Root + "/" + pathKey.FullPath()
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
