package p2p

import (
	"encoding/binary"
	"encoding/gob"
	"io"
)

// Decoder defines how payload data is decoded from a reader into an RPC.
type Decoder interface {
	Decode(io.Reader, *RPC) error
}

// DefaultDecoder reads a 4-byte big-endian length prefix, followed by that exact number of bytes.
type DefaultDecoder struct{}

func (dec DefaultDecoder) Decode(r io.Reader, msg *RPC) error {
	var length int32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	msg.Payload = buf
	return nil
}

// GOBDecoder decodes gob-encoded RPC directly from the stream.
type GOBDecoder struct{}

func (dec GOBDecoder) Decode(r io.Reader, msg *RPC) error {
	return gob.NewDecoder(r).Decode(msg)
}
