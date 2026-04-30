package main

import (
	"bytes"
	"fmt"
	"testing"
)

func TestPathTrasformFunc(t *testing.T) {
	key := "noods"
	pathName := CASPathTrasformFunc(key)
	expectedPathName := "f87e0/e4b4b/50575/0ffb0/0a1fc/cc0c1/32bf2/4c4ef"
	if pathName != expectedPathName{
		t.Errorf(" have %s want %s ",pathName,expectedPathName)
	}
}

func TestStore(t *testing.T) {
	opts := StoreOpts{
		PathTransformFunc: CASPathTrasformFunc,
	}
	s := NewStore(opts)
	data := bytes.NewReader([]byte("some png"))
	if err := s.writeStream("noods", data); err != nil {
		t.Error(err)
	}
}
