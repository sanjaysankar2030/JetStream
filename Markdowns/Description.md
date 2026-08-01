# Distributed File System in Go — Complete Project Reference
---

## Table of Contents

1. [What This Project Is](#1-what-this-project-is)
2. [Tech Stack](#2-tech-stack)
3. [Project Structure](#3-project-structure)
4. [The Two Big Parts](#4-the-two-big-parts)
5. [Core Concepts You Must Understand](#5-core-concepts-you-must-understand)
6. [Content-Addressed Storage (CAS) — How It Works](#6-content-addressed-storage-cas--how-it-works)
7. [Encryption — How It Works](#7-encryption--how-it-works)
8. [P2P Transport Layer](#8-p2p-transport-layer)
9. [FileServer — The Node](#9-fileserver--the-node)
10. [Message Protocol](#10-message-protocol)
11. [Store/Get Flow — End to End](#11-storeget-flow--end-to-end)
12. [Node Identity & Namespacing](#12-node-identity--namespacing)
13. [Bootstrap & Peer Discovery](#13-bootstrap--peer-discovery)
14. [Validation & Error Handling](#14-validation--error-handling)
15. [Configuration](#15-configuration)
16. [Current Scope vs. What's Missing](#16-current-scope-vs-whats-missing)
17. [Common Mistakes to Avoid](#17-common-mistakes-to-avoid)
18. [Nice to Haves — Improvements & Future Work](#18-nice-to-haves--improvements--future-work)

---

## 1. What This Project Is

A **peer-to-peer distributed file system** built entirely in Go. Every node is simultaneously a server and a client — there is no central coordinator, and no external dependencies.

Think of it as a stepping stone between IPFS (simple DHT + CAS) and real distributed filesystems (GFS/HDFS): any node can store a file across multiple peers with automatic replication detection, and any node can retrieve it from the network while respecting chunk locality.

**Core Goals (In Priority Order):**
1. **Networking layer** (supporting role): Persistent TCP connections between peers with heartbeat-based failure detection and exponential backoff reconnection
2. **Metadata service**: Track which chunks belong to which files and which nodes hold which chunks (in-memory maps initially; single leader)
3. **File chunking**: Split large files into fixed-size chunks (4MB default), each content-addressed via SHA-256
4. **Replication**: Asynchronously replicate chunks to N replica nodes, tracking replica locations in metadata
5. **Smart retrieval**: When requesting a file, fetch chunks from known replica locations instead of broadcasting to all peers
6. **Encryption**: All data in transit encrypted with AES-256-CTR so no peer sees plaintext bytes over the wire

**What this is NOT (skip for now):**
- It is **not** a consensus system (Raft/BFT). Metadata is currently centralized (leader only). Consensus comes after we nail replication.
- It is **not** blockchain or DHT-first. Networking is a supporting layer; focus is DFS logic.
- It is **not** production-grade. No erasure coding, no garbage collection, no quota management. We're learning DFS architecture.
- It is **not** GFS/HDFS yet. Those have chunk splitting at 64+MB, block reports, formal replication protocol. We have simpler equivalents now.

---

## 2. Tech Stack

| Layer | Technology |
|---|---|
| Language | Go (1.21+) |
| Transport | Raw TCP (`net` package) |
| Serialisation | `encoding/gob` |
| Encryption | AES-256-CTR (`crypto/aes`, `crypto/cipher`) |
| Storage Layout | Content-Addressed (SHA-1 hash → nested directories) |
| Key Hashing | MD5 (for network key lookup — see improvement notes) |
| ID Generation | `crypto/rand` (32-byte hex string) |
| Concurrency | `sync.Mutex`, goroutines, channels |
| Build | `make` (`Makefile`) |

---

## 3. Project Structure

```
distributedfilesystemgo/
│
├── main.go                     ← Bootstrap: wires up 3 nodes, runs demo Store/Get
│
├── server.go                   ← FileServer: core node logic (Store, Get, file reconstruction)
├── store.go                    ← CAS storage layer: disk read/write with SHA-256 path layout
├── crypto.go                   ← AES-256-CTR encrypt/decrypt + key generation
│
├── peer/
│   ├── pool.go                 ← PeerPool: persistent TCP connection, heartbeat, backoff reconnect
│   ├── message.go              ← Protocol messages (PING, GET_CHUNK, PUT_CHUNK, REPLICATE, DELETE)
│   └── protocol.go             ← Message serialization/deserialization
│
├── metadata/
│   ├── service.go              ← MetadataService: file→chunks mapping, chunk→nodes tracking
│   ├── chunking.go             ← File splitting logic (fixed 4MB chunks), chunk ID generation
│   └── index.go                ← In-memory metadata maps (files, chunks, replicas)
│
├── p2p/
│   ├── transport.go            ← Transport + Peer interfaces (legacy, being refactored)
│   ├── tcp_transport.go        ← TCP implementation of Transport
│   ├── tcp_peer.go             ← TCP implementation of Peer
│   ├── encoding.go             ← Decoder interface + GOBDecoder
│   ├── handshake.go            ← Handshake function type
│   └── message.go              ← Legacy RPC struct (will consolidate with peer/message.go)
│
├── crypto_test.go              ← Unit tests: encryption round-trip
├── store_test.go               ← Unit tests: CAS store read/write/delete
├── metadata_test.go            ← Unit tests: chunking, file reconstruction
├── peer_pool_test.go           ← Unit tests: connection management, heartbeat
│
├── go.mod
├── go.sum
├── Makefile
└── .gitignore
```

---

## 4. The Three-Layer Architecture

### Layer 1 — Networking (Supporting Role)
Handles peer connectivity and message transport. **Not** the focus of learning; should be as simple as possible.

**Key Components:**
- `PeerPool`: Maintains persistent TCP connection to each peer
  - Automatic reconnection with exponential backoff
  - Heartbeat (PING) every 5s to detect failures
  - Marks peer dead after 3 consecutive missed heartbeats
- Message protocol: `PING`, `GET_CHUNK`, `PUT_CHUNK`, `REPLICATE`, `DELETE`
- Request/response correlation via message IDs

**Responsibility:** Get bytes reliably from point A to point B. Nothing more.

### Layer 2 — Distributed Filesystem (Core Learning)
Where the real DFS problems live.

**Key Components:**
- `MetadataService`: Single leader (for now)
  - Maps `filename → [chunkID1, chunkID2, ...]`
  - Maps `chunkID → [nodeA, nodeB, nodeC]` (replica locations)
  - Persisted to disk (write-ahead log, snapshots come later)
  
- `ChunkingService`: Split files into 4MB chunks
  - Compute SHA-256 hash of each chunk → chunk ID
  - No padding; last chunk can be smaller
  - Reconstruct file by concatenating chunks in order

- `ReplicationManager`: Async replication logic
  - When storing chunk, spawn goroutines to replicate to N peers
  - Track which replicas confirmed receipt
  - If replication fails, log and move on (auto-repair comes later)

**Responsibility:** Answer "what chunks form this file?" and "which nodes have this chunk?"

### Layer 3 — Local Storage (CAS Layer)
The `Store` struct manages disk reads/writes on a single node. Independent of networking.

- **Content-addressed path derivation** — SHA-256 hash of chunk ID (not filename), split into nested directories
- **Per-node namespacing** — all chunks stored under `root/<nodeID>/`, so multiple nodes on same machine don't collide
- **Pluggable path transform** — allows swapping layout strategy

**Responsibility:** Durable, efficient storage of chunk bytes on local disk.

---

## 5. Core Concepts You Must Understand

### Content-Addressed Storage (CAS)
A storage scheme where the file's **location on disk is determined by a hash of its key**, not its original name. The key design advantage: identical content always lands in the same path, and collisions are cryptographically impossible.

### Peer-to-Peer (P2P) Topology
Every node is equal. There is no master. Any node can initiate a `Store` or `Get`. Replication happens via broadcast — when you store a file, it is pushed to **all** currently connected peers.

### Transport Abstraction
The `p2p.Transport` interface decouples the network protocol from the application. Right now only TCP is implemented, but the server code doesn't know or care — it calls `transport.Consume()` and `peer.Send()`.

### Gob Encoding
Go's `encoding/gob` is used to serialise `Message` structs over the wire. Both message types (`MessageStoreFile`, `MessageGetFile`) are registered with `gob.Register` at init time.

### In-band Stream Signalling
The protocol uses a single magic byte prefix to distinguish between a gob-encoded message and a raw binary stream:
- `p2p.IncomingMessage` — the next bytes are a gob-encoded `Message`
- `p2p.IncomingStream` — the next bytes are raw (encrypted) file data

---

## 6. Content-Addressed Storage (CAS) — How It Works

### Path Derivation (`CASPathTransformFunc`)

Given a key like `"myfile.txt"`, the CAS function:

1. Computes SHA-1 of the key: `a94a8fe5ccb19ba61c4c0873d391e987982fbbd3`
2. Splits the hex string into 5-character chunks: `a94a8`, `fe5cc`, `b19ba`, `61c4c`, `0873d`, `391e9`, `87982`, `fbbd3`
3. Joins with `/` to form the path: `a94a8/fe5cc/b19ba/61c4c/0873d/391e9/87982`
4. The filename is the full hash: `a94a8fe5ccb19ba61c4c0873d391e987982fbbd3`

Full path on disk:
```
ggnetwork/<nodeID>/a94a8/fe5cc/b19ba/61c4c/0873d/391e9/87982/a94a8fe5ccb19ba61c4c0873d391e987982fbbd3
```

### Why This Layout?
- Avoids having thousands of files in a single directory (which degrades filesystem performance)
- Same trick used internally by Git's object store
- Deleting a file means `os.RemoveAll` on just the first path segment — clean and atomic

### Store Interface

| Method | Signature | What It Does |
|---|---|---|
| `Has` | `(id, key string) bool` | Checks if file exists on disk via `os.Stat` |
| `Write` | `(id, key string, r io.Reader) (int64, error)` | Writes raw bytes from reader to disk |
| `WriteDecrypt` | `(encKey []byte, id, key string, r io.Reader) (int64, error)` | Decrypts AES-CTR stream while writing to disk |
| `Read` | `(id, key string) (int64, io.Reader, error)` | Opens file and returns size + reader |
| `Delete` | `(id, key string) error` | Removes the entire first path segment tree |
| `Clear` | `() error` | Removes the entire root directory (used in tests) |

### Default vs CAS Transform
```go
// DefaultPathTransformFunc — flat layout, for dev/testing
PathKey{ PathName: key, Filename: key }

// CASPathTransformFunc — production layout
PathKey{ PathName: "a94a8/fe5cc/...", Filename: "a94a8fe5ccb19ba61c..." }
```

---

## 7. Encryption — How It Works

All file data transferred between peers is encrypted with **AES-256 in Counter (CTR) mode**.

### Encrypt (`copyEncrypt`)

```
1. Generate a random 32-byte AES key (or use the server's EncKey)
2. Create AES cipher block from the key
3. Generate a random 16-byte IV from crypto/rand
4. Prepend IV to the output stream (so the receiver can extract it)
5. Create a CTR stream cipher: cipher.NewCTR(block, iv)
6. XOR all plaintext bytes through the stream cipher → write to dst
```

### Decrypt (`copyDecrypt`)

```
1. Read the first block.BlockSize() (16) bytes from src — this is the IV
2. Create AES cipher block from the same key
3. Create CTR stream with the extracted IV
4. XOR all remaining bytes back to plaintext → write to dst
```

### Why CTR Mode?
CTR mode turns a block cipher into a stream cipher — it never needs padding, can be streamed byte-by-byte without buffering the full file, and encryption/decryption use the same operation (XOR), making the code symmetric.

### Key Generation

```go
func newEncryptionKey() []byte {
    keyBuf := make([]byte, 32)
    io.ReadFull(rand.Reader, keyBuf)
    return keyBuf   // 256-bit random AES key
}
```

### Known Weakness: MD5 Key Hashing
The `hashKey()` function (used to derive the on-disk key from the network key) uses MD5:
```go
func hashKey(key string) string {
    hash := md5.Sum([]byte(key))
    return hex.EncodeToString(hash[:])
}
```
MD5 is cryptographically broken and collision-prone. This should be replaced with SHA-256 (see Nice to Haves).

---

## 8. P2P Transport Layer

The `p2p` package defines a clean abstraction over the network protocol.

### Interfaces

```go
// Peer represents a connected remote node
type Peer interface {
    net.Conn
    Send([]byte) error
    CloseStream()
}

// Transport manages connections — listen, dial, and consume messages
type Transport interface {
    Addr() string
    Dial(string) error
    ListenAndAccept() error
    Consume() <-chan RPC
    Close() error
}
```

### RPC Message

```go
type RPC struct {
    From    string   // remote address of the sender
    Payload []byte   // raw gob-encoded Message bytes
    Stream  bool     // true if this RPC is a raw binary stream
}
```

### TCP Transport Flow

```
Dial(addr)
    └── net.Dial("tcp", addr)
        └── TCPPeer created (outbound=true)
            └── OnPeer callback fires → peer added to FileServer.peers map

ListenAndAccept()
    └── net.Listen("tcp", listenAddr)
        └── for each incoming conn:
            TCPPeer created (outbound=false)
            └── handleConn() goroutine → reads RPC loop
```

### Stream vs Message Signalling

The first byte of every transmission signals its type:

| Byte Value | Constant | Meaning |
|---|---|---|
| `0x1` | `IncomingMessage` | Next bytes are a gob-encoded `Message` struct |
| `0x2` | `IncomingStream` | Next bytes are raw binary (encrypted file data) |

The `CloseStream()` call on a peer signals end-of-stream, allowing the transport's read loop to resume consuming the next message.

---

## 9. FileServer — The Node

`FileServer` is the central type. It wires together storage, encryption, and transport.

### Configuration (`FileServerOpts`)

| Field | Type | Purpose |
|---|---|---|
| `ID` | `string` | Unique node identity (auto-generated if empty) |
| `EncKey` | `[]byte` | 32-byte AES key used to encrypt/decrypt all transfers |
| `StorageRoot` | `string` | Local root directory for this node's files |
| `PathTransformFunc` | `PathTransformFunc` | CAS or Default layout |
| `Transport` | `p2p.Transport` | TCP transport instance |
| `BootstrapNodes` | `[]string` | Addresses to dial on startup |

### Key Methods

| Method | Signature | What It Does |
|---|---|---|
| `Start` | `() error` | Begins listening, bootstraps network, enters message loop |
| `Stop` | `()` | Closes the quit channel, triggers graceful loop exit |
| `Store` | `(key string, r io.Reader) error` | Writes file locally + broadcasts to all peers (encrypted) |
| `Get` | `(key string) (io.Reader, error)` | Returns file from local disk or fetches from network |
| `OnPeer` | `(p p2p.Peer) error` | Callback invoked when a new peer connects; adds to peer map |
| `broadcast` | `(msg *Message) error` | Gob-encodes and sends a Message to all peers |
| `loop` | `()` | Main event loop — consumes RPCs and dispatches to handlers |
| `handleMessage` | `(from string, msg *Message) error` | Routes to Store or Get handler based on message type |
| `bootstrapNetwork` | `() error` | Dials all BootstrapNodes concurrently on startup |

### Peer Map Locking

```go
type FileServer struct {
    peerLock sync.Mutex
    peers    map[string]p2p.Peer
    ...
}
```

The `peerLock` mutex guards the peer map against concurrent reads/writes from multiple goroutines (each incoming connection runs in its own goroutine).

---

## 10. Message Protocol

All control messages use `encoding/gob`. Two message types are defined:

### `MessageStoreFile`
Sent by a node that just stored a file locally, before streaming the encrypted bytes.

```go
type MessageStoreFile struct {
    ID   string  // sender's node ID (used for storage namespacing)
    Key  string  // MD5 hash of the file key
    Size int64   // encrypted file size in bytes (plaintext size + 16 for IV)
}
```

### `MessageGetFile`
Sent by a node that doesn't have a file locally and is asking peers for it.

```go
type MessageGetFile struct {
    ID  string  // requesting node's ID
    Key string  // MD5 hash of the file key
}
```

### Message Envelope

```go
type Message struct {
    Payload any   // either MessageStoreFile or MessageGetFile
}
```

`gob.Register` is called for both concrete types at `init()` time — required because `Payload` is typed as `any`.

---

## 11. Store/Get Flow — End to End

### Store Flow

```
s.Store("myphoto.jpg", reader)

    ↓ Step 1: Write locally
    io.TeeReader(r, fileBuffer)  ← simultaneously fills fileBuffer while writing
    s.store.Write(s.ID, key, tee)
    → disk: ggnetwork/<nodeID>/<sha1-path>/

    ↓ Step 2: Broadcast control message
    msg = Message{ Payload: MessageStoreFile{ ID, Key: hashKey(key), Size: size+16 } }
    for each peer:
        peer.Send([]byte{IncomingMessage})
        peer.Send(gob.Encode(msg))

    ↓ Step 3: Stream encrypted file bytes to all peers
    mw = io.MultiWriter(all peers...)
    mw.Write([]byte{IncomingStream})       ← signal: raw stream incoming
    copyEncrypt(s.EncKey, fileBuffer, mw)  ← AES-CTR encrypt → all peers simultaneously

↓ Each receiving peer (handleMessageStoreFile):
    store.Write(msg.ID, msg.Key, io.LimitReader(peer, msg.Size))
    peer.CloseStream()
```

### Get Flow

```
s.Get("myphoto.jpg")

    ↓ Step 1: Check local disk
    if s.store.Has(s.ID, key) → return s.store.Read(s.ID, key)  ✓

    ↓ Step 2: Broadcast request to all peers
    msg = Message{ Payload: MessageGetFile{ ID: s.ID, Key: hashKey(key) } }
    broadcast(msg)

    ↓ Step 3: Wait (currently a hardcoded 500ms sleep)
    time.Sleep(500ms)   ← ⚠️ fragile — see Nice to Haves

    ↓ Step 4: Read response from each peer
    for each peer:
        binary.Read(peer, LittleEndian, &fileSize)  ← peer sends size first
        store.WriteDecrypt(s.EncKey, s.ID, key, io.LimitReader(peer, fileSize))
        peer.CloseStream()

    ↓ Step 5: Read from local disk (now cached)
    return s.store.Read(s.ID, key)

↓ Each responding peer (handleMessageGetFile):
    fileSize, r, _ = store.Read(msg.ID, msg.Key)
    peer.Send([]byte{IncomingStream})
    binary.Write(peer, LittleEndian, fileSize)   ← send size header
    io.Copy(peer, r)                              ← send plaintext file bytes
```

> **Note:** The responding peer sends the file **unencrypted** (`io.Copy` not `copyEncrypt`) in `handleMessageGetFile`. This is a bug — the receiver calls `WriteDecrypt` but the sender never encrypted the data. See Nice to Haves.

---

## 12. Node Identity & Namespacing

Each `FileServer` has a unique `ID` — a 64-character hex string derived from 32 random bytes:

```go
func generateID() string {
    buf := make([]byte, 32)
    io.ReadFull(rand.Reader, buf)
    return hex.EncodeToString(buf)
}
```

This ID is used to namespace files on disk:
```
<StorageRoot>/<ID>/<cas-path>/<filename>
```

This means multiple nodes can run on the same machine with the same `StorageRoot` without colliding — each has its own subtree. It also means when a node rebroadcasts a received file to a new peer, the file is stored under the **original sender's ID**, preserving provenance.

---

## 13. Bootstrap & Peer Discovery

### How Nodes Find Each Other

Peer discovery is **static** — you provide a list of bootstrap addresses in `FileServerOpts.BootstrapNodes`. On `Start()`, the server dials them all concurrently:

```go
func (s *FileServer) bootstrapNetwork() error {
    for _, addr := range s.BootstrapNodes {
        go func(addr string) {
            s.Transport.Dial(addr)
        }(addr)
    }
    return nil
}
```

Once a dial succeeds, the `TCPTransport`'s `OnPeer` callback fires, which calls `FileServer.OnPeer()`, which adds the peer to the map.

### Demo Setup (main.go)

```
Node 1 — listens on :3000, no bootstrap nodes
Node 2 — listens on :7000, bootstraps to :3000
Node 3 — listens on :5000, bootstraps to :3000 and :7000

Demo:
  s3.Store("myPrivateKey", bytes.NewReader([]byte("my super secret")))
  time.Sleep(500ms)
  r, _ := s1.Get("myPrivateKey")
  io.Copy(os.Stdout, r)
```

---

## 14. Validation & Error Handling

The project is lean on explicit validation. Here is what is and isn't checked:

### What Is Validated

| Scenario | Handled How |
|---|---|
| File not found locally on Get | Broadcasts to peers; logs if no peer has it |
| Peer not in map on message handle | Returns `fmt.Errorf("peer %s not in map", from)` |
| `gob.Decode` failure in loop | Logs error and continues; does not crash |
| `handleMessage` error | Logs error and continues |
| Bootstrap dial failure | Logs error but doesn't stop the server |
| `os.Stat` failure on `Has` | Returns `false` (non-existence assumed) |

### What Is NOT Validated (gaps)

| Gap | Risk |
|---|---|
| No check if `EncKey` is nil or wrong length | `aes.NewCipher` will panic with a misleading error |
| No check if file exists before Store (no dedup) | Same file stored multiple times, wastes disk |
| Dead peers never removed from peer map | `Send` calls on a closed peer return errors silently |
| No file integrity check on received data | A corrupt or truncated file is written to disk silently |
| `time.Sleep(500ms)` in Get | If network is slow, response arrives after sleep and is missed |

---

## 15. Configuration

Nodes are configured entirely in Go code via `FileServerOpts`. There is no config file.

### Example (from main.go)

```go
s1 := makeServer(":3000", "")           // no bootstrap
s2 := makeServer(":7000", ":3000")      // bootstraps to s1
s3 := makeServer(":5000", ":3000", ":7000") // bootstraps to s1 and s2

func makeServer(listenAddr string, nodes ...string) *FileServer {
    tcpOpts := p2p.TCPTransportOpts{
        ListenAddr:    listenAddr,
        HandshakeFunc: p2p.NOPHandshakeFunc,
        Decoder:       p2p.DefaultDecoder{},
    }

    return NewFileServer(FileServerOpts{
        EncKey:            newEncryptionKey(),
        StorageRoot:       listenAddr + "_network",
        PathTransformFunc: CASPathTransformFunc,
        Transport:         p2p.NewTCPTransport(tcpOpts),
        BootstrapNodes:    nodes,
    })
}
```

### Configuration Fields Reference

| Field | Default | Notes |
|---|---|---|
| `ID` | Auto-generated (64-char hex) | Set explicitly if you want stable node identity across restarts |
| `EncKey` | Must be provided | 32 bytes (256-bit). Use `newEncryptionKey()` for a fresh random key |
| `StorageRoot` | Must be provided | Separate roots per node if running locally (e.g. use listen address) |
| `PathTransformFunc` | `DefaultPathTransformFunc` | Use `CASPathTransformFunc` for production |
| `BootstrapNodes` | `[]string{}` | Empty = isolated node; provide peer addresses to join the network |

---

## 16. Immediate Priorities: What To Build Next

### Phase 1: Build-Measure-Optimize (Start Here)

Add **two features**, identify bottlenecks, fix them, repeat. Don't gold-plate.

#### Networking Feature #1: Connection Management
**What to build:**
- `PeerPool`: persistent TCP connection to each peer (one pool per peer)
- Heartbeat: PING every 5s
- Exponential backoff reconnection: 100ms → 200ms → 400ms → ... → cap at 30s
- Dead peer detection: mark dead after 3 consecutive missed heartbeats
- Remove dead peer from active set

**Why this first:** All other features depend on stable peer connectivity. No point implementing replication if connections drop randomly.

**Test it:** Kill a peer, watch it get marked dead after 15s. Reconnect it, watch it rejoin.

**Estimated work:** 50-100 lines of Go. 2-3 hours.

#### DFS Feature #1: Metadata Service + File Chunking
**What to build:**
- `MetadataService`: two in-memory maps:
  - `files[filename] = [chunkID1, chunkID2, ...]`
  - `chunks[chunkID] = [nodeA, nodeB, nodeC]`
- `ChunkingService`: split file into 4MB chunks
  - Hash each chunk with SHA-256 → chunk ID
  - Store chunks in CAS individually
  - Reconstruct file by concatenating chunks in order
- `ReplicationManager`: async replicate chunks
  - When storing chunk, spawn goroutines to replicate to N peers
  - Track which nodes confirmed receipt
  - Log failures (don't auto-retry yet)

**Why this second:** Core DFS problem. Once this works, you know "where is this chunk?" without broadcasting. This is the fundamental insight that separates real DFS from dumb broadcasting.

**Test it:** Store 10MB file → split into 3 chunks → replicate to 2 peers → retrieve from 3rd peer → verify byte-for-byte integrity.

**Estimated work:** 200-300 lines. 6-8 hours.

---

### Phase 2: Fix Bottlenecks (After Phase 1)
Once Phase 1 works, **immediately run it under load and see what breaks:**

- [ ] Dead peer replicas aren't re-replicated (auto-repair)
- [ ] Checksum mismatches on receipt aren't detected (add SHA-256 verification)
- [ ] Replication confirmations aren't tracked (do we know if all N replicas succeeded?)
- [ ] Metadata isn't persistent (restart node → lose all file knowledge)

Fix these one by one, measuring impact each time.

---

### Phase 3: Scalability (After Phase 2)
Only after Phase 1 + 2 works reliably:

- [ ] Consistent hashing (smart peer selection instead of random)
- [ ] Topology awareness (don't replicate to same rack)
- [ ] Metadata persistence (write-ahead log + snapshots)
- [ ] Leader election (when leader dies, promote follower)

---

### Explicitly NOT Doing (Yet)
- ❌ **Blockchain**: Zero use case for DFS. Skip entirely.
- ❌ **DHT**: Premature. Broadcast works fine at 3-10 nodes. Add after you hit bottlenecks at 100+ nodes.
- ❌ **Raft consensus**: Only needed if metadata leader crashes. Single leader sufficient for learning.
- ❌ **HTTP/gRPC API**: Not needed for core DFS learning. Add after architecture is solid.
- ❌ **Erasure coding**: Complex. Replication teaches the same concepts and is simpler.
- ❌ **MapReduce**: Comes after DFS itself is production-ready.

---

## 17. Common Mistakes to Avoid

| Mistake | Why It's a Problem | Fix |
|---|---|---|
| Providing `EncKey` of wrong length | `aes.NewCipher` requires exactly 16, 24, or 32 bytes. Wrong length = panic | Always use `newEncryptionKey()` which generates exactly 32 bytes |
| Same `StorageRoot` for multiple nodes on same machine | Node files collide because they share the same `root/` directory | Use distinct roots per node (e.g. `":3000_network"`, `":7000_network"`) |
| Using `DefaultPathTransformFunc` in production | Files stored flat by raw key name — no directory sharding, filesystem degrades at scale | Always use `CASPathTransformFunc` |
| Replicating to ALL peers instead of N | Broadcast replication doesn't scale beyond 10 nodes. Every Store = O(n) messages | Implement replication factor control early. Track per-chunk replica locations. |
| Broadcasting every Get to all peers | Query traffic grows O(n). At 100 nodes, every Get = 100 messages | Use metadata service to know which nodes have the chunk. Query 1-2, not all. |
| Not detecting dead peers | Dead peer connections remain in map forever. All sends to dead peer fail silently. | Implement heartbeat: PING every 5s, mark dead after 3 missed heartbeats. |
| Buffering entire file in memory before replication | A 2GB video file = 2GB heap. Node crashes or OOMs. | Stream chunks individually (4MB chunks). Replicate as you go. |
| Ignoring metadata persistence | Node restarts → lose all file knowledge. Must re-broadcast everything. | Add write-ahead log. Persist metadata to disk before returning ACK. |
| Single metadata leader with no backup | Leader crashes → entire cluster is down (can't write new files). | Implement follower promotion or Raft after Phase 2 is stable. |

---

## 18. The Build-Measure-Optimize Loop

This is the most important pattern to understand:

1. **Build** Phase 1 (Connection Pool + Metadata Service)
2. **Measure** under load: throughput, latency, CPU, memory
3. **Identify** the bottleneck (network? metadata? storage? memory?)
4. **Optimize** that one thing
5. **Measure** again
6. Repeat

Example bottleneck trajectory:
```
Phase 1 → Bottleneck: Dead peers not detected
        → Fix: Heartbeat + exponential backoff
        
Phase 2 → Bottleneck: Metadata not persistent (crashes lose data)
        → Fix: WAL + snapshots
        
Phase 3 → Bottleneck: Metadata leader is SPOF
        → Fix: Leader election (Raft) or follower promotion
        
Phase 4 → Bottleneck: Replication to all nodes doesn't scale
        → Fix: Consistent hashing + selective replication
        
Phase 5 → Bottleneck: Network topology not considered
        → Fix: Rack-aware placement
```

Each phase teaches a different DFS lesson. Rushing to Raft or DHT before Phase 1 is stable = wasted effort.

---

## 19. Future Work (After Phase 2)

If you continue building beyond Phase 2, this is the natural progression:

- **Phase 3**: Metadata persistence + leader election (single-leader or Raft)
- **Phase 4**: Automatic replica repair (detect missing replicas, re-replicate)
- **Phase 5**: Consistent hashing + topology awareness
- **Phase 6**: Snapshot metadata, log compaction
- **Phase 7**: HTTP/gRPC API for external clients
- **Phase 8**: Eventually: MapReduce, data locality, distributed computing

But stop after Phase 1. Don't skip ahead. Each phase's bottlenecks naturally motivate the next phase's design.

---

## 17. Common Mistakes to Avoid

| Mistake | Why It's a Problem | Fix |
|---|---|---|
| Providing `EncKey` of wrong length | `aes.NewCipher` requires exactly 16, 24, or 32 bytes. Wrong length = panic | Always use `newEncryptionKey()` which generates exactly 32 bytes |
| Same `StorageRoot` for multiple nodes on same machine | Node files collide because they share the same `root/` directory | Use distinct roots per node (e.g. `":3000_network"`, `":7000_network"`) |
| Using `DefaultPathTransformFunc` in production | Files stored flat by raw key name — no directory sharding, filesystem degrades at scale | Always use `CASPathTransformFunc` |
| Relying on the 500ms Sleep in Get | If the network is slow or the peer is under load, the response arrives after sleep and is silently ignored | Replace with channel-based synchronisation |
| Not calling `gob.Register` for new message types | Gob decoding of `any` typed fields requires all concrete types to be registered at init time | Add `gob.Register(YourNewType{})` in the `init()` function |
| Forgetting `peer.CloseStream()` after reading | The transport's read loop stays blocked waiting for more stream bytes — the node hangs | Always call `CloseStream()` after consuming a stream |
| Re-using the same `EncKey` across all nodes | If one node's key leaks, all stored data is compromised | Consider per-session or per-peer key exchange (see Nice to Haves) |
| Checking `Has` without the correct node ID | `Has(wrongID, key)` always returns false even if the file exists on disk under a different ID | Use `s.ID` (the local node's ID) for local lookups |
| Ignoring the `handleMessageGetFile` encryption bug | Receiver calls `WriteDecrypt` but sender never encrypts — file written to disk is garbled | Sender must call `copyEncrypt` before streaming, or receiver must call plain `Write` |
| Running without bootstrap nodes | Node starts but is isolated — Store/Get only works locally, no network effect | Always provide at least one known peer in `BootstrapNodes` |

---
