package server

import (
	"fmt"
	"io"
	"jetstream/p2p"
	"jetstream/storage"
	"log"
)

type P2PServerOpts struct {
	ListenAddr        string
	StorageRoot       string
	PathTransformFunc storage.PathTransformFunc
	Transport         p2p.Transport
	BootStrapNodes    []string
}

type P2PServer struct {
	P2PServerOpts
	store  *storage.Store
	quitch chan struct{}
}

func NewP2PServer(opts P2PServerOpts) *P2PServer {
	storeOpts := storage.StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}
	return &P2PServer{
		P2PServerOpts: opts,
		store:         storage.NewStore(storeOpts),
		quitch:        make(chan struct{}),
	}
}

func (p *P2PServer) loop() {
	defer func() {
		log.Println("Server Closed due to the quitch in P2PServer invoked ")
		p.Transport.Close()
	}()
	for {
		select {
		case msg := <-p.P2PServerOpts.Transport.Consume():
			fmt.Printf(" The Message Received is %s \n ", msg)
		case <-p.quitch:
			return
		}
	}
}

func (p *P2PServer) bootStrapNetwork() error {
	for _, addr := range p.BootStrapNodes {
		if len(addr) == 0 {
			continue
		}
		go func(addr string) {
			log.Println("Attempting to connect with ", addr)
			if err := p.Transport.Dial(addr); err != nil {
				log.Println("Dial Error : ", err)
			}
		}(addr)
	}
	return nil
}

func (p *P2PServer) Start() error {
	if err := p.P2PServerOpts.Transport.ListenAndAccept(); err != nil {
		return err
	}
	p.bootStrapNetwork()
	log.Printf("Listening with the port %s \n", p.ListenAddr)
	p.loop()
	return nil
}

func (s *P2PServer) Store(key string, r io.Reader) error {
	return s.store.Write(key, r)
}

func (s *P2PServer) Stop() {
	close(s.quitch)
	// s.Transport.Close()
}
