package main

//TODO: Wails golang frontend implementation
import (
	"bytes"
	"fmt"
	"jetstream/p2p"
	"jetstream/server"
	"jetstream/storage"
	"log"
	"time"
)

func makeServer(listenAddr string, nodes ...string) *server.P2PServer {
	tcpTransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandShakeFunc: p2p.NOPHandShakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}
	tcpTransport := p2p.NewTcpTranport(tcpTransportOpts)
	address := listenAddr[1:]
	FileServerOpts := server.P2PServerOpts{
		StorageRoot:       address + "_network",
		PathTransformFunc: storage.CASPathTrasformFunc,
		Transport:         tcpTransport,
		BootStrapNodes:    nodes,
	}
	fmt.Printf("%s : Server Establised \n", listenAddr)
	fmt.Println("Nodes : ", nodes)
	server := server.NewP2PServer(FileServerOpts)
	tcpTransport.OnPeer = server.OnPeer
	return server
}

func main() {
	s1 := makeServer(":3000")
	s2 := makeServer(":4000", ":3000")
	go func() {
		log.Fatal(s1.Start())
	}()

	go s2.Start()
	time.Sleep(1 * time.Second)

	data := bytes.NewReader([]byte("Hello Seamen"))
	s2.StoreData("myPrivateData", data)
}
