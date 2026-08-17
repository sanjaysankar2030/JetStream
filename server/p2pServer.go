package server

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"jetstream/p2p"
	"jetstream/storage"
)

type P2PServerOpts struct {
	ListenAddr        string
	StorageRoot       string
	PathTransformFunc storage.PathTransformFunc
	Transport         p2p.Transport
	BootStrapNodes    []string
}

type Message struct {
	From           string
	MessagePayload any
}

type MessageStoreFile struct {
	Key string
}

type P2PServer struct {
	P2PServerOpts
	peerLock sync.Mutex
	peers    map[string]p2p.Peer

	storage *storage.Store
	quitch  chan struct{}
}

func init() {
	gob.Register(MessageStoreFile{})
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
	return nil
}

func (p *P2PServer) loop() {
	defer func() {
		log.Printf("Server Closed due to the quitch in P2PServer invoked ")
		p.Transport.Close()
	}()
	for {
		select {
		case rpc := <-p.P2PServerOpts.Transport.Consume():
			var msgBlock Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msgBlock); err != nil {
				fmt.Println("Error while packing the payload in loop()")
				log.Fatal(err)
				return
			}
			fmt.Printf(" The Message Recieved in the loop : %+v \n", msgBlock.MessagePayload)

			peer, ok := p.peers[rpc.From]
			if !ok {
				panic("No peers in map peers")
			}
			fmt.Println("Peer data in loop ()", peer)
			log.Println("_____ This is the peer.Read() statement ________")
			b := make([]byte, 1000)
			n, err := peer.Read(b)
			if err != nil {
				log.Println("Error while reading the peer", err)
				panic(err)
			}
			fmt.Println("Data that is Read() ", string(b[:n]))
			// if err := p.handlePayload(&message); err != nil {
			// 	log.Fatal(err)
			// }
			// dataWritten := string(p.Data)
			// fmt.Printf(" The Data Recieved is %+v  \n", dataWritten)
			peer.(*p2p.TCPPeer).Wg.Done()
		case <-p.quitch:
			return
		}
	}
}

// func (p *P2PServer) handlePayload(m *Message) error {
// 	switch v := m.messagePayload.(type) {
// 	case *Payload:
// 		fmt.Println("Recieved Payload", v)
// 	}
// 	return nil
// }

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

func (s *P2PServer) broadcast(msg *Message) error {
	broadcastNetwork := []io.Writer{}
	for _, peer := range s.peers {
		broadcastNetwork = append(broadcastNetwork, peer)
	}
	mw := io.MultiWriter(broadcastNetwork...)
	return gob.NewEncoder(mw).Encode(msg)
}

func (s *P2PServer) StoreData(key string, r io.Reader) error {
	buf := new(bytes.Buffer)
	msg := Message{
		MessagePayload: MessageStoreFile{
			Key: key,
		},
	}

	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}
	for _, peer := range s.peers {
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}

	time.Sleep(time.Second * 3)

	payload := []byte("This is a large file")
	for _, peer := range s.peers {
		if err := peer.Send(payload); err != nil {
			return err
		}
	}

	return nil
	// tempBuff := new(bytes.Buffer)
	// tee := io.TeeReader(r, tempBuff)
	// if err := s.storage.Write(key, tee); err != nil {
	// log.Println("Error while Writing to writer", err)
	// return err
	// }

	// fmt.Println("------------------------------")
	// fmt.Println("Bytes Written ", tempBuff.Bytes())
	// fmt.Println("------------------------------")

	// p := &Payload{
	// Key:  key,
	// Data: tempBuff.Bytes(),
	// }

	// return s.broadcast(&Message{
	// From:           "todo",
	// messagePayload: p,
	// })
}

func (p *P2PServer) Stop() {
	close(p.quitch)
}

func (s *P2PServer) Close() {
	close(s.quitch)
}
