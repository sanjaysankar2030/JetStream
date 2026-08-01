# Software Requirements Specification

## Distributed Content-Addressed Storage System with Streaming Ingestion

**Version:** 1.0
**Status:** Draft
**Author:** [Your Name]
**Date:** July 2026

---

## 1. Introduction

### 1.1 Purpose

This document specifies the functional and non-functional requirements for a
**peer-to-peer, content-addressed storage system** extended to support
**streaming, unordered event ingestion**. The system is a substantial rewrite
and extension of a reference TCP-based P2P content-addressed file store,
evolved from a static file-replication tool into an append-only, distributed
commit-log style storage engine with idempotent writes and out-of-order event
handling.

This SRS is intended to:
- Define what the system must do (functional requirements) and how well it
  must do it (non-functional requirements)
- Serve as a scope-control document, explicitly separating what is in scope
  from what is deliberately excluded
- Act as the basis for a project report / academic review, and as a resume
  / interview reference document

### 1.2 Scope

The system provides:
- A peer-to-peer network of nodes that discover and connect to each other
  over TCP
- Content-addressed storage: every piece of data is identified by a
  cryptographic hash of its contents, not by a file name or path
- Replication of stored data across multiple peers for durability
- A streaming ingestion path that accepts a continuous flow of small events
  (not just whole-file uploads), buffers them, and seals them into immutable,
  hash-addressed segments
- Idempotent ingestion — duplicate events (from retries, at-least-once
  delivery, etc.) are detected and dropped using content hashing
- Watermark-based handling of out-of-order events, so that events arriving
  late (relative to their own timestamp) are handled predictably rather than
  silently corrupting segment ordering

**Explicitly out of scope** (see Section 8.2 for full list):
- Any query language, filtering, or aggregation over stored data
- A SQL or SQL-like frontend of any kind
- Cross-datacenter / WAN replication (LAN / single-cluster scope only)
- Authentication/authorization and encryption of network traffic (noted as
  future work, not a requirement of this version)
- A user-facing UI/dashboard (a CLI is sufficient for this version)

### 1.3 Definitions, Acronyms, and Abbreviations

| Term | Definition |
|---|---|
| **CAS** | Content-Addressed Storage — data is retrieved by the hash of its content, not by a name or location |
| **Node** | A single running instance of the system; acts as both client and server |
| **Peer** | Any other node a given node is connected to |
| **Chunk** | A unit of stored, hashed, immutable content |
| **Segment** | A sealed, immutable batch of ingested events, identified by its content hash |
| **Active segment** | The current, not-yet-sealed, in-memory buffer accepting new events |
| **Sealing** | The act of freezing an active segment, hashing it, and making it immutable and replicable |
| **Manifest** | A record mapping logical identifiers (segment order, partition, etc.) to content hashes and peer locations |
| **Watermark** | A tracked point in event-time below which all events are assumed to have arrived |
| **Event-time** | The timestamp an event claims to have occurred at (as opposed to the time it physically arrived) |
| **Late event** | An event whose event-time falls before the current watermark |
| **Idempotent ingestion** | Ingesting the same event more than once has no additional effect beyond the first time |
| **At-least-once delivery** | A delivery guarantee where a message may be delivered more than once but never zero times |

### 1.4 References

- Reference P2P implementation basis: `anthdm/distributedfilesystemgo` (TCP-based P2P content-addressed file store)
- Conceptual precedents: Apache Kafka (commit log / segment model), Apache Flink / Kafka Streams (watermarks, event-time processing), Git and BitTorrent (content addressing, hash-based integrity)

### 1.5 Document Overview

Section 2 describes the system at a high level. Section 3 covers architecture.
Section 4 specifies functional requirements. Section 5 specifies
non-functional requirements. Section 6 specifies interfaces. Section 7
specifies data formats. Section 8 lists constraints and explicit
non-goals. Section 9 contains supporting appendices.

---

## 2. Overall Description

### 2.1 Product Perspective

The system is a standalone, self-hosted storage engine, not a hosted service
or a component of a larger commercial product. It is designed to run as
identical software on every node — there is no dedicated coordinator,
load balancer, or single point of control. Any node can accept writes;
any node can serve reads.

It builds directly on top of an existing TCP-based P2P transport and
hash-based storage layer, extending it from "store and replicate whole
files" to "continuously ingest, seal, and replicate ordered batches of
small events."

### 2.2 Product Functions (Summary)

1. Discover and maintain connections to peer nodes over TCP
2. Accept a continuous stream of individual events from producers
3. Detect and silently drop duplicate events using content hashing
4. Buffer incoming events in an active, in-memory segment
5. Track a watermark based on event-time and handle late-arriving events
   according to a defined policy
6. Seal the active segment into an immutable, hash-addressed segment once a
   size/time/watermark condition is met
7. Replicate sealed segments to peer nodes
8. Allow retrieval of any sealed segment, by any node, given its hash or its
   position in a manifest

### 2.3 User Characteristics

The primary "users" of this system are:
- **Developers/operators** running nodes and configuring cluster membership
- **Producer clients** (scripts, simulated event generators, or real
  applications) that send events into the system
- **Consumer clients** that read sealed segments back out for inspection or
  downstream processing (outside the scope of this system)

No end-user GUI is assumed; interaction is via CLI and a minimal HTTP/TCP
ingestion API.

### 2.4 Constraints

- Implementation language: Go (consistent with the existing codebase being
  extended)
- Network transport: TCP for peer-to-peer communication (existing transport
  layer is reused, not replaced)
- Single local network / cluster scope — no NAT traversal or WAN bootstrap
  is required
- No external database dependency — storage is files/chunks on local disk
  per node, addressed by hash

### 2.5 Assumptions and Dependencies

- Clocks across nodes are not assumed to be perfectly synchronized; the
  watermark mechanism is designed to tolerate some clock skew and network
  delay, not eliminate it
- Producers may retry sends (at-least-once delivery is assumed as the
  baseline, not exactly-once)
- Peers are assumed to be mutually trusted (no adversarial-peer handling
  in this version — see Section 8.2)

---

## 3. System Architecture Overview

### 3.1 High-Level Architecture

```
        Producer(s)                     Producer(s)
             │                                │
             ▼                                ▼
      ┌─────────────┐                  ┌─────────────┐
      │   Node A    │◄────TCP P2P─────►│   Node B    │◄──── ... Node N
      │             │                  │             │
      │ ┌─────────┐ │                  │ ┌─────────┐ │
      │ │ Ingest  │ │                  │ │ Ingest  │ │
      │ │ Pipeline│ │                  │ │ Pipeline│ │
      │ └────┬────┘ │                  │ └────┬────┘ │
      │      ▼      │                  │      ▼      │
      │ ┌─────────┐ │                  │ ┌─────────┐ │
      │ │ Active  │ │                  │ │ Active  │ │
      │ │ Segment │ │                  │ │ Segment │ │
      │ └────┬────┘ │                  │ └────┬────┘ │
      │      ▼ seal │                  │      ▼ seal │
      │ ┌─────────┐ │   replicate      │ ┌─────────┐ │
      │ │ Sealed  │ │─────────────────►│ │ Sealed  │ │
      │ │ Segments│ │◄─────────────────│ │ Segments│ │
      │ │ (CAS)   │ │                  │ │ (CAS)   │ │
      │ └─────────┘ │                  │ └─────────┘ │
      │  + Manifest │                  │  + Manifest │
      └─────────────┘                  └─────────────┘
```

### 3.2 Component Breakdown

| Component | Responsibility |
|---|---|
| **P2P Transport** | TCP connection management, peer handshake, message framing (existing layer, extended) |
| **Storage Engine (CAS)** | Content hashing, chunk storage on disk, chunk retrieval by hash |
| **Ingestion Pipeline** | Accepts incoming events, deduplicates by hash, applies watermark logic |
| **Active Segment Buffer** | In-memory holding area for not-yet-sealed events |
| **Sealing Manager** | Decides when to seal the active segment; hands sealed segment to Storage Engine |
| **Replication Manager** | Pushes sealed segments to peers; tracks replication acknowledgements |
| **Manifest Store** | Maps segment order / partition keys to content hashes and peer locations |

---

## 4. Functional Requirements

Each requirement has an ID, description, and priority (**M**andatory,
**S**hould-have, **C**ould-have).

### 4.1 Peer Discovery & Membership

| ID | Requirement | Priority |
|---|---|---|
| FR-1.1 | The system shall allow a node to be started with a list of one or more known peer addresses (bootstrap nodes) | M |
| FR-1.2 | The system shall establish and maintain TCP connections to configured peers | M |
| FR-1.3 | The system shall detect when a peer connection is lost and mark that peer as unreachable | M |
| FR-1.4 | The system shall attempt to reconnect to a lost peer on a retry interval | S |

### 4.2 Content-Addressed Storage

| ID | Requirement | Priority |
|---|---|---|
| FR-2.1 | The system shall compute a cryptographic hash (e.g. SHA-256) of any stored content to derive its address | M |
| FR-2.2 | The system shall store content on local disk keyed by its content hash | M |
| FR-2.3 | The system shall retrieve stored content given its content hash | M |
| FR-2.4 | The system shall reject or ignore writes of content whose hash already exists locally, without re-storing it | M |

### 4.3 Idempotent Ingestion

| ID | Requirement | Priority |
|---|---|---|
| FR-3.1 | The system shall compute a content hash for every incoming event at ingestion time | M |
| FR-3.2 | The system shall check whether an event's hash has already been ingested (within a bounded recent window) before accepting it | M |
| FR-3.3 | The system shall silently drop an event identified as a duplicate, without error to the producer | M |
| FR-3.4 | The system shall log/count duplicate-drop events for observability purposes | S |

### 4.4 Active Segment Buffering & Sealing

| ID | Requirement | Priority |
|---|---|---|
| FR-4.1 | The system shall maintain one active, mutable, in-memory segment per node that accepts new, non-duplicate events | M |
| FR-4.2 | The system shall seal the active segment when it reaches a configurable size threshold (event count or byte size) | M |
| FR-4.3 | The system shall seal the active segment when it reaches a configurable time threshold, even if the size threshold is unmet | M |
| FR-4.4 | The system shall seal the active segment when the watermark advances past a configured boundary (see 4.5) | S |
| FR-4.5 | Upon sealing, the system shall compute a content hash over the sealed segment's contents and store it immutably via the Storage Engine | M |
| FR-4.6 | Once sealed, a segment's contents shall not be modified | M |
| FR-4.7 | The system shall start a new empty active segment immediately after sealing | M |

### 4.5 Watermark-Based Out-of-Order Handling

| ID | Requirement | Priority |
|---|---|---|
| FR-5.1 | The system shall track a watermark value representing the event-time below which all events are assumed to have arrived | M |
| FR-5.2 | The system shall advance the watermark based on observed event-times, using a configurable lag/allowance to tolerate network delay | M |
| FR-5.3 | The system shall buffer incoming events for a configurable grace period to allow modest out-of-order arrival before sealing | M |
| FR-5.4 | The system shall classify any event whose event-time falls below the current watermark as "late" | M |
| FR-5.5 | The system shall apply a configurable late-event policy: (a) drop, or (b) route to a separate "late" segment | M |
| FR-5.6 | Within an active segment, the system shall order buffered events by event-time before sealing | S |

### 4.6 Replication

| ID | Requirement | Priority |
|---|---|---|
| FR-6.1 | Upon sealing, the system shall replicate the sealed segment to a configurable number of peer nodes | M |
| FR-6.2 | The system shall verify replication success via acknowledgement from the receiving peer | S |
| FR-6.3 | The system shall retry replication to a peer that failed to acknowledge, up to a configurable retry limit | S |
| FR-6.4 | The system shall update the Manifest Store with the set of peers holding a copy of each sealed segment | M |

### 4.7 Retrieval

| ID | Requirement | Priority |
|---|---|---|
| FR-7.1 | The system shall allow retrieval of any sealed segment by its content hash, from any node holding a copy | M |
| FR-7.2 | The system shall allow listing of sealed segments known to a node, via the Manifest Store | M |
| FR-7.3 | If a segment is not present locally, the system shall attempt retrieval from a peer known (via manifest) to hold it | S |

### 4.8 Integrity (Stretch)

| ID | Requirement | Priority |
|---|---|---|
| FR-8.1 | Each sealed segment may store the content hash of the immediately preceding segment, forming a hash chain | C |
| FR-8.2 | A node may verify the integrity of a sequence of segments by validating the hash chain | C |

---

## 5. Non-Functional Requirements

### 5.1 Performance

- NFR-1.1: Duplicate-detection checks (FR-3.2) should add negligible overhead relative to raw ingestion (target: sub-millisecond per event on commodity hardware)
- NFR-1.2: Segment sealing (FR-4.5) should not block acceptance of new events into the next active segment

### 5.2 Scalability

- NFR-2.1: The system should support a configurable number of peer nodes within a single LAN/cluster without requiring changes to core logic
- NFR-2.2: Segment size should be tunable so that memory usage of the active segment buffer remains bounded and predictable

### 5.3 Reliability / Fault Tolerance

- NFR-3.1: Loss of a single peer shall not prevent other peers from continuing to ingest, seal, and replicate independently
- NFR-3.2: A node restarting shall be able to reconstruct its manifest from sealed segments already on disk (no data loss for already-sealed segments)

### 5.4 Security

- NFR-4.1: Out of scope for this version — no authentication or encryption of peer traffic is required (documented explicitly as a known limitation, see Section 8.2)

### 5.5 Maintainability

- NFR-5.1: The Ingestion Pipeline, Sealing Manager, and Replication Manager shall be implemented as separable components with clear interfaces, so each can be tested independently
- NFR-5.2: Configuration (size thresholds, time thresholds, watermark lag, replication factor) shall be externalized, not hardcoded

### 5.6 Observability

- NFR-6.1: The system shall expose basic counters: events ingested, duplicates dropped, segments sealed, late events observed, replication successes/failures
- NFR-6.2: These counters should be queryable via a simple status endpoint or log output (a full metrics system such as Prometheus is optional, not required)

---

## 6. External Interface Requirements

### 6.1 Node-to-Node Protocol (P2P)

- Reuses the existing TCP-based framing and handshake from the base P2P
  transport layer
- New message types required:
  - `SEGMENT_PUSH` — sender pushes a sealed segment's content + hash to a peer
  - `SEGMENT_ACK` — receiver confirms successful storage of a pushed segment
  - `SEGMENT_FETCH` — requester asks a peer for a segment by hash
  - `SEGMENT_FETCH_RESPONSE` — responder returns segment content or a not-found indication

### 6.2 Producer Ingestion API

A minimal interface for producers to submit events, e.g. over HTTP:

```
POST /ingest
Content-Type: application/json

{
  "event_time": "2026-07-04T10:15:00Z",
  "payload": { ... arbitrary event fields ... }
}
```

Response:
```
200 OK        -> accepted (new event)
200 OK (dup)  -> accepted but identified as duplicate, no-op
202 Accepted  -> accepted as "late", routed per late-event policy
```

### 6.3 Retrieval / Status API

```
GET /segments                 -> list known sealed segments (hash, size, event count, time range)
GET /segments/{hash}          -> retrieve a sealed segment's contents
GET /status                   -> node health, peer list, counters (NFR-6.1)
```

### 6.4 Configuration Interface

Node startup configuration (file or flags) shall include, at minimum:
- Bootstrap peer addresses
- Segment size threshold (bytes or event count)
- Segment time threshold (seconds)
- Watermark lag / grace period (seconds)
- Late-event policy (`drop` | `late-segment`)
- Replication factor (number of peers to replicate each segment to)

---

## 7. Data Requirements

### 7.1 Event Format (pre-sealing)

```go
type Event struct {
    Hash      string    // content hash, computed at ingestion
    EventTime time.Time // producer-supplied event-time
    Payload   []byte    // arbitrary event content
    Received  time.Time // node-local arrival time (for observability only)
}
```

### 7.2 Sealed Segment Format

```go
type Segment struct {
    Hash        string    // content hash of the sealed segment as a whole
    PrevHash    string    // optional: hash of previous segment (FR-8.1, stretch)
    SealedAt    time.Time
    EventTimeMin time.Time
    EventTimeMax time.Time
    EventCount  int
    Events      []Event   // ordered by event-time (FR-5.6)
}
```

### 7.3 Manifest Entry

```go
type ManifestEntry struct {
    SegmentHash string
    SealedAt    time.Time
    Peers       []string // node addresses known to hold a replica
}
```

### 7.4 Watermark State (per node)

```go
type WatermarkState struct {
    Current    time.Time // current watermark value
    LagAllowed time.Duration // configured tolerance
}
```

---

## 8. Constraints, Assumptions, and Explicit Non-Goals

### 8.1 Design Constraints

- Must be built as an extension of the existing TCP P2P transport and CAS
  storage layer, not a from-scratch rewrite of those parts
- Must remain queryless: no filtering, aggregation, or query language of any
  kind is to be implemented as part of this system

### 8.2 Explicit Non-Goals

Stating these clearly is a strength for a report/viva, not a weakness:

- **Not a query engine or data warehouse frontend** — there is no SQL, no
  filter/aggregate API, and no plan to add one. Any "querying" is left to
  systems built on top of this one.
- **Not WAN-scale** — designed and tested for a single LAN/cluster; no NAT
  traversal or cross-network bootstrap.
- **Not secure by default** — no authentication or transport encryption in
  this version. Any peer on the network can, in principle, push or request
  segments. Documented as a known limitation and future-work item, not a
  hidden gap.
- **Not exactly-once delivery** — idempotent ingestion (FR-3.x) makes
  duplicate *effects* harmless, but this is not the same guarantee as
  exactly-once delivery at the transport level.
- **Not perfectly ordered** — the watermark mechanism (FR-5.x) bounds how
  out-of-order the system tolerates events, it does not guarantee perfect
  global ordering across all producers.

---

## 9. Appendices

### 9.1 Use Case Scenarios

**UC-1: Normal ingestion and sealing**
A producer sends a steady stream of events with mostly-increasing
event-times. The active segment fills to its size threshold, seals, hashes,
and replicates to peers. A new active segment begins accepting events
immediately.

**UC-2: Duplicate event due to producer retry**
A producer times out waiting for an ingestion acknowledgement and resends
the same event. The resent event hashes identically to the original, is
detected as a duplicate, and is dropped — the segment's event count is
unaffected.

**UC-3: Late-arriving event**
An event with event-time earlier than the current watermark arrives (e.g.
due to network delay). Per the configured late-event policy, it is either
dropped or routed to a separate late-segment, rather than silently
corrupting the ordering of an already-sealing segment.

**UC-4: Peer failure during replication**
A node seals a segment and attempts to replicate it to a peer that has just
gone offline. Replication is retried up to the configured limit; if still
unsuccessful, the segment remains valid and retrievable from the originating
node, and replication is retried later once the peer reconnects.

### 9.2 Glossary

See Section 1.3.

### 9.3 Future Work (Not Part of This Version)

- Authentication and encryption of peer-to-peer traffic
- Hash-chained segment integrity verification (FR-8.x) as a default rather
  than stretch feature
- Parallel multi-peer chunk retrieval for faster reads
- Tiered replication (hot/cold placement) based on access frequency
- Cross-cluster / WAN bootstrap via a rendezvous server

---

*End of document.*
