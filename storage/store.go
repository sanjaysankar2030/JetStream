package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"jetstream/crypto"
)

const defaultRoot string = "stream"

type PathTransformFunc func(string) PathKey

type PathKey struct {
	PathName string
	Filename string
}

func (p PathKey) FullPath() string {
	return filepath.ToSlash(filepath.Join(p.PathName, p.Filename))
}

func (p PathKey) FirstPathName() string {
	parts := strings.Split(filepath.ToSlash(p.PathName), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
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

func (s *Store) pathFor(id, subpath string) string {
	if len(id) == 0 {
		return filepath.Join(s.Root, filepath.FromSlash(subpath))
	}
	return filepath.Join(s.Root, id, filepath.FromSlash(subpath))
}

// Has checks if the file exists on disk under root/<id>/<pathKey>.
func (s *Store) Has(id, key string) bool {
	pathKey := s.PathTransformFunc(key)
	fullPath := s.pathFor(id, pathKey.FullPath())
	_, err := os.Stat(fullPath)
	return !errors.Is(err, os.ErrNotExist) && err == nil
}

// Write writes raw bytes from reader to disk under root/<id>/<pathKey>.
// Returns the number of bytes written.
func (s *Store) Write(id, key string, r io.Reader) (int64, error) {
	pathKey := s.PathTransformFunc(key)
	dirPath := s.pathFor(id, pathKey.PathName)
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return 0, err
	}

	fullPath := s.pathFor(id, pathKey.FullPath())
	f, err := os.Create(fullPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return io.Copy(f, r)
}

// WriteDecrypt decrypts AES-CTR encrypted stream from r and writes plaintext to disk.
// Returns the number of plaintext bytes written.
func (s *Store) WriteDecrypt(encKey []byte, id, key string, r io.Reader) (int64, error) {
	pathKey := s.PathTransformFunc(key)
	dirPath := s.pathFor(id, pathKey.PathName)
	if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return 0, err
	}

	fullPath := s.pathFor(id, pathKey.FullPath())
	f, err := os.Create(fullPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return crypto.CopyDecrypt(encKey, r, f)
}

// Read opens the file under root/<id>/<pathKey> and returns its size and reader.
func (s *Store) Read(id, key string) (int64, io.Reader, error) {
	pathKey := s.PathTransformFunc(key)
	fullPath := s.pathFor(id, pathKey.FullPath())
	f, err := os.Open(fullPath)
	if err != nil {
		return 0, nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return 0, nil, err
	}
	return fi.Size(), f, nil
}

// Delete removes the entire first directory segment for the key under root/<id>/.
func (s *Store) Delete(id, key string) error {
	pathKey := s.PathTransformFunc(key)
	first := pathKey.FirstPathName()
	if len(first) == 0 {
		return nil
	}
	targetDir := s.pathFor(id, first)
	return os.RemoveAll(targetDir)
}

// Clear removes the entire root directory.
func (s *Store) Clear() error {
	return os.RemoveAll(s.Root)
}
