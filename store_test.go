package main

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestPathTrasformFunc(t *testing.T) {
	key := "noods"
	pathKey := CASPathTrasformFunc(key)
	expectedOriginalKey := "f87e0e4b4b505750ffb00a1fccc0c132bf24c4ef"
	expectedPathName := "f87e0/e4b4b/50575/0ffb0/0a1fc/cc0c1/32bf2/4c4ef"
	if pathKey.PathName != expectedPathName || pathKey.Filename != expectedOriginalKey {
		t.Errorf(" have %s want %s ", pathKey.PathName, expectedPathName)
		t.Errorf(" have %s want %s ", pathKey.Filename, expectedOriginalKey)
	}
}

func TestStoreDeleteKey(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: CASPathTrasformFunc,
	}
	key := "noods"
	s := NewStore(opts)
	// Reader reads the slice of bytes
	data := []byte("some png")
	if err := s.writeStream(key, bytes.NewReader(data)); err != nil {
		t.Error(err)
	}
	if err := s.Delete(key); err != nil {
		t.Error(err)
	}
}

func TestStore(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: CASPathTrasformFunc,
	}
	key := "noods"
	s := NewStore(opts)
	// Reader reads the slice of bytes
	data := []byte("some png")
	if err := s.writeStream(key, bytes.NewReader(data)); err != nil {
		t.Error(err)
	}
	r, err := s.Read(key)
	if err != nil {
		t.Error(err)
	}
	b, _ := io.ReadAll(r)
	fmt.Println(string(b))
	if string(b) != string(data) {
		t.Errorf("Want | %s | Have | %s | ", b, data)
	}
	if string(b) == string(data) {
		fmt.Printf("Bytes | %s |  ", b)
	}
}
