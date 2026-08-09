package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"time"

	"jetstream/gui"
	"jetstream/p2p"
	"jetstream/server"
	"jetstream/storage"

	"gioui.org/app"
)

func makeServer(listenAddr string, nodes ...string) (string, []string, *server.P2PServer) {
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
	fmt.Printf("%s : Server Established \n", listenAddr)
	fmt.Println("Nodes : ", nodes)
	srv := server.NewP2PServer(FileServerOpts)
	tcpTransport.OnPeer = srv.OnPeer
	return listenAddr, nodes, srv
}

func main() {
	guiMode := flag.Bool("gui", false, "Launch GUI mode")
	flag.Parse()

	// Default mode (headless)
	addr1, nodes1, s1 := makeServer(":3000")
	addr2, nodes2, s2 := makeServer(":4000", ":3000")

	if *guiMode {

		var addr []string
		var nodes []string

		addr = append(addr, addr1)
		addr = append(addr, addr2)

		fmt.Println("Addr's returned by make Server")
		nodes = append(nodes, nodes1...)
		nodes = append(nodes, nodes2...)

		fmt.Println("Nodes returned by make Server")
		fmt.Println(nodes)
		guiStart(addr, nodes)

	} else {
		go func() {
			log.Fatal(s1.Start())
		}()
		time.Sleep(1 * time.Second)
		go s2.Start()

		data := bytes.NewReader([]byte("Hello Seamen"))
		s2.StoreData("myPrivateData", data)
		select {}

	}
}
func guiStart(addrList []string, nodesList []string) {
	go func() {
		viz := gui.NewNodeVisualization()
		for _, address := range addrList {
			for _, node := range nodesList {
				viz.AddNode(address, address, node)
			}
		}

		if err := viz.Run(); err != nil {
			log.Fatal(err)
		}
	}()
	app.Main()
}
