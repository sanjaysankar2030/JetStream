# Quick Reference & Learning Roadmap

## Your Learning Journey

```
Week 1-2: Foundation
  ├─ Understand deadlocks/locks from DBMS → why consensus?
  ├─ Learn latency breakdown (cache, memory, disk, network)
  └─ Understand state machine replication

Week 3-4: Consensus
  ├─ Study Raft algorithm (5-10 hours)
  ├─ Implement simplified Raft (20-30 hours)
  └─ Test with 3-node cluster locally

Week 5-6: Integration
  ├─ Add Raft to your DFS metadata
  ├─ Benchmark before/after
  └─ Identify bottlenecks

Week 7-8: Optimization
  ├─ Apply TigerBeetle patterns (structural sharing, batching)
  ├─ Apply Matklad patterns (dense arrays, incremental processing)
  └─ Measure latency improvements
```

---

## Mental Model Cheat Sheet

### Consensus Algorithms (Choose One)

```
┌─ Raft (Choose this for your DFS)
│  ├─ Leader-based
│  ├─ Easier to understand
│  ├─ Latency: ~50-100ms
│  ├─ Used in: etcd, Consul, CockroachDB
│  └─ Implementation: hashicorp/raft (Go)
│
├─ Paxos (Academic, harder)
│  ├─ Leaderless
│  ├─ More complex
│  ├─ Latency: Same as Raft
│  └─ Used in: Google Chubby (internal)
│
└─ BFT (Blockchain)
   ├─ Handles malicious nodes
   ├─ Huge overhead
   ├─ Latency: ~1-5 seconds
   └─ Used in: Bitcoin, Ethereum
```

### Latency Hierarchy (Memorize These)

```
L1 Cache:      ~4 nanoseconds      (fastest)
L3 Cache:      ~40 nanoseconds
RAM:           ~100 nanoseconds
SSD Seek:      ~100 microseconds
Network (LAN): ~100 microseconds
HDD Seek:      ~10 milliseconds
Network (WAN): ~10-150 milliseconds (slowest)

Golden rule:
  Each level ~100x slower than previous
  Network is ~1,000,000x slower than L1 cache
```

### Throughput vs Latency Trade-off

```
YOUR OPTIONS:

1. Single-threaded (no locks)
   Latency: LOW (no contention)
   Throughput: LIMITED (one thread)
   Best for: Ultra-low latency needs

2. Multi-threaded (locks)
   Latency: HIGH (contention)
   Throughput: GOOD (parallel)
   Best for: Throughput-critical systems

3. Sharded (multiple independent threads)
   Latency: LOW (no contention within shard)
   Throughput: GOOD (parallel across shards)
   Best for: BALANCED (your DFS)
   
4. Batch processing
   Latency: MEDIUM (amortized)
   Throughput: EXCELLENT (batching)
   Best for: TigerBeetle-style systems
```

---

## Key Concepts (One Sentence Each)

| Concept | Meaning | Example |
|---|---|---|
| **Consensus** | Multiple nodes agree on a single value | All peers agree on which node holds file X |
| **State Machine** | Deterministic: same input → same output | Apply [Op1, Op2, Op3] in order = consistent result |
| **Log Replication** | Replicate operations (not state) | Send [Op1, Op2] to all nodes; they apply locally |
| **Quorum** | Majority (>50%) must confirm | If 3 nodes, 2 confirmations = safe |
| **Latency** | Time for one operation | Store operation takes 100ms |
| **Throughput** | Operations per second | System handles 10,000 ops/sec |
| **Structural Sharing** | Share old data, create new only for changes | Update file → new FileIndex shares old entries |
| **Locality of Reference** | Access nearby data (cache-friendly) | Use arrays, not pointers; data fits in L1 |
| **Pipelined** | Start next operation before previous finishes | Send Op2 while Op1 replicating |

---

## Code Patterns to Remember

### Pattern 1: Single-Threaded Event Loop
```go
func eventLoop() {
    for op := range opQueue {
        // NO LOCKS — only this goroutine accesses state
        applyOperation(op)
    }
}
```

### Pattern 2: Batch Processing
```go
batch := []Op{}
for {
    batch = append(batch, <-opQueue)
    if len(batch) >= 1000 {
        persistToDisk(batch)  // One I/O, 1000 ops
        batch = []Op{}
    }
}
```

### Pattern 3: State Machine Replication
```go
// All nodes apply same operations in same order
for _, op := range log {
    state = applyOperation(state, op)
}
// Result: all nodes have identical state
```

### Pattern 4: Log Replication (Raft)
```go
// Leader sends log entries to followers
leader.log = append(leader.log, op)
for _, follower := range followers {
    follower.send(op)
    if follower.confirms() {
        leader.commitIndex++
    }
}
```

### Pattern 5: Dense Arrays (Cache-Friendly)
```go
// SLOW: Random access via pointers
peers := []*Peer{peer1, peer2, ...}

// FAST: Dense arrays
latencies := []int32{10, 15, 20, ...}
for _, lat := range latencies {  // Sequential access = cache prefetch
    selectIfBest(lat)
}
```

---

## Tools & Libraries

### For Consensus
- **hashicorp/raft** (Go) — Production Raft implementation
- **etcd** (Go) — Distributed consensus store (uses Raft)
- **CockroachDB** (Go) — Database using Raft
- **TigerBeetle** (Zig) — Financial ledger (reference implementation)

### For Testing
- **testify** (Go) — Assertions and mocks
- **pprof** (Go builtin) — CPU/memory profiling
- **staticcheck** (Go) — Code quality

### For Observability
- **prometheus** — Metrics collection
- **jaeger** — Distributed tracing
- **slog** (Go 1.21+) — Structured logging

---

## Common Pitfalls & How to Avoid

| Pitfall | Symptom | Fix |
|---|---|---|
| Infinite election | Two leaders fighting | Add term checking (Raft requires this) |
| Lost writes | Data vanishes after crash | Persist log to disk before confirming |
| Split brain | Cluster splits into two groups | Require quorum (majority only) |
| Cache misses | Profiler shows 90% stalled cycles | Switch to dense arrays |
| Lock contention | More cores = slower | Eliminate locks with single-threaded loop |
| Network flooding | Network at 100% utilization | Batch operations, not individual ops |

---

## Performance Targets (After Each Phase)

### Phase 1: Single-threaded (Weeks 1-2)
```
Store throughput: 1,000,000 ops/sec
Get latency: ~1 microsecond
Network usage: N/A (local only)
Memory: All in-process
```

### Phase 2: Add Raft metadata (Weeks 3-4)
```
Metadata consensus: 5,000-10,000 ops/sec
Metadata latency: 50-100ms
Get latency: 1-5ms (metadata + disk read)
Network usage: O(log n) messages
Consistency: Strong (guaranteed)
```

### Phase 3: Optimize with TigerBeetle patterns (Weeks 5-6)
```
Metadata throughput: 50,000-100,000 ops/sec (with sharding)
Get latency: 1-2ms (structural sharing speeds up lookups)
Store latency: 10-50ms (batching amortizes Raft cost)
Memory: 50% of before (structural sharing)
CPU: Better cache locality (dense arrays)
```

---

## Debugging Checklist

When things go wrong, check in this order:

- [ ] **Is the leader elected?** (use `raft.State()`)
- [ ] **Are followers receiving log entries?** (check `follower.lastLogIndex`)
- [ ] **Is the commit index advancing?** (check `leader.commitIndex`)
- [ ] **Are entries being applied?** (check `node.lastApplied`)
- [ ] **Are the state machines synchronized?** (compare `node.state` across cluster)
- [ ] **Is the network working?** (ping test between nodes)
- [ ] **Did someone crash?** (check logs for panics)
- [ ] **Is there a partition?** (can nodes reach each other?)

---

## Reading List (By Difficulty)

### Easy (Start Here)
1. **"Designing Data-Intensive Applications" Ch. 8-9** (Martin Kleppmann)
   - Clear explanation of consensus
   - ~20 pages

2. **Raft Visualization** (https://raft.github.io/raftscope/index.html)
   - Interactive demo
   - ~1 hour

### Medium
3. **Raft Paper** (https://raft.github.io/raft.pdf)
   - Original paper (very readable)
   - ~20 pages
   - ~4 hours to fully understand

4. **TigerBeetle Design Doc** (https://github.com/tigerbeetle/tigerbeetle)
   - Real system design
   - ~30 pages
   - ~8 hours

### Hard
5. **Paxos Made Simple** (Lamport)
   - More complex than Raft
   - ~10 pages
   - ~6 hours

6. **Matklad's Blog Posts**
   - Deep dives on incremental processing
   - Variable length
   - Focus on sections relevant to your work

---

## Quick Start (TL;DR)

### If you have 2 weeks:
1. Read "Designing Data-Intensive Applications" Ch. 8
2. Watch Raft visualization
3. Implement 3-node Raft cluster (from examples/02)
4. Test with your DFS

### If you have 4 weeks:
1. Do above
2. Read Raft paper
3. Integrate Raft into your DFS
4. Benchmark improvements
5. Identify bottlenecks

### If you have 8 weeks (recommended):
1. Do above
2. Study TigerBeetle patterns
3. Implement structural sharing + dense arrays
4. Profile with pprof
5. Achieve 50,000+ ops/sec

---

## Success Metrics

After implementing:

```
BASELINE (your current system):
  - Store: 100-200 ops/sec
  - Get: 500ms+ latency
  - Network: O(n) broadcast
  - Consistency: Eventual (unreliable)

TARGET (with consensus):
  - Store: 5,000-10,000 ops/sec
  - Get: 1-5ms latency
  - Network: O(log n) with Raft
  - Consistency: Strong (guaranteed)

STRETCH GOAL (with optimization):
  - Store: 50,000-100,000 ops/sec (sharded)
  - Get: <1ms latency (cached)
  - Network: Minimal (metadata only)
  - Consistency: Strong + Durable

MEASUREMENT SCRIPT:
  1. Store 10,000 files sequentially → measure throughput
  2. Get same 10,000 files sequentially → measure latency
  3. Delete half → check consistency across replicas
  4. Kill leader → verify new leader elected
  5. Network partition → verify quorum survives
```

---

## One More Thing: The Intuition

> **Consensus is about having a "single source of truth" without actually having a single point of failure.**

Before consensus:
- One master = fast (no coordination)
- But crashes = lose everything

After consensus:
- Multiple replicas coordinate
- Slower (network roundtrips)
- But any node can be down (quorum survives)

That trade-off is the entire foundation of distributed systems.

---

## Resources by Topic

### Consensus Algorithms
- Raft Paper: https://raft.github.io/raft.pdf
- Raft Visualizer: https://raft.github.io/raftscope/index.html
- Interactive Demo: https://github.com/otoolep/raft

### High Performance Systems
- TigerBeetle: https://github.com/tigerbeetle/tigerbeetle
- Matklad's Blog: https://matklad.github.io/
- Designing Data-Intensive Applications: https://dataintensive.net/

### Go Libraries
- hashicorp/raft: https://github.com/hashicorp/raft
- etcd: https://etcd.io/
- gRPC: https://grpc.io/

---

## Next Document to Read

After finishing this, read:
1. **02_PRACTICAL_GO_EXAMPLES.md** — Code patterns
2. **03_DFS_CONSENSUS_INTEGRATION.md** — Your specific integration
3. **04_TIGERBEETLE_MATKLAD_PATTERNS.md** — Deep optimization
