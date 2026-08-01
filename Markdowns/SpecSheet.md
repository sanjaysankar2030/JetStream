# Distributed File System in Go — Implementation Guide

Build order matters. Each phase depends on the previous one being solid.
Don't move to the next phase until the current one works independently.

---

## Phase 1 — Local CAS Storage

**What you're building:** A struct that saves files to disk at a path derived from the file's key, not its name.

**Steps:**

1. Create `store.go`. Define a `PathTransformFunc` type — it's just a function that takes a string key and returns a struct with two fields: the nested directory path, and the filename.

2. Implement `DefaultPathTransformFunc` first — it just returns the key as both the path and filename. Flat, no nesting. Use this to get the basics working.

3. Implement `CASPathTransformFunc` — SHA-1 hash the key, take the hex string, split it into 5-character chunks, join them with `/` as the path, and use the full hash as the filename.

4. Define the `Store` struct. It needs a root directory string and a `PathTransformFunc`.

5. Implement these methods one at a time, testing each:
   - `Has(id, key)` — does the file exist on disk?
   - `Write(id, key, reader)` — write bytes from a reader to the derived path under `root/id/`
   - `Read(id, key)` — open the file and return its size and a reader
   - `Delete(id, key)` — remove the entire first directory segment (not just the file)
   - `Clear()` — remove the whole root directory (for tests)

6. Write unit tests. Store a string, read it back, confirm it matches. Delete it, confirm it's gone.

**Why the `id` parameter?** Multiple nodes on the same machine share the same root. The `id` namespaces each node's files so they don't collide.

---

## Phase 2 — Encryption

**What you're building:** Two streaming functions — one encrypts, one decrypts — using AES-256 in CTR mode.

**Steps:**

1. Create `crypto.go`.

2. Write `copyEncrypt(key, src, dst)` — generates a random 16-byte IV, creates an AES block cipher, creates a CTR stream, writes the IV to `dst` first, then streams all bytes from `src` through the cipher into `dst`.

3. Write `copyDecrypt(key, src, dst)` — reads the first 16 bytes from `src` as the IV, creates the same cipher setup, streams remaining bytes back to plaintext into `dst`.

4. Write `newEncryptionKey()` — generates 32 random bytes from `crypto/rand`. That's your 256-bit AES key.

5. Write `generateID()` — 32 random bytes encoded as a 64-character hex string. This becomes a node's identity.

6. Write `hashKey(key)` — for now use MD5 to derive a fixed-length key for network lookups. (You'll replace this with SHA-256 later.)

7. Write unit tests — encrypt a string, decrypt it, confirm round-trip.

**Why CTR mode?** It turns AES into a stream cipher. No padding needed, works byte-by-byte, and encrypt/decrypt are the same operation. Perfect for streaming files of unknown size.

---

## Phase 3 — P2P Abstractions

**What you're building:** Interfaces that decouple the network protocol from the application logic. You're not implementing TCP yet — just defining the contracts.

**Steps:**

1. Create the `p2p/` package.

2. In `p2p/transport.go`, define two interfaces:
   - `Peer` — embeds `net.Conn`, adds `Send([]byte) error` and `CloseStream()`
   - `Transport` — has `Addr()`, `Dial(addr)`, `ListenAndAccept()`, `Consume()`, `Close()`

3. In `p2p/message.go`, define the `RPC` struct — it carries the sender's address, a raw payload byte slice, and a boolean `Stream` flag.

4. In `p2p/encoding.go`, define a `Decoder` interface with a single `Decode(conn, *RPC) error` method. Implement `GOBDecoder` and `DefaultDecoder`. The decoder reads from a connection and fills an RPC.

5. In `p2p/handshake.go`, define `HandshakeFunc` as a function type that takes a `Peer` and returns an error. Implement `NOPHandshakeFunc` — it does nothing and returns nil. This is the default.

6. Define two constants — `IncomingMessage` (byte `0x1`) and `IncomingStream` (byte `0x2`). These are the magic prefix bytes that signal what follows on the wire.

**Why interfaces?** `FileServer` will only talk to `Transport` and `Peer` — never to `TCPTransport` or `TCPPeer` directly. This means you could swap TCP for QUIC later without touching server logic.

---

## Phase 4 — TCP Transport

**What you're building:** A concrete implementation of `Transport` and `Peer` over raw TCP.

**Steps:**

1. Create `p2p/tcp_peer.go`. `TCPPeer` wraps a `net.Conn`. It has an `outbound bool` field (did we dial out, or did they connect to us?). Implement `Send` (just writes bytes to the connection) and `CloseStream` (signals end of a binary stream — use a channel or a wait group so the read loop knows to resume).

2. Create `p2p/tcp_transport.go`. `TCPTransport` holds a listen address, a listener, an `OnPeer` callback, and a channel of `RPC` messages.

3. Implement `ListenAndAccept` — calls `net.Listen`, then loops accepting connections. Each accepted connection becomes a `TCPPeer` and gets its own `handleConn` goroutine.

4. Implement `handleConn` — this is the read loop. It reads one byte (the magic prefix). If it's `IncomingMessage`, decode the next bytes as a gob-encoded payload and push an RPC to the channel. If it's `IncomingStream`, push an RPC with `Stream: true` and wait for `CloseStream` before resuming the loop.

5. Implement `Dial` — calls `net.Dial`, wraps the connection as a `TCPPeer` with `outbound: true`, fires the `OnPeer` callback, and starts the read loop.

6. Implement `Consume` — returns the RPC channel (read-only).

**The tricky part:** When the read loop sees `IncomingStream`, it must pause and let the application layer read the raw bytes directly from the connection. It only resumes after `CloseStream` is called. Get this right before moving on.

---

## Phase 5 — Message Protocol

**What you're building:** The two control message types that nodes send each other.

**Steps:**

1. Create `server.go`. Define the `Message` struct with a single `Payload any` field.

2. Define `MessageStoreFile` — fields: `ID string`, `Key string`, `Size int64`.

3. Define `MessageGetFile` — fields: `ID string`, `Key string`.

4. In an `init()` function, call `gob.Register` for both concrete types. This is required because `Payload` is typed as `any` — gob needs to know the concrete types ahead of time.

**Why two separate types?** Store and Get are fundamentally different operations. Store says "I have this file, here it comes." Get says "Does anyone have this file? Send it to me."

---

## Phase 6 — FileServer

**What you're building:** The core node. It wires together storage, encryption, and transport.

**Steps:**

1. Define `FileServerOpts` — fields: `ID`, `EncKey`, `StorageRoot`, `PathTransformFunc`, `Transport`, `BootstrapNodes`.

2. Define `FileServer` — holds opts, a `Store`, a peer map (`map[string]Peer`), a mutex protecting the peer map, and a quit channel.

3. Implement `NewFileServer` — auto-generate `ID` if empty, wire up the store, set `OnPeer` callback on the transport.

4. Implement `OnPeer(peer)` — locks the peer map, adds the peer keyed by its remote address.

5. Implement `Start()` — calls `transport.ListenAndAccept()`, calls `bootstrapNetwork()`, then enters the main loop.

6. Implement `bootstrapNetwork()` — dials each address in `BootstrapNodes` concurrently (one goroutine per address).

7. Implement the main `loop()` — selects on the transport's consume channel and the quit channel. For each RPC: if `Stream` is false, gob-decode the payload into a `Message` and call `handleMessage`. If `Stream` is true, handle accordingly.

8. Implement `broadcast(msg)` — gob-encodes the message, sends the `IncomingMessage` byte to every peer, then sends the encoded bytes.

9. Implement `Store(key, reader)`:
   - Use `io.TeeReader` to simultaneously write to a local buffer and to disk
   - After writing locally, broadcast a `MessageStoreFile` (with the encrypted size = plaintext size + 16 for the IV)
   - Send the `IncomingStream` byte to all peers
   - Use `io.MultiWriter` across all peers and call `copyEncrypt` once, streaming to everyone simultaneously

10. Implement `handleMessageStoreFile(from, msg)` — find the peer in the map, write the incoming stream to disk with `store.Write`, call `peer.CloseStream()` when done.

11. Implement `Get(key, reader)`:
    - Check `store.Has` first — if local, just return `store.Read`
    - If not local, broadcast `MessageGetFile`, sleep 500ms (placeholder), then read the response from peers with `store.WriteDecrypt`
    - After caching locally, return `store.Read`

12. Implement `handleMessageGetFile(from, msg)` — find the peer, read the file from local store, send size as a binary header, then stream the bytes.

13. Implement `Stop()` — close the quit channel.

---

## Phase 7 — Wire It Together (main.go)

**What you're building:** A demo that proves the whole system works end to end.

**Steps:**

1. Write a `makeServer(listenAddr, bootstrapNodes...)` helper — creates a `TCPTransport`, creates a `FileServer` with a fresh `EncKey`, unique `StorageRoot` per node (use the listen address as part of the name), and `CASPathTransformFunc`.

2. Spin up three nodes:
   - Node 1 on `:3000` — no bootstrap nodes
   - Node 2 on `:7000` — bootstraps to `:3000`
   - Node 3 on `:5000` — bootstraps to `:3000` and `:7000`

3. Start each node in its own goroutine.

4. Give the network a moment to connect (a small sleep).

5. Store a file on Node 3. Wait briefly. Retrieve it on Node 1. Print the contents.

6. Run with `go run main.go` — if you see the file contents printed, everything is working.

---

## Known Bugs to Fix After It Works

Once the happy path works, go back and fix these:

**Encryption bug in Get:** When a peer responds to `MessageGetFile`, it sends the file with plain `io.Copy`. But the receiver calls `WriteDecrypt`. This means the written file is corrupted. Fix: the sender must call `copyEncrypt`, or the receiver must call plain `Write`.

**Race condition in Get:** The 500ms sleep is fragile. Replace it with a channel-based mechanism — create a channel per in-flight Get request, have the handler send the result into it, and use `select` with a timeout instead of sleeping.

**MD5 key hashing:** `hashKey` uses MD5 which is broken. Replace with SHA-256.

**Dead peers:** Peers that disconnect stay in the map. Add a way to detect and remove them.

---

## Suggested Build Order for Testing

```
Phase 1 store tests pass
  → Phase 2 crypto tests pass
    → Phase 3 + 4 manual TCP echo test
      → Phase 5 message encoding/decoding test
        → Phase 6 FileServer with two nodes
          → Phase 7 three-node demo
            → Fix known bugs
```

Never skip a phase. The bugs you'll introduce by skipping will cost more time to debug than building in order.
