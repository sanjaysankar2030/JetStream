package main

import (
	"jetstream/p2p"
	"log"
)

func main() {
	tr := p2p.NewTcpTranport(":6969")
	if err := tr.ListenAndAccept(); err != nil {
		log.Fatal(err)
	}
	select {}

}
