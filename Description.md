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

A **peer-to-peer distributed file system** built entirely in Go. Every node in the network is simultaneously a server and a client — there is no central master, no coordinator, and no external dependency.

Think of it as a simplified version of IPFS or an early BitTorrent storage layer: any node can store a file, and any other node can retrieve it — even if that node never saw the file before.

**Core Goals:**
- Allow any node to `Store` a file and have it automatically broadcast to all connected peers
- Allow any node to `Get` a file — serving it from local disk if present, or fetching it from the network if not
- Store files in a content-addressed, collision-proof layout on disk
- Encrypt all data in transit using AES-256-CTR so no peer sees plaintext bytes over the wire

**What this is NOT:**
- It is **not** Google File System (GFS). GFS has a single master server, chunk splitting at 64MB, a metadata namespace, and a formal replication protocol. None of that exists here.
- It is **not** HDFS. There are no NameNodes, DataNodes, or block reports.
- It is a flat, serverless, educational P2P storage system with broadcast-based replication.

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
├── main.go              ← Bootstrap: wires up 3 nodes, runs demo Store/Get
│
├── server.go            ← FileServer: the core node logic (Store, Get, broadcast)
├── store.go             ← CAS storage layer: disk read/write with SHA-1 path layout
├── crypto.go            ← AES-256-CTR encrypt/decrypt helpers + ID/key generation
│
├── crypto_test.go       ← Unit tests for encryption round-trip
├── store_test.go        ← Unit tests for CAS store read/write/delete
│
├── p2p/
│   ├── transport.go     ← Transport + Peer interfaces
│   ├── tcp_transport.go ← TCP implementation of Transport
│   ├── tcp_peer.go      ← TCP implementation of Peer (wraps net.Conn)
│   ├── encoding.go      ← Decoder interface + GOBDecoder implementation
│   ├── handshake.go     ← Handshake function type
│   └── message.go       ← RPC struct (From string, Payload []byte)
│
├── go.mod
├── go.sum
├── Makefile
└── .gitignore
```

---

## 4. The Two Big Parts

### Part 1 — Local Storage (CAS Layer)
The `Store` struct manages how files are written to and read from disk on a single node. It is completely independent of the network — you can use it standalone. It implements:

- **Content-addressed path derivation** — SHA-1 hash of the key, split into nested 5-character directories
- **Per-node namespacing** — all files are stored under `root/<nodeID>/`, so multiple nodes on the same machine don't collide
- **Pluggable path transform** — `PathTransformFunc` lets you swap the layout strategy; `CASPathTransformFunc` is the production default

### Part 2 — Network Layer (P2P FileServer)
The `FileServer` connects nodes together. It:

1. Listens for incoming TCP connections via the `Transport` interface
2. Maintains a live map of connected `Peer`s
3. When a file is stored locally, it serialises a `MessageStoreFile` and broadcasts it to all peers, then streams the encrypted file bytes
4. When a file is requested that isn't local, it broadcasts a `MessageGetFile` to all peers; the first peer that has it streams the file back

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

## 16. Current Scope vs. What's Missing

### What Is Implemented ✅

| Feature | Status |
|---|---|
| TCP P2P transport | ✅ |
| Content-addressed local storage (SHA-1) | ✅ |
| AES-256-CTR encryption on Store broadcast | ✅ |
| Broadcast Store to all connected peers | ✅ |
| Network Get with local caching | ✅ |
| Per-node ID and namespaced storage | ✅ |
| Static bootstrap peer discovery | ✅ |
| Pluggable PathTransformFunc | ✅ |
| Pluggable Transport interface | ✅ |
| Unit tests for CAS store and crypto | ✅ |

### What Is NOT Implemented ❌

| Feature | Impact |
|---|---|
| Replication factor control | Files always replicated to ALL peers — unscalable |
| File metadata / index | Cannot list what files exist in the network |
| Node failure detection / heartbeats | Dead peers stay in map forever |
| File chunking for large files | Full file buffered in memory before broadcast |
| DHT-based peer routing | Every Get/Store broadcasts to all peers |
| File integrity verification (checksum) | Corrupt transfers written silently |
| Proper sync on Get (replaces Sleep) | Race condition if network is slow |
| Encryption on handleMessageGetFile | Bug: responding peer sends plaintext |
| HTTP or CLI interface | No user-facing API — demo only in main.go |
| Persistence of peer list | Restarts lose all peer knowledge |
| Authentication between peers | Any TCP client can connect and read/write |

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

## 18. Nice to Haves — Improvements & Future Work

These are ordered by impact and implementation difficulty. Items at the top provide the most value relative to effort.

---

### 1. Fix the Get Synchronisation (Replace `time.Sleep`)

**Priority: Critical — correctness bug**

The `Get` method sleeps 500ms after broadcasting `MessageGetFile` and then reads from whatever peer happens to have responded. This is a race condition — slow networks miss the window entirely.

**Fix:** Use a dedicated channel per inflight Get request:

```go
// In FileServer, maintain a map of inflight gets
type getResult struct{ reader io.Reader }

inflightGets map[string]chan getResult

// In Get()
ch := make(chan getResult, 1)
s.inflightGets[hashKey(key)] = ch
s.broadcast(msg)
select {
case result := <-ch:
    return result.reader, nil
case <-time.After(5 * time.Second):
    return nil, fmt.Errorf("timeout: no peer has file %s", key)
}
```

---

### 2. Fix the Encryption Bug in `handleMessageGetFile`

**Priority: Critical — security bug**

When a peer responds to a `MessageGetFile`, it sends the file via `io.Copy(peer, r)` — plaintext. But the requester decodes it with `store.WriteDecrypt(s.EncKey, ...)`. This means the written file is corrupted (decrypted gibberish written as if it were encrypted data).

**Fix:** Change `handleMessageGetFile` to encrypt the response:

```go
// Before (broken):
n, err := io.Copy(peer, r)

// After (correct):
n, err := copyEncrypt(s.EncKey, r, peer)
```

---

### 3. Replace MD5 Key Hashing with SHA-256

**Priority: High — security hardening**

`hashKey()` uses MD5 which is cryptographically broken. MD5 collisions can be produced in milliseconds with modern hardware — two different file keys could map to the same hash, causing a silent file collision on disk.

**Fix:**
```go
func hashKey(key string) string {
    hash := sha256.Sum256([]byte(key))
    return hex.EncodeToString(hash[:])
}
```

---

### 4. Add File Integrity Verification

**Priority: High — data correctness**

Currently a truncated or corrupt transfer is written to disk with no detection. Received files should be verified against a checksum included in the `MessageStoreFile` control message.

**Fix:** Include a SHA-256 hash of the plaintext in `MessageStoreFile`. After `WriteDecrypt`, hash the written file and compare. Delete the file and return an error if they don't match.

---

### 5. Add Replication Factor Control

**Priority: High — scalability**

The current design replicates to **every connected peer** — this becomes bandwidth-prohibitive at scale. Add a `ReplicationFactor int` to `FileServerOpts` and track which peers hold which keys.

```go
// In FileServerOpts
ReplicationFactor int  // e.g. 3

// In Store: pick n peers instead of all
chosen := selectPeers(s.peers, s.ReplicationFactor)
for _, peer := range chosen {
    peer.Send(...)
}
```

Pair this with a `FileIndex` — a map of `key → []peerAddr` — so that `Get` knows which specific peer to query rather than broadcasting.

---

### 6. Add Node Failure Detection (Heartbeats)

**Priority: High — reliability**

Dead peers remain in the peer map forever. When a peer disconnects, all subsequent `Send` calls to it return errors that are currently silently swallowed. Add a heartbeat goroutine per peer and remove peers that fail to respond.

```go
// Periodic ping every 10s; remove peer on 3 consecutive failures
go s.heartbeatLoop(peer)

func (s *FileServer) heartbeatLoop(peer p2p.Peer) {
    ticker := time.NewTicker(10 * time.Second)
    fails := 0
    for range ticker.C {
        if err := peer.Send(pingBytes); err != nil {
            fails++
            if fails >= 3 {
                s.removePeer(peer)
                return
            }
        } else {
            fails = 0
        }
    }
}
```

---

### 7. Add File Chunking for Large Files

**Priority: Medium — memory safety**

`Store` buffers the entire file into a `bytes.Buffer` before broadcasting:
```go
fileBuffer = new(bytes.Buffer)
tee = io.TeeReader(r, fileBuffer)
```

A 2GB video file would require 2GB of heap. Fix this by splitting files into fixed-size chunks (e.g. 1MB), streaming each chunk independently, and re-assembling on the receiver.

---

### 8. Replace Full Broadcast with a DHT

**Priority: Medium — scalability architecture**

Every `Get` and `Store` broadcasts to all peers. At 100 nodes this means every operation triggers 99 simultaneous TCP messages. Replace the flat peer map with a Kademlia DHT: each node only talks to O(log n) peers to locate or route a file.

---

### 9. Add an HTTP or gRPC API

**Priority: Medium — usability**

Right now the only entry point is `main.go`. Add a thin HTTP layer so the system can be used without writing Go code:

```
PUT  /files/{key}     ← body = file bytes
GET  /files/{key}     ← returns file bytes
DELETE /files/{key}   ← removes locally (and optionally notifies peers)
GET  /peers           ← returns currently connected peers
GET  /files           ← lists locally stored file keys
```

---

### 10. Stable Node Identity Across Restarts

**Priority: Medium — operational**

Node IDs are regenerated on every startup because `generateID()` uses `crypto/rand`. If a node restarts, all peers that cached its ID lose the association. Persist the ID to a `.nodeid` file in `StorageRoot` and reuse it on restart.

---

### 11. Add Peer Authentication (TLS)

**Priority: Medium — security**

Any TCP client can connect and issue Store/Get commands. Wrap the TCP transport with `crypto/tls` using mutual certificate authentication, so only trusted nodes can join the network.

---

### 12. Implement a File Listing / Metadata Index

**Priority: Low — feature completeness**

There is no way to discover what files exist in the network. Add a `FileIndex` structure (backed by a local BoltDB or SQLite) that maps `key → {peerAddr, size, storedAt}`. Expose it via `GET /files` over the HTTP API.

---

### 13. Add Comprehensive Integration Tests

**Priority: Low — quality**

Unit tests cover crypto and the CAS store, but there are no network-level tests. Add an integration test that spins up 3 in-process nodes over loopback TCP, stores a file on one, and verifies retrieval on all others.

```go
func TestStoreAndGetAcrossNetwork(t *testing.T) {
    s1 := makeTestServer(t, ":14000")
    s2 := makeTestServer(t, ":14001", ":14000")
    // ...
    s3.Store("testkey", bytes.NewReader([]byte("hello world")))
    time.Sleep(100 * time.Millisecond)
    r, err := s1.Get("testkey")
    // assert r content == "hello world"
}
```

---

### 14. Add Observability (Metrics & Structured Logging)

**Priority: Low — operational**

The project uses raw `fmt.Printf` and `log.Println`. Replace with structured logging (`log/slog` in Go 1.21+) and add Prometheus metrics:
- `dfs_bytes_stored_total`
- `dfs_bytes_fetched_network_total`
- `dfs_peer_count`
- `dfs_get_latency_ms`
