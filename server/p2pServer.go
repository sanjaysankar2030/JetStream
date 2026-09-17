package server

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"jetstream/crypto"
	"jetstream/p2p"
	"jetstream/storage"
)

func init() {
	gob.Register(MessageStoreFile{})
	gob.Register(MessageGetFile{})
}

// MessageStoreFile is sent before streaming an encrypted file to peers.
type MessageStoreFile struct {
	ID   string // Sender node ID (used for namespacing)
	Key  string // Hashed key of the file
	Size int64  // Encrypted file size in bytes (plaintext size + 16 for IV)
}

// MessageGetFile is broadcast to query peers for a file.
type MessageGetFile struct {
	ID  string // Requesting node ID
	Key string // Hashed key of the file
}

// Message is the gob-encoded protocol envelope.
type Message struct {
	Payload any
}

// FileServerOpts defines the configuration for a FileServer node.
type FileServerOpts struct {
	ID                string
	EncKey            []byte
	StorageRoot       string
	PathTransformFunc storage.PathTransformFunc
	Transport         p2p.Transport
	BootstrapNodes    []string
}

// P2PServerOpts is an alias for FileServerOpts.
type P2PServerOpts = FileServerOpts

// DefaultFileServer implements the distributed FileServer node.
type DefaultFileServer struct {
	FileServerOpts

	peerLock sync.RWMutex
	peers    map[string]p2p.Peer

	storage *storage.Store
	quitch  chan struct{}

	pendingLock  sync.Mutex
	pendingStore map[string]MessageStoreFile // maps remoteAddr -> MessageStoreFile

	inflightLock sync.Mutex
	inflightGets map[string]chan struct{} // maps hashedKey -> channel
}

// FileServer alias
type P2PServer = DefaultFileServer

// NewFileServer constructs a new FileServer node.
func NewFileServer(opts FileServerOpts) *DefaultFileServer {
	if len(opts.ID) == 0 {
	
	opts.ID = crypto.GenerateID()
	}
	if len(opts.EncKey) == 0 {
		opts.EncKey = crypto.NewEncryptionKey()
	} else if len(opts.EncKey) != 32 {
		panic(fmt.Sprintf("FileServer: EncKey must be exactly 32 bytes (got %d)", len(opts.EncKey)))
	}
	if len(opts.StorageRoot) == 0 {
		opts.StorageRoot = "dfs_network"
	}
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = storage.CASPathTransformFunc
	}

	storeOpts := storage.StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}

	fs := &DefaultFileServer{
		FileServerOpts: opts,
		storage:        storage.NewStore(storeOpts),
		quitch:         make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
		pendingStore:   make(map[string]MessageStoreFile),
		inflightGets:   make(map[string]chan struct{}),
	}

	if tcpTr, ok := opts.Transport.(*p2p.TCPTransport); ok {
		tcpTr.OnPeer = fs.OnPeer
		tcpTr.OnPeerDisconnect = fs.OnPeerDisconnect
	}

	return fs
}

// NewP2PServer alias for NewFileServer.
func NewP2PServer(opts P2PServerOpts) *DefaultFileServer {
	return NewFileServer(opts)
}

// OnPeer is invoked when a remote peer successfully connects.
func (s *DefaultFileServer) OnPeer(p p2p.Peer) error {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()
	addr := p.RemoteAddr().String()
	s.peers[addr] = p
	log.Printf("[%s] Connected to peer: %s\n", s.shortID(), addr)
	return nil
}

// OnPeerDisconnect cleans up dead peers when they disconnect.
func (s *DefaultFileServer) OnPeerDisconnect(p p2p.Peer) {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()
	addr := p.RemoteAddr().String()
	delete(s.peers, addr)

	s.pendingLock.Lock()
	delete(s.pendingStore, addr)
	s.pendingLock.Unlock()

	log.Printf("[%s] Peer disconnected and removed: %s\n", s.shortID(), addr)
}

func (s *DefaultFileServer) getPeer(addr string) (p2p.Peer, bool) {
	s.peerLock.RLock()
	defer s.peerLock.RUnlock()
	p, ok := s.peers[addr]
	return p, ok
}

// Start begins listening, bootstraps to peers, and enters the message processing loop.
func (s *DefaultFileServer) Start() error {
	if err := s.Transport.ListenAndAccept(); err != nil {
		return err
	}
	s.bootstrapNetwork()
	s.loop()
	return nil
}

// Stop shuts down the server.
func (s *DefaultFileServer) Stop() {
	select {
	case <-s.quitch:
	default:
		close(s.quitch)
	}
	s.Transport.Close()
}

// Close implements FileServer.
func (s *DefaultFileServer) Close() {
	s.Stop()
}

func (s *DefaultFileServer) bootstrapNetwork() {
	for _, addr := range s.BootstrapNodes {
		go func(addr string) {
			log.Printf("[%s] Dialing bootstrap node: %s\n", s.shortID(), addr)
			if err := s.Transport.Dial(addr); err != nil {
				log.Printf("[%s] Failed to dial bootstrap node %s: %v\n", s.shortID(), addr, err)
			}
		}(addr)
	}
}

func (s *DefaultFileServer) sendMsg(peer p2p.Peer, msg *Message) error {
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}
	if err := peer.Send([]byte{p2p.IncomingMessage}); err != nil {
		return err
	}
	length := int32(buf.Len())
	if err := binary.Write(peer, binary.BigEndian, length); err != nil {
		return err
	}
	return peer.Send(buf.Bytes())
}

func (s *DefaultFileServer) broadcast(msg *Message) error {
	s.peerLock.RLock()
	peers := make([]p2p.Peer, 0, len(s.peers))
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	s.peerLock.RUnlock()

	for _, peer := range peers {
		if err := s.sendMsg(peer, msg); err != nil {
			log.Printf("[%s] Broadcast error to %s: %v\n", s.shortID(), peer.RemoteAddr(), err)
		}
	}
	return nil
}

// Store writes the file to local storage and broadcasts the encrypted file to all connected peers.
func (s *DefaultFileServer) Store(key string, r io.Reader) error {
	hashedKey := crypto.HashKey(key)

	// Step 1: Write file to local disk under hashedKey
	fileBuffer := new(bytes.Buffer)
	tee := io.TeeReader(r, fileBuffer)
	size, err := s.storage.Write(s.ID, hashedKey, tee)
	if err != nil {
		return fmt.Errorf("store local write error: %w", err)
	}

	// Step 2: Broadcast control message to all connected peers
	s.peerLock.RLock()
	peers := make([]p2p.Peer, 0, len(s.peers))
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	s.peerLock.RUnlock()

	if len(peers) == 0 {
		return nil
	}

	msg := &Message{
		Payload: MessageStoreFile{
			ID:   s.ID,
			Key:  hashedKey,
			Size: size + 16, // plaintext size + 16 bytes IV
		},
	}

	for _, peer := range peers {
		if err := s.sendMsg(peer, msg); err != nil {
			log.Printf("[%s] Error sending MessageStoreFile to %s: %v\n", s.shortID(), peer.RemoteAddr(), err)
		}
	}

	// Step 3: Stream encrypted file bytes to all peers simultaneously
	writers := make([]io.Writer, len(peers))
	for i, peer := range peers {
		if err := peer.Send([]byte{p2p.IncomingStream}); err != nil {
			log.Printf("[%s] Error sending IncomingStream to %s: %v\n", s.shortID(), peer.RemoteAddr(), err)
		}
		writers[i] = peer
	}

	mw := io.MultiWriter(writers...)
	if _, err := crypto.CopyEncrypt(s.EncKey, fileBuffer, mw); err != nil {
		return fmt.Errorf("broadcast encrypt error: %w", err)
	}

	return nil
}

// StoreData is an alias for Store.
func (s *DefaultFileServer) StoreData(key string, r io.Reader) error {
	return s.Store(key, r)
}

// Get retrieves a file by key, serving from local disk if available, or fetching and caching from the network.
func (s *DefaultFileServer) Get(key string) (io.Reader, error) {
	hashedKey := crypto.HashKey(key)

	// Check local storage under raw key and hashed key
	if s.storage.Has(s.ID, key) {
		log.Printf("[%s] Serving file '%s' from local storage\n", s.shortID(), key)
		_, r, err := s.storage.Read(s.ID, key)
		return r, err
	}
	if s.storage.Has(s.ID, hashedKey) {
		log.Printf("[%s] Serving file '%s' (hash: %s) from local storage\n", s.shortID(), key, hashedKey)
		_, r, err := s.storage.Read(s.ID, hashedKey)
		return r, err
	}

	// Register in-flight Get channel
	ch := make(chan struct{}, 1)
	s.inflightLock.Lock()
	s.inflightGets[hashedKey] = ch
	s.inflightLock.Unlock()

	defer func() {
		s.inflightLock.Lock()
		delete(s.inflightGets, hashedKey)
		s.inflightLock.Unlock()
	}()

	// Broadcast MessageGetFile to all peers
	msg := &Message{
		Payload: MessageGetFile{
			ID:  s.ID,
			Key: hashedKey,
		},
	}
	if err := s.broadcast(msg); err != nil {
		return nil, fmt.Errorf("broadcast get request error: %w", err)
	}

	// Channel-based synchronization replacing fragile sleep (Nice-to-Have #1)
	select {
	case <-ch:
		_, r, err := s.storage.Read(s.ID, hashedKey)
		if err != nil {
			return nil, err
		}
		return r, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout: file '%s' not found across network", key)
	case <-s.quitch:
		return nil, errors.New("server stopped")
	}
}

func (s *DefaultFileServer) loop() {
	defer func() {
		log.Printf("[%s] Server loop terminated\n", s.shortID())
	}()

	for {
		select {
		case <-s.quitch:
			return
		case rpc, ok := <-s.Transport.Consume():
			if !ok {
				return
			}
			if rpc.Stream {
				if err := s.handleStream(rpc.From); err != nil {
					log.Printf("[%s] handleStream error from %s: %v\n", s.shortID(), rpc.From, err)
				}
			} else {
				var msg Message
				if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
					log.Printf("[%s] gob decode error from %s: %v\n", s.shortID(), rpc.From, err)
					continue
				}
				if err := s.handleMessage(rpc.From, &msg); err != nil {
					log.Printf("[%s] handleMessage error from %s: %v\n", s.shortID(), rpc.From, err)
				}
			}
		}
	}
}

func (s *DefaultFileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		return s.handleMessageStoreFile(from, v)
	case MessageGetFile:
		return s.handleMessageGetFile(from, v)
	default:
		return fmt.Errorf("unknown message type: %T", v)
	}
}

func (s *DefaultFileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	s.pendingLock.Lock()
	s.pendingStore[from] = msg
	s.pendingLock.Unlock()
	return nil
}

func (s *DefaultFileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	peer, ok := s.getPeer(from)
	if !ok {
		return fmt.Errorf("peer %s not found in peer map", from)
	}

	targetKey := msg.Key
	var targetID string

	// Check if this node has the requested file under local ID or msg.ID
	if s.storage.Has(s.ID, targetKey) {
		targetID = s.ID
	} else if s.storage.Has(msg.ID, targetKey) {
		targetID = msg.ID
	} else {
		// File not stored on this node
		return nil
	}

	fileSize, r, err := s.storage.Read(targetID, targetKey)
	if err != nil {
		return err
	}
	if rc, ok := r.(io.Closer); ok {
		defer rc.Close()
	}

	// 1. Signal IncomingStream
	if err := peer.Send([]byte{p2p.IncomingStream}); err != nil {
		return err
	}

	// 2. Send key length and key header
	keyBytes := []byte(targetKey)
	keyLen := int16(len(keyBytes))
	if err := binary.Write(peer, binary.LittleEndian, keyLen); err != nil {
		return err
	}
	if err := peer.Send(keyBytes); err != nil {
		return err
	}

	// 3. Send encrypted file size (fileSize + 16 for IV)
	encSize := fileSize + 16
	if err := binary.Write(peer, binary.LittleEndian, encSize); err != nil {
		return err
	}

	// 4. Stream encrypted file bytes (Fixes encryption bug in handleMessageGetFile)
	_, err = crypto.CopyEncrypt(s.EncKey, r, peer)
	return err
}

func (s *DefaultFileServer) handleStream(from string) error {
	peer, ok := s.getPeer(from)
	if !ok {
		return fmt.Errorf("peer %s not found for incoming stream", from)
	}

	s.pendingLock.Lock()
	pending, isStore := s.pendingStore[from]
	if isStore {
		delete(s.pendingStore, from)
	}
	s.pendingLock.Unlock()

	if isStore {
		defer peer.CloseStream()
		// Store under local s.ID so this node's CAS store owns the file
		n, err := s.storage.WriteDecrypt(s.EncKey, s.ID, pending.Key, io.LimitReader(peer, pending.Size))
		if err != nil {
			return fmt.Errorf("failed to write decrypted stream to disk: %w", err)
		}
		log.Printf("[%s] Stored %d bytes from %s (key: %s)\n", s.shortID(), n, from, pending.Key)
		return nil
	}

	// Otherwise, this stream is a response to an in-flight Get request
	return s.handleGetStream(peer)
}

func (s *DefaultFileServer) handleGetStream(peer p2p.Peer) error {
	defer peer.CloseStream()

	var keyLen int16
	if err := binary.Read(peer, binary.LittleEndian, &keyLen); err != nil {
		return err
	}
	keyBuf := make([]byte, keyLen)
	if _, err := io.ReadFull(peer, keyBuf); err != nil {
		return err
	}
	hashedKey := string(keyBuf)

	var encSize int64
	if err := binary.Read(peer, binary.LittleEndian, &encSize); err != nil {
		return err
	}

	s.inflightLock.Lock()
	ch, waiting := s.inflightGets[hashedKey]
	s.inflightLock.Unlock()

	if !waiting {
		// Discard redundant or late stream
		_, err := io.CopyN(io.Discard, peer, encSize)
		return err
	}

	// Decrypt and write to local disk under s.ID
	n, err := s.storage.WriteDecrypt(s.EncKey, s.ID, hashedKey, io.LimitReader(peer, encSize))
	if err != nil {
		return fmt.Errorf("failed to write decrypted get stream: %w", err)
	}
	log.Printf("[%s] Received Get response for key '%s' (%d bytes)\n", s.shortID(), hashedKey, n)

	select {
	case ch <- struct{}{}:
	default:
	}

	return nil
}

func (s *DefaultFileServer) shortID() string {
	if len(s.ID) > 8 {
		return s.ID[:8]
	}
	return s.ID
}
