package storage

import (
	"bytes"
	"fmt"
	"io"
	"log"
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
	s := NewStore(opts)
	// defer teardown(t, s)
	for i := range 50 {
		key := fmt.Sprintf("foo_%d", i)
		// New Store Initialization
		// Teardown deletes the root dir which is defaultRoot
		// Reader reads the slice of bytes
		data := []byte("some png")
		if err := s.writeStream(key, bytes.NewReader(data)); err != nil {
			t.Error(err)
		}
		if has := s.Has(key); !has {
			t.Errorf("Expected to have key %s have Null", key)
		}
		r, err := s.Read(key)
		if err != nil {
			log.Print("Error while Reading ")
			t.Error(err)
		}
		b, _ := io.ReadAll(r)
		if string(b) != string(data) {
			t.Errorf("Want | %s | Have | %s | ", b, data)
		}
		del_err := s.Delete(key)
		if del_err != nil {
			t.Errorf("Error while Deletion\n [%s]", del_err)
		}
		fmt.Println("Deletion Succesful")
		if has := s.Has(key); has {
			t.Errorf("Expected to Not have key but have key \n [%s] ", key)
		}
	}
}

func newStore() *Store {
	opts := StoreOpts{
		PathTransformFunc: CASPathTrasformFunc,
	}
	return NewStore(opts)
}

func teardown(t *testing.T, s *Store) {
	if err := s.Clear(); err != nil {
		t.Error(err)
	}
}
