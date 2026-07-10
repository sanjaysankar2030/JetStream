package main

import (
	"jetstream/p2p"
	"jetstream/server"
	"jetstream/storage"
	"log"
)

func main() {
	tcpTransportOpts := p2p.TCPTransportOpts{
		ListenAddr:    ":3000",
		HandShakeFunc: p2p.NOPHandShakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}
	tcpTransport := p2p.NewTcpTranport(tcpTransportOpts)
	FileServerOpts := server.P2PServerOpts{
		ListenAddr:        ":3000",
		StorageRoot:       ":3000_network",
		PathTransformFunc: storage.CASPathTrasformFunc,
		Transport:         tcpTransport,
	}
	s := server.NewP2PServer(FileServerOpts)
	if err := s.Start(); err != nil {
		log.Fatal(err)
	}
	select {}
}
