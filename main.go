package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"jetstream/crypto"
	"jetstream/p2p"
	"jetstream/server"
	"jetstream/storage"
)

// Shared cluster encryption key for the demo cluster (256-bit AES)
var clusterKey = crypto.NewEncryptionKey()

func makeServer(listenAddr string, nodes ...string) ( *server.DefaultFileServer) {
	tcpTransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandShakeFunc: p2p.NOPHandShakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}
	tcpTransport := p2p.NewTCPTransport(tcpTransportOpts)
	address := listenAddr[1:]
	opts := server.FileServerOpts{
		EncKey:            clusterKey,
		StorageRoot:       address + "_network",
		PathTransformFunc: storage.CASPathTransformFunc,
		Transport:         tcpTransport,
		BootstrapNodes:    nodes,
	}

	fmt.Printf("[+] Establishing Node on %s with bootstrap nodes %v\n", listenAddr, nodes)
	srv := server.NewFileServer(opts)
	return  srv
}

func main() {

	// 3-Node Distributed Cluster setup per README specification
	s1 := makeServer(":3000")
	s2 := makeServer(":7000", ":3000")
	s3 := makeServer(":5000", ":3000", ":7000")

	// Start Node 1 (Bootstrap root)
	go func() {
		if err := s1.Start(); err != nil {
			log.Fatalf("Node 1 error: %v", err)
		}
	}()
	time.Sleep(500 * time.Millisecond)

	// Start Node 2 (Bootstraps to Node 1)
	go func() {
		if err := s2.Start(); err != nil {
			log.Fatalf("Node 2 error: %v", err)
		}
	}()
	time.Sleep(500 * time.Millisecond)

	// Start Node 3 (Bootstraps to Node 1 and Node 2)
	go func() {
		if err := s3.Start(); err != nil {
			log.Fatalf("Node 3 error: %v", err)
		}
	}()
	time.Sleep(1 * time.Second)

	fmt.Println("\n=======================================================")
	fmt.Println(" JetStream Distributed File System Cluster Running")
	fmt.Println("=======================================================")

	// Store file on Node 3
	fileKey := "myPrivateKey"
	secretData := []byte("my super secret distributed file system data!")
	fmt.Printf("\n[1] Storing file '%s' on Node 3 (:5000)...\n", fileKey)
	if err := s3.Store(fileKey, bytes.NewReader(secretData)); err != nil {
		log.Fatalf("Failed to store on Node 3: %v", err)
	}
	fmt.Printf("✓ Successfully stored '%s' on Node 3 and broadcast to cluster\n", fileKey)

	// Allow replication to propagate
	time.Sleep(1 * time.Second)

	// Retrieve file from Node 1 (:3000)
	fmt.Printf("\n[2] Retrieving file '%s' from Node 1 (:3000)...\n", fileKey)
	r, err := s1.Get(fileKey)
	if err != nil {
		log.Fatalf("Failed to retrieve file from Node 1: %v", err)
	}

	fmt.Print("✓ Retrieved content from Node 1: ")
	if _, err := io.Copy(os.Stdout, r); err != nil {
		log.Fatalf("Failed to read retrieved content: %v", err)
	}
	fmt.Println("\n\n✓ End-to-end distributed Store & Get demo completed successfully!")
	fmt.Println("=======================================================")

	select {}
}


