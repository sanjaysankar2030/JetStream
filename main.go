package main

import (
	"jetstream/p2p"
	"log"
)

func main() {
	opts := p2p.TCPTransportOpts{
		ListenAddr:    ":6969",
		HandShakeFunc: p2p.NOPHandShakeFunc,
		Decoder:       p2p.DefaultDecoder{},
	}
	tr := p2p.NewTcpTranport(opts)
	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}
	select {}
}
