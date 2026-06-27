package server

import (
	"jetstream/p2p"
	"jetstream/storage"
)

type P2PServerOpts struct {
	ListenAddr        string
	StorageRoot       string
	PathTransformFunc storage.PathTransformFunc
	transport         p2p.TCPTransport
}

type P2PServer struct {
	P2PServerOpts
	store *storage.Store
}

func NewP2PServer(opts P2PServerOpts) *P2PServer {
	storeOpts := storage.StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}
	return &P2PServer{
		P2PServerOpts: opts,
		store:         storage.NewStore(storeOpts),
	}
}
func (p *P2PServer) Start() error {
	if err := p.P2PServerOpts.transport.ListenAndAccept(); err != nil {
		return err
	}
	return nil

}
