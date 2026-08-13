package p2p

import (
	"encoding/gob"
	"io"
)

type Decoder interface {
	Decode(io.Reader, *RPC) error
	//io.Reader *RPC
}

// Gob encoder => "encoding/gob"
type GOBDecoder struct{}

func (dec GOBDecoder) Decode(r io.Reader, msg *RPC) error {
	err := gob.NewDecoder(r).Decode(msg)
	return err
}

type DefaultDecoder struct{}

// We are gonna stream data directly to the file
func (dec DefaultDecoder) Decode(r io.Reader, msg *RPC) error {
	buf := make([]byte, 1028)
	n, err := r.Read(buf)
	if err != nil {
		return err
	}
	msg.Payload = buf[:n]
	return nil
}
