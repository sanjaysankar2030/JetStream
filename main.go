package main

import (
	"fmt"
	"jetstream/p2p"
	"log"
)

func OnPeer(peer p2p.Peer) error {
	fmt.Println("Doing some logic outside of the TCPTransport")
	peer.Close()
	return nil
}

func main() {
	opts := p2p.TCPTransportOpts{
		ListenAddr:    ":6969",
		HandShakeFunc: p2p.NOPHandShakeFunc,
		Decoder:       p2p.DefaultDecoder{},
		OnPeer:        OnPeer,
	}
	tr := p2p.NewTcpTranport(opts)

	go func() {
		for {
			msg := <-tr.Consume()
			fmt.Printf("Message %+v \n", msg)
		}
	}()

	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}
	select {}
}
