package main

//TODO: Wails golang frontend implementation
import (
	"fmt"
	"jetstream/p2p"
	"jetstream/server"
	"jetstream/storage"
	"log"
)

func makeServer(listenAddr string, nodes ...string) *server.P2PServer {
	tcpTransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandShakeFunc: p2p.NOPHandShakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}
	tcpTransport := p2p.NewTcpTranport(tcpTransportOpts)
	FileServerOpts := server.P2PServerOpts{
		StorageRoot:       listenAddr + "_network",
		PathTransformFunc: storage.CASPathTrasformFunc,
		Transport:         tcpTransport,
		BootStrapNodes:    nodes,
	}
	fmt.Printf("%s : Server Establised \n", listenAddr)
	fmt.Println("Nodes : ", nodes)
	return server.NewP2PServer(FileServerOpts)
}

func main() {
	s1 := makeServer(":3000")
	s2 := makeServer(":4000", ":3000")
	go func() {
		log.Fatal(s1.Start())
	}()
	s2.Start()
}
