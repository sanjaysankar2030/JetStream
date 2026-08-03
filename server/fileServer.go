package server

import (
	"io"
)
type FileServer interface {
	Start()error
	Stop()
	StoreData(string , io.Reader)error
	Close()
}
