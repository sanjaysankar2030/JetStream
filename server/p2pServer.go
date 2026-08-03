package server

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"jetstream/p2p"
	"jetstream/storage"
	"log"
	"sync"
)

type P2PServerOpts struct {
	ListenAddr        string
	StorageRoot       string
	PathTransformFunc storage.PathTransformFunc
	Transport         p2p.Transport
	BootStrapNodes    []string
}

type Payload struct {
	Key  string
	Data []byte
}

type P2PServer struct {
	P2PServerOpts
	peerLock sync.Mutex
	peers    map[string]p2p.Peer

	storage *storage.Store
	quitch  chan struct{}
}

func NewP2PServer(opts P2PServerOpts) *P2PServer {
	storeOpts := storage.StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}
	return &P2PServer{
		P2PServerOpts: opts,
		storage:       storage.NewStore(storeOpts),
		quitch:        make(chan struct{}),
		peers:         make(map[string]p2p.Peer),
	}
}
func (ps *P2PServer) OnPeer(p p2p.Peer) error {
	ps.peerLock.Lock()
	defer ps.peerLock.Unlock()
	ps.peers[p.ReturnAddr().String()] = p
	log.Println("Connected with the Remote Peer with Adress ", p.ReturnAddr().String(), "And Saved to peer map.")
	// for addr, peer := range ps.peers {
	// 	fmt.Printf("Current  Addr [%s] Peers [%+v] \n", addr, peer)
	// }
	return nil
}

func (p *P2PServer) loop() {
	defer func() {
		log.Printf("Server Closed due to the quitch in P2PServer invoked ")
		p.Transport.Close()
	}()
	for {
		select {
		case msg := <-p.P2PServerOpts.Transport.Consume():
			fmt.Printf(" The Message Recieved is %s  \n", msg)
		case <-p.quitch:
			return
		}
	}
}

func (p *P2PServer) bootStrapNetwork() error {
	var err error
	for _, addr := range p.BootStrapNodes {
		go func(addr string, err error) {
			fmt.Println("Attempting to connect with remote : ", addr)

			if err := p.Transport.Dial(addr); err != nil {
				log.Println("Dial Error | Addr :", addr)
				log.Println("Error while Dialing in Bootstrap", err)
			}
		}(addr, err)
	}
	return err
}

func (p *P2PServer) Start() error {
	if err := p.P2PServerOpts.Transport.ListenAndAccept(); err != nil {
		return err
	}
	p.bootStrapNetwork()
	p.loop()
	return nil
}

func (s *P2PServer) broadcast(p *Payload) error {
	broadcastNetwork := []io.Writer{}
	for _, peer := range s.peers {
		broadcastNetwork = append(broadcastNetwork, peer)
	}
	mw := io.MultiWriter(broadcastNetwork...)
	return gob.NewEncoder(mw).Encode(p)
}

func (s *P2PServer) StoreData(key string, r io.Reader) error {
	if err := s.storage.Write(key, r); err != nil {
		log.Println("Error while Writing to writer", err)
		return err
	}
	tempBuff := new(bytes.Buffer)
	if _, err := io.Copy(tempBuff, r); err != nil {
		log.Println("Error while Copying ", err)
		return err
	}
	log.Println("Bytes Written ", tempBuff)

	p := &Payload{
		Key:  key,
		Data: tempBuff.Bytes(),
	}

	return s.broadcast(p)
}

func (p *P2PServer) Stop() {
	close(p.quitch)
}

func (s *P2PServer) Close() {
	close(s.quitch)
}
