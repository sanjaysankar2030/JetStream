package server

import "io"

// FileServer is the interface for a distributed file system node.
type FileServer interface {
	Start() error
	Stop()
	Store(string, io.Reader) error
	StoreData(string, io.Reader) error
	Get(string) (io.Reader, error)
	Close()
}
