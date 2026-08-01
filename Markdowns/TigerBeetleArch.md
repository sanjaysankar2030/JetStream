# TigerBeetle & Matklad: Patterns for High-Performance Distributed Systems

Connecting the dots between consensus algorithms, low-latency database design, and parser optimization.

---

## Why These Two?

**TigerBeetle**: Optimizes for sub-millisecond latency in distributed financial systems.

**Matklad (rust-analyzer)**: Optimizes for fast incremental parsing (cache-friendly algorithms).

**Synthesis**: Both use the same underlying principles:
1. **Structural sharing** (don't copy, reference)
2. **Locality of reference** (keep data together in memory)
3. **Batching** (amortize expensive operations)
4. **Incremental processing** (only process what changed)

---

## Core Pattern 1: Structural Sharing

### The Problem

```go
// NAIVE: Copy entire state on each update
type FileMetadata struct {
    Key      string
    Replicas []string
    Version  int
}

func updateFile(meta *FileMetadata) {
    newMeta := *meta  // Full copy
    newMeta.Replicas = append(newMeta.Replicas, "peer4")
    newMeta.Version++
    // Old meta still exists (memory waste)
}
```

When you have 1 million files and update one:
- You copy the entire metadata map
- Memory usage = O(n)
- Time = O(n)

### TigerBeetle Solution

```go
// STRUCTURAL SHARING: Only allocate what's new
type FileMetadata struct {
    Key      string
    Replicas []*Replica  // Pointers, not copies
    Version  int
    Parent   *FileMetadata  // Link to previous version
}

func updateFile(meta *FileMetadata) *FileMetadata {
    // Create only NEW node
    return &FileMetadata{
        Key:     meta.Key,
        Replicas: append(meta.Replicas, &Replica{id: "peer4"}),  // Shallow copy of slice
        Version: meta.Version + 1,
        Parent:  meta,  // Link to previous
    }
}

// Benefits:
// - Time to update: O(k) where k = changed items (not n)
// - Memory: Only new items allocated
// - Can recover any historical version (traverse Parent pointers)
```

### Matklad's Approach (from rust-analyzer)

```
Previous State:
  SyntaxTree {
    children: [
      Node("fn"),
      Node("name"),
      Node("params"), ← Only this changed
      Node("body"),
    ]
  }

New State (after edit in params):
  SyntaxTree {
    children: [
      Node("fn"),        ← reused (same)
      Node("name"),      ← reused (same)
      Node("params"),    ← NEW (reparsed)
      Node("body"),      ← reused (same)
    ]
  }

Total memory for both: Old + New shared + Updated node
No duplication of unchanged parts
```

### Applied to Your DFS

```go
type FileIndex struct {
    Files    map[string]*FileMetadata
    Version  int
    Previous *FileIndex  // Structural sharing link
}

func (idx *FileIndex) AddFile(meta *FileMetadata) *FileIndex {
    newFiles := make(map[string]*FileMetadata, len(idx.Files)+1)
    
    // Shallow copy all old entries (just copy pointers)
    for k, v := range idx.Files {
        newFiles[k] = v
    }
    
    // Add new entry
    newFiles[meta.Key] = meta
    
    // Return new index (shares all old entries)
    return &FileIndex{
        Files:    newFiles,
        Version:  idx.Version + 1,
        Previous: idx,  // Can traverse history
    }
}

// Query any historical version
func (idx *FileIndex) GetAtVersion(version int) *FileIndex {
    current := idx
    for current.Version > version {
        current = current.Previous
    }
    return current
}
```

**Memory cost:**
- Before: Adding file = copy entire map = O(n) memory
- After: Adding file = one new map entry = O(1) memory

---

## Core Pattern 2: Locality of Reference

### The Problem

```go
// RANDOM ACCESS: Cache misses everywhere
type Peer struct {
    ID        string  // Accessed rarely
    Addr      string  // Accessed always
    Score     float64 // Accessed rarely
    Bandwidth int     // Accessed rarely
    LastSeen  int64   // Accessed rarely
}

peers := []*Peer{peer1, peer2, peer3, ...}

// When selecting peer for replication:
for _, peer := range peers {
    if peer.Addr == target {  // Jump to random memory address
        // L1 cache miss (CPU had to wait ~4 nanoseconds)
    }
}
```

Matklad's insight: Pointers are cache-killers. Every pointer dereference jumps CPU to random memory.

### Matklad Solution (from rust-analyzer)

```
Instead of:
  struct TokenInfo { id, line, col, kind, ... }
  let tokens: Vec<&TokenInfo> = ...

Do:
  struct TokenID(usize)
  let ids: Vec<TokenID> = ...           // Dense array
  let lines: Vec<u32> = ...             // Dense array
  let cols: Vec<u32> = ...              // Dense array
  let kinds: Vec<TokenKind> = ...       // Dense array
  
  fn get_token(id: TokenID) -> Token {
      let i = id.0;
      return Token {
          line: lines[i],
          col: cols[i],
          kind: kinds[i],
      }
  }

Why? All arrays fit in L1 cache. CPU prefetch works.
Performance: 1000x faster than pointer chasing.
```

### Applied to Your DFS

```go
// NAIVE: Pointers everywhere (cache misses)
type Peer struct {
    ID       string
    Addr     string
    Latency  int       // milliseconds
    Health   string
}

type PeerIndex struct {
    Peers []*Peer  // Pointers
}

// FAST: Dense arrays (cache-friendly)
type PeerID int32

type PeerStore struct {
    ids      []string         // Dense array
    addrs    []string         // Dense array
    latencies []int32         // Dense array
    health   []uint8          // Dense array (1 byte per peer)
    
    idToIndex map[string]PeerID  // Lookup
}

func (ps *PeerStore) GetPeer(id PeerID) Peer {
    i := int(id)
    return Peer{
        ID:      ps.ids[i],
        Addr:    ps.addrs[i],
        Latency: ps.latencies[i],
        Health:  ps.health[i],
    }
}

// Selecting peer with lowest latency
func (ps *PeerStore) SelectFastest() PeerID {
    best := PeerID(0)
    bestLatency := ps.latencies[0]
    
    // Linear scan through dense arrays
    // CPU can prefetch next elements → blazing fast
    for i := 1; i < len(ps.latencies); i++ {
        if ps.latencies[i] < bestLatency {
            best = PeerID(i)
            bestLatency = ps.latencies[i]
        }
    }
    return best
}
```

**Performance:**
- Before: Random access to 100 Peer pointers = 100 cache misses
- After: Linear scan of 100 latencies in one array = 2 cache misses (prefetch)

---

## Core Pattern 3: Single-Threaded Event Loop

### Why Locks are Slow (Revisited)

TigerBeetle's insight:
> **Lock contention is latency poison.**
> The cost of a lock isn't the lock itself; it's context switching.

```
Thread 1 wants lock:
  1. Wait for Thread 2 to release (spin)
  2. OS reschedules Thread 1
  3. CPU flushes cache (wrong thread's data!)
  4. Thread 1 loads cache for critical section
  5. Run critical section
  6. Release lock
  
Cost: ~1 millisecond per context switch
```

### Solution: Single Logical Thread (Multiple Physical Cores)

```go
// Option 1: One goroutine (simple, but wastes cores)
func eventLoop() {
    for op := range opQueue {
        applyOperation(op)
    }
}

// Option 2: Fan-out by shard (use all cores)
type ShardedQueue struct {
    queues [8]chan Operation  // 8 shards
    hashes []RaftNode         // 8 Raft instances
}

func (sq *ShardedQueue) eventLoop() {
    for i := 0; i < 8; i++ {
        go sq.shardWorker(i)  // One per core
    }
}

func (sq *ShardedQueue) shardWorker(shard int) {
    for op := range sq.queues[shard] {
        sq.hashes[shard].Apply(op)
    }
}

// When submitting operation:
func (sq *ShardedQueue) Submit(op Operation) {
    shard := hash(op.Key) % 8  // Route by key
    sq.queues[shard] <- op     // Send to relevant shard
}

// Benefits:
// - 8 logical threads (one per core)
// - No lock contention (each shard independent)
// - Each core has warm cache
// - Throughput: 8x single-threaded
```

---

## Core Pattern 4: Incremental Processing (Batching)

### Matklad's Insight: Process Delta, Not Full State

```
rust-analyzer workflow:

File 1: fn foo() { ... }
File 2: fn bar() { ... }
File 3: fn baz() { ... }

User edits File 2 (line 5):

Option 1 (NAIVE): Reparse ALL files
  Time: Parse File 1 + File 2 + File 3 = 100ms

Option 2 (INCREMENTAL): Reparse ONLY changed file
  Time: Parse File 2 = 10ms
  Result: 10x faster
```

### Applied to Your DFS

```go
// NAIVE: Entire metadata sync on every change
type FileIndex map[string]*FileMetadata

func (s *Server) Broadcast(index FileIndex) {
    // Send ENTIRE index to all peers
    for _, peer := range s.peers {
        peer.Send(index)  // 10MB of metadata!
    }
}

// INCREMENTAL: Only send changes
type MetadataUpdate struct {
    Added    map[string]*FileMetadata
    Removed  map[string]bool
    Modified map[string]*FileMetadata
    Version  int
}

func (s *Server) BroadcastDelta(update MetadataUpdate) {
    // Send ONLY changes (100KB instead of 10MB)
    for _, peer := range s.peers {
        peer.Send(update)  // Delta
    }
}

// Peer applies locally:
func (idx *FileIndex) ApplyDelta(update MetadataUpdate) {
    for k, v := range update.Added {
        (*idx)[k] = v
    }
    for k := range update.Removed {
        delete(*idx, k)
    }
    for k, v := range update.Modified {
        (*idx)[k] = v
    }
}

// Cost reduction:
// - Network: 100x less data
// - CPU: Only process changes
// - Memory: Only allocate new entries
```

---

## TigerBeetle's Complete Picture

### The Ledger Operations Pattern

TigerBeetle is a **financial ledger**. Operations:

```
Transfer Account A → B: $100
  Before: {A: $1000, B: $500}
  After:  {A: $900,  B: $600}

This requires:
  1. Atomicity (both operations or neither)
  2. Durability (survives crash)
  3. Consistency (can't create/destroy money)
  4. Low latency (~1ms)
  5. High throughput (~1M ops/sec)
```

### How TigerBeetle Achieves It

1. **Deterministic State Machine**
   - Every transfer operation is deterministic
   - Same input → same output (always)
   - If all replicas apply same ops in order → consistency

2. **Log-Structured Replication**
   ```
   Node 1: [Transfer1, Transfer2, Transfer3, ...]
   Node 2: [Transfer1, Transfer2, Transfer3, ...]
   Node 3: [Transfer1, Transfer2, Transfer3, ...]
   
   All apply in order → all have same state
   ```

3. **Pipelined Consensus**
   ```
   Time 0: Send Transfer1 to replicas
   Time 1: Replicas respond to Transfer1
           Meanwhile, send Transfer2 to replicas
   Time 2: Replicas respond to Transfer2
           Meanwhile, send Transfer3 to replicas
   
   Latency per transfer: Network roundtrip (constant)
   Throughput: Unlimited (by pipelining)
   ```

4. **Disk I/O Batching**
   ```
   Batch 1000 operations together
   Write to disk ONCE (amortized)
   Reply to clients
   
   Cost per operation:
   - Disk I/O: 10ms ÷ 1000 = 10µs
   - Network: ~1ms
   - Total: ~1ms per operation
   ```

5. **SIMD Accounting**
   ```go
   // Old: Check each transfer individually
   for _, t := range transfers {
       if accounts[t.From].balance < t.Amount {
           return error  // Insufficient funds
       }
   }
   
   // TigerBeetle: Process in parallel (SIMD)
   // Check 64 transfers at once using CPU vector instructions
   // Result: 64x speedup
   ```

---

## Synthesis: Building Your Optimal DFS

### Layer 1: Fast Metadata (TigerBeetle-inspired)

```go
// Single-threaded event loop, no locks
func (cluster *Cluster) metadataLoop() {
    batch := []MetadataOp{}
    
    for {
        // Collect batch
        op := <-cluster.opQueue
        batch = append(batch, op)
        
        if len(batch) >= 1000 {
            // Flush batch
            cluster.persistBatch(batch)
            cluster.replicateBatch(batch)
            cluster.applyBatch(batch)
            batch = []MetadataOp{}
        }
    }
}

// Structural sharing: metadata versions
type FileIndex struct {
    files    map[string]*FileMetadata
    version  int
    previous *FileIndex  // History
}

// Dense arrays: peer storage
type PeerStore struct {
    ids       []string
    addrs     []string
    latencies []int32
    // All in one cache line
}
```

### Layer 2: Distributed Consensus (Raft)

```go
// Raft handles replication automatically
// We just apply operations to state machine

// Query: O(1) local lookup
func (cluster *Cluster) Get(key string) (*FileMetadata, error) {
    return cluster.currentFileIndex.Get(key)
}

// Write: O(log n) network messages (Raft)
// Committed when majority responds
```

### Layer 3: File Storage (Async Replication)

```go
// File transfer decoupled from metadata consensus
// Metadata says "file should be on peer 1, 2, 3"
// Replication handles actually getting it there

// Incremental transfers
func (s *Server) transferFile(dst *Peer, key string) {
    // Transfer only once (not on every operation)
    file := s.store.Read(key)
    s.streamToPeer(dst, file)
}
```

---

## Performance Model

```
Metadata Operations (Raft Consensus):
  Latency: 10-100ms (network roundtrip)
  Throughput: 1,000-10,000 ops/sec
  Consistency: Strong
  
File Storage (Direct Transfer):
  Latency: 1-10ms (just network)
  Throughput: Limited by bandwidth
  Consistency: Eventual

Total System:
  Client writes: 10-100ms (metadata consensus)
  Client reads: 1-5ms (local lookup + disk read)
  Network efficiency: 100x better than broadcast
  Scalability: O(log n) instead of O(n)
```

---

## Matklad's Testing Philosophy

Matklad emphasizes:
> "Don't test the system. Test the invariants."

Applied to your DFS:

```go
// Instead of: "Test that Store() works"
// Test: "After any sequence of Store/Delete/Get,
//        all replicas have identical state"

func TestInvariant_ReplicasConsistent(t *testing.T) {
    for trial := 0; trial < 100; trial++ {
        // Random operations
        ops := generateRandomOps(1000)
        
        for _, op := range ops {
            cluster.Apply(op)
        }
        
        // Invariant: all replicas identical
        state1 := cluster.node1.FileIndex
        state2 := cluster.node2.FileIndex
        state3 := cluster.node3.FileIndex
        
        assert.Equal(t, state1, state2)
        assert.Equal(t, state2, state3)
    }
}

// This catches bugs that specific tests miss
```

---

## Key Takeaways

| Pattern | Source | Benefit |
|---|---|---|
| Structural Sharing | TigerBeetle + Matklad | Memory efficiency (O(k) not O(n)) |
| Dense Arrays | Matklad (rust-analyzer) | CPU cache efficiency (1000x speedup) |
| Single-Threaded Loop | TigerBeetle | No lock contention |
| Incremental Processing | Matklad | Process only changes |
| Log-Structured | TigerBeetle | Durability + consistency |
| Pipelined Replication | TigerBeetle | High throughput despite high latency |
| Invariant Testing | Matklad | Catch all bugs, not just expected ones |

---

## Your Next Steps

1. **Implement structural sharing** in your FileIndex
2. **Convert to dense arrays** for PeerStore
3. **Add Raft** for metadata consensus (use hashicorp/raft)
4. **Measure latency** before and after
5. **Write invariant tests** for replication consistency
6. **Profile cache misses** (pprof) to identify bottlenecks

You'll likely see:
- **Store latency**: 500ms → 50-100ms
- **Get latency**: 500ms+ → 1-5ms
- **Throughput**: 100 ops/sec → 5,000+ ops/sec
- **Network efficiency**: O(n) → O(log n) messages