
# P2P Storage Network Desktop Explorer

## 1. Introduction

### 1.1 Purpose

This document defines the design and implementation plan for a **cross-platform desktop graphical user interface (GUI)** for a Go-based peer-to-peer, content-addressed storage system.

The underlying system is not a traditional centralized distributed file system such as HDFS.

It is closer to an **IPFS-like storage network**, where:

* Nodes operate independently.
* Nodes communicate directly with other nodes.
* Persistent TCP connections are maintained between peers.
* Data is identified using content-derived identifiers.
* Blocks can be retrieved from remote peers.
* Multiple peers may possess the same content.
* There is no mandatory centralized NameNode or master.
* Network state is inherently dynamic.

The desktop application will therefore act primarily as a:

> **P2P Network Explorer, Node Monitor, Content Explorer, and Development/Debugging Interface.**

It is not intended to replace the underlying node or protocol.

---

# 2. Problem Statement

The P2P system is primarily a backend/networking project.

Without a GUI, observing the system requires commands, logs, debuggers, or manually inspecting internal state.

For example, understanding:

```text
Peer A
   │
   ├── TCP connection → Peer B
   │
   ├── TCP connection → Peer C
   │
   └── requests block X from Peer D
```

from textual logs becomes difficult as the number of peers increases.

The GUI should therefore provide a visual representation of:

1. The local node.
2. Connected peers.
3. Persistent TCP connections.
4. Network activity.
5. Stored content.
6. Content providers.
7. Block retrieval.
8. Protocol events.
9. Node statistics.

The GUI is also intended to serve as a learning project for understanding **desktop GUI programming in Go**.

---

# 3. Project Goals

## 3.1 Primary Goals

The application shall:

* Run on Windows, Linux, and macOS.
* Be implemented primarily in Go.
* Use a native/cross-platform Go GUI toolkit.
* Connect to the P2P node without coupling UI code to networking internals.
* Display live peer information.
* Display network topology.
* Display content/block information.
* Display protocol events.
* Provide basic node controls.
* Provide useful debugging information.
* Remain usable when the P2P network contains multiple nodes.

## 3.2 Secondary Goals

The project should teach:

* GUI application architecture.
* Event-driven programming.
* Application state management.
* Layout management.
* Rendering.
* User input handling.
* Concurrency between networking and UI.
* Cross-platform desktop development.
* Separation between application logic and presentation.
* Visualization of graph/network data.

---

# 4. Non-Goals

The first version should NOT attempt to implement:

* A complete file manager.
* A full IPFS-compatible client.
* A sophisticated graphical IDE.
* A complete monitoring platform.
* Authentication and enterprise user management.
* Complex animations.
* A browser-based frontend.
* A custom rendering engine.
* A replacement for Prometheus/Grafana.

The goal is to build a **small but technically meaningful desktop application**.

---

# 5. Conceptual Model

The underlying system should be thought of as a collection of independent nodes.

```text
                    P2P NETWORK

              ┌───────────────┐
              │    Peer B     │
              └───────┬───────┘
                      │
                    TCP
                      │
                      │
┌───────────────┐     │     ┌───────────────┐
│    Peer C     │─────┼─────│    Peer D     │
└───────────────┘     │     └───────────────┘
                      │
                    TCP
                      │
              ┌───────▼───────┐
              │   Local Node  │
              └───────────────┘
```

Each node contains its own:

```text
Node
├── Identity
├── Peer Manager
├── Connection Manager
├── Protocol Engine
├── Block Store
├── Content Addressing
├── Request/Response Handling
├── Replication/Retrieval Logic
└── Metrics/Event System
```

The GUI observes and interacts with one node.

---

# 6. Important Architectural Principle

The GUI must NOT become part of the networking implementation.

Avoid:

```text
Fyne Button
     ↓
TCP Connection
     ↓
Protocol Packet
```

Instead:

```text
                 ┌──────────────────┐
                 │   Desktop GUI    │
                 └────────┬─────────┘
                          │
                    Application API
                          │
                 ┌────────▼─────────┐
                 │    P2P Node      │
                 └────────┬─────────┘
                          │
                    P2P Protocol
                          │
              ┌───────────┼───────────┐
              ▼           ▼           ▼
            Peer A      Peer B      Peer C
```

The GUI should communicate with an abstraction representing the node.

This separation allows the node to run:

* with a GUI,
* without a GUI,
* on a server,
* inside a test,
* inside a container,
* or as a headless process.

---

# 7. Recommended Technology

## 7.1 Language

Go.

Reason:

The underlying system is already written in Go, allowing the GUI to share:

* data structures,
* node interfaces,
* protocol definitions,
* statistics models,
* event definitions.

---

# 8. GUI Toolkit

## 8.1 Recommended Initial Choice

Use **Fyne**.

Fyne is a cross-platform GUI toolkit written in Go.

Official documentation:

https://docs.fyne.io/

The important reason for choosing Fyne is not that it is necessarily the most powerful GUI framework.

The reason is:

> It allows learning GUI programming without introducing another major language ecosystem.

The application remains conceptually:

```text
Go
 ├── P2P networking
 ├── storage
 ├── protocol
 ├── application logic
 └── GUI
```

rather than:

```text
Go backend
     +
JavaScript
     +
TypeScript
     +
React
     +
bundler
     +
CSS
     +
component library
     +
desktop wrapper
```

---

# 9. Alternative GUI Technologies

## 9.1 Gio

Gio is another Go GUI framework.

https://gioui.org/

It is worth studying later because it exposes concepts closer to rendering and immediate-mode GUI programming.

Use it if the project eventually requires:

* highly customized rendering,
* custom visualizations,
* unusual interactions,
* more control over drawing.

---

## 9.2 Wails

Wails allows Go applications to use a web-based frontend.

https://wails.io/

Architecture:

```text
Go
│
├── P2P system
│
└── Wails
      │
      └── HTML/CSS/JS UI
```

This provides excellent flexibility for complex visualizations.

However, it introduces the web frontend ecosystem that this project is deliberately trying to avoid initially.

---

# 10. GUI Architecture

The recommended architecture is:

```text
┌──────────────────────────────────────────────┐
│                 Desktop UI                   │
│                                              │
│ Network │ Peers │ Content │ Node │ Events    │
└──────────────────────┬───────────────────────┘
                       │
                 Application API
                       │
┌──────────────────────▼───────────────────────┐
│                  P2P Node                    │
│                                              │
│ Peer Manager                                 │
│ Connection Manager                           │
│ Protocol Engine                              │
│ Block Store                                  │
│ Content Addressing                           │
│ Retrieval                                    │
└──────────────────────┬───────────────────────┘
                       │
                 Persistent TCP
                       │
          ┌────────────┼────────────┐
          ▼            ▼            ▼
        Peer A       Peer B       Peer C
```

The GUI is a consumer of node state.

---

# 11. Application Layer Interface

The GUI should not directly access internal fields.

Instead, define interfaces such as:

```go
type NodeInspector interface {
    NodeInfo() NodeInfo
    Peers() []PeerInfo
    Connections() []ConnectionInfo
    Blocks() []BlockInfo
    NetworkStats() NetworkStats
}
```

For operations:

```go
type NodeController interface {
    Start() error
    Stop() error

    Connect(peer PeerAddress) error
    Disconnect(peerID PeerID) error

    RequestBlock(cid CID) error
}
```

For events:

```go
type EventSource interface {
    Events() <-chan Event
}
```

The GUI then depends on:

```text
NodeInspector
NodeController
EventSource
```

rather than:

```text
TCPConnection
PacketParser
Socket
BlockStoreInternal
PeerTableInternal
```

---

# 12. Core GUI Screens

The initial application should contain five major views:

```text
┌──────────────────────────────────────────────┐
│ P2P Explorer                                 │
├─────────────┬────────────────────────────────┤
│             │                                │
│ Network     │                                │
│             │                                │
│ Peers       │       Main View                │
│             │                                │
│ Content     │                                │
│             │                                │
│ Node        │                                │
│             │                                │
│ Events      │                                │
└─────────────┴────────────────────────────────┘
```

The five views are:

1. Network
2. Peers
3. Content
4. Node
5. Events

---

# 13. Screen 1 — Network

## Purpose

The Network screen is the primary visualization.

It represents:

* nodes,
* peers,
* connections,
* connection states,
* network relationships.

Example:

```text
                 ┌───────────┐
                 │  Peer B   │
                 └─────┬─────┘
                       │
                       │ TCP
                       │
              ┌────────▼────────┐
              │    Local Node   │
              └───────┬─────────┘
                     / \
                    /   \
                   /     \
                  ▼       ▼
            ┌────────┐ ┌────────┐
            │ Peer C │ │ Peer D │
            └────────┘ └────────┘
```

---

# 14. Network Visualization Data

The GUI should receive a graph.

Conceptually:

```go
type NetworkGraph struct {
    Nodes []GraphNode
    Edges []GraphEdge
}
```

Node:

```go
type GraphNode struct {
    ID     string
    Address string
    Status NodeStatus
}
```

Edge:

```go
type GraphEdge struct {
    Source string
    Target string
    State  ConnectionState
}
```

The UI transforms this data into visual elements.

---

# 15. Network Visualization States

A peer can have states such as:

```text
DISCONNECTED
CONNECTING
CONNECTED
FAILED
```

The UI should represent these states visually.

For example:

```text
CONNECTED
    ●

CONNECTING
    ◌

FAILED
    ×

DISCONNECTED
    ○
```

The exact visual design is less important than making state obvious.

---

# 16. Screen 2 — Peers

The Peers screen provides a structured representation of network participants.

Example:

```text
Peer ID          Address             Status
──────────────────────────────────────────────
12D3Koo...       192.168.1.20:7001   CONNECTED
12D4Koo...       192.168.1.21:7001   CONNECTED
12D5Koo...       192.168.1.22:7001   CONNECTING
```

Selecting a peer displays detailed information.

```text
Peer Information
─────────────────────────────

Peer ID:
12D3KooW...

Address:
192.168.1.20:7001

Connection:
ESTABLISHED

Uptime:
02:31:42

RTT:
14 ms

Bytes Sent:
1.2 GB

Bytes Received:
843 MB

Messages Sent:
182381

Messages Received:
179234
```

---

# 17. Why Peer Inspection Matters

The peer screen allows the GUI to expose the underlying distributed system.

Instead of seeing:

```text
"Network is working"
```

the developer can inspect:

```text
Who am I connected to?
How long has the connection existed?
How much data has been exchanged?
How many protocol messages were sent?
What is the current RTT?
```

This turns the GUI into a debugging instrument.

---

# 18. Screen 3 — Content Explorer

Because the system is content-addressed, content should be represented using its content identifier.

Example:

```text
CID:
bafybeigd...
```

The Content screen allows searching for a CID.

Example:

```text
┌──────────────────────────────────────────┐
│ CID                                      │
│ [ bafybeigd...                     ]     │
│                                          │
│ [ Search ]                               │
└──────────────────────────────────────────┘
```

Result:

```text
Content
──────────────────────────────

CID:
bafybeigd...

Size:
4.8 MB

Local:
YES

Providers:
Peer A
Peer C
Peer F
```

---

# 19. Content Provider Visualization

A content object can be represented as:

```text
                CID
                 │
       ┌─────────┼─────────┐
       │         │         │
       ▼         ▼         ▼
    Peer A     Peer C     Peer F
      ✓           ✓          ✓
```

This answers a fundamental question:

> "Where does this content currently exist?"

This is more meaningful for an IPFS-like system than a traditional file-system directory tree.

---

# 20. Content Retrieval Visualization

When retrieving content:

```text
Local Node
    │
    │ GET_BLOCK
    ▼
 Peer C
    │
    │ BLOCK
    ▼
Local Node
    │
    ▼
Verify Hash
    │
    ▼
Store Block
```

The GUI should eventually show this as an event sequence.

---

# 21. Screen 4 — Node

The Node screen displays local node information.

Example:

```text
LOCAL NODE
──────────────────────────────

Node ID
12D3KooW...

Listening Address
0.0.0.0:7001

Status
RUNNING

Uptime
04:21:31

Connected Peers
7

Stored Blocks
1,284

Storage Used
2.8 GB
```

---

# 22. Node Network Statistics

The Node screen can also display:

```text
Network
──────────────────────────────

Bytes Sent        1.24 GB
Bytes Received    2.81 GB

Requests Sent     18,293
Requests Received 21,982

Successful        20,381
Failed            1,601

Average RTT       23 ms
```

---

# 23. Screen 5 — Event Stream

The event stream is one of the most useful components during development.

Example:

```text
17:42:01  PEER_CONNECTED
          peer=12D3Koo...

17:42:02  GET_BLOCK
          cid=bafy...

17:42:02  BLOCK_RECEIVED
          peer=12D3Koo...

17:42:02  BLOCK_VERIFIED
          cid=bafy...

17:42:03  BLOCK_STORED
          cid=bafy...

17:42:04  PEER_DISCONNECTED
          peer=12D4Koo...
```

This provides a human-readable representation of the protocol.

---

# 24. Event Model

Define a common event type.

```go
type Event struct {
    Time    time.Time
    Type    EventType
    PeerID  string
    CID     string
    Message string
}
```

Possible event types:

```text
NODE_STARTED
NODE_STOPPED

PEER_CONNECTED
PEER_DISCONNECTED
PEER_CONNECTION_FAILED

GET_BLOCK
BLOCK_RECEIVED
BLOCK_VERIFIED
BLOCK_STORED

BLOCK_REQUEST_FAILED

REPLICATION_STARTED
REPLICATION_COMPLETED
REPLICATION_FAILED
```

The exact list should evolve with the protocol.

---

# 25. GUI State

One of the most important concepts to learn is **application state**.

The UI should have a representation of what it currently knows.

For example:

```go
type AppState struct {
    Node       NodeInfo
    Peers      []PeerInfo
    Connections []ConnectionInfo
    Events     []Event
    Stats      NetworkStats
}
```

The application receives updates:

```text
P2P Node
   │
   │ event
   ▼
Application State
   │
   │ state changed
   ▼
GUI redraw
```

This is a fundamental GUI programming concept.

---

# 26. Event-Driven GUI Model

A GUI is fundamentally event-driven.

Instead of:

```text
main()
  ↓
draw everything once
  ↓
exit
```

the application behaves approximately like:

```text
          ┌─────────────────────┐
          │     Event Loop      │
          └──────────┬──────────┘
                     │
       ┌─────────────┼─────────────┐
       ▼             ▼             ▼
   Mouse Event    Key Event    Node Event
       │             │             │
       └─────────────┼─────────────┘
                     ▼
               Update State
                     │
                     ▼
                  Redraw
```

Examples of events:

```text
Button clicked
Mouse moved
Window resized
Keyboard pressed
Peer connected
Block received
TCP connection failed
Timer fired
```

---

# 27. Concurrency Problem

The P2P system will almost certainly use goroutines.

For example:

```go
go connectionReader()
go connectionWriter()
go peerManager()
go replicationWorker()
```

The GUI also has its own execution model.

Therefore, avoid directly modifying UI state from arbitrary networking goroutines.

Bad conceptual model:

```text
TCP goroutine
     │
     └──────► modify GUI widget
```

Better:

```text
TCP goroutine
     │
     ▼
Event channel
     │
     ▼
Application state
     │
     ▼
GUI update
```

Example:

```go
events := make(chan Event)
```

Networking:

```go
events <- Event{
    Type:   PeerConnected,
    PeerID: peer.ID,
}
```

Application layer consumes the event and updates state.

---

# 28. Thread-Safety Principle

The following rule should guide the implementation:

> **Networking code owns networking state. The GUI owns presentation state.**

The GUI should not directly manipulate internal peer maps.

Instead:

```text
Peer Manager
     │
     │ snapshot
     ▼
Application Model
     │
     ▼
GUI
```

---

# 29. Data Flow

The complete data flow should look like:

```text
                TCP NETWORK
                    │
                    ▼
              P2P Node
                    │
          ┌─────────┴─────────┐
          │                   │
       State                Events
          │                   │
          ▼                   ▼
     Node Inspector      Event Channel
          │                   │
          └─────────┬─────────┘
                    ▼
             Application Model
                    │
                    ▼
                   GUI
```

---

# 30. Basic Node Lifecycle

The application should support:

```text
START
  │
  ▼
INITIALIZE
  │
  ▼
START NODE
  │
  ▼
LISTEN FOR TCP
  │
  ▼
DISCOVER/CONNECT PEERS
  │
  ▼
RUNNING
  │
  ├───────────────┐
  │               │
  ▼               ▼
STOP           FAILURE
  │               │
  ▼               ▼
SHUTDOWN       RECOVER
```

The GUI should expose at least:

```text
Start
Stop
Restart
```

in the initial version.

---

# 31. Implementation Stages

Do not implement the entire interface at once.

The project should be developed incrementally.

---

# Stage 1 — First Window

Goal:

Learn the absolute basics of GUI programming.

Implement:

```text
Window
 ├── Title
 ├── Start button
 ├── Stop button
 └── Status label
```

Example:

```text
┌─────────────────────────────┐
│ P2P Explorer                │
├─────────────────────────────┤
│                             │
│ Status: STOPPED             │
│                             │
│ [ Start Node ]              │
│ [ Stop Node  ]              │
│                             │
└─────────────────────────────┘
```

Learning objectives:

* Window creation.
* Widgets.
* Buttons.
* Callbacks.
* Layout.
* Application lifecycle.

---

# Stage 2 — Connect GUI to Node

Replace fake state with the actual P2P node.

```text
[ Start Node ]
      │
      ▼
node.Start()
      │
      ▼
Status = RUNNING
```

The GUI should display:

```text
Status: RUNNING
Node ID: 12D3Koo...
Address: :7001
```

Learning objectives:

* Calling Go application logic from GUI callbacks.
* Separating UI and application code.
* Handling errors.

---

# Stage 3 — Peer List

Add:

```text
Peers
```

Example:

```text
Peers
──────────────────────────────

12D3Koo...    CONNECTED
12D4Koo...    CONNECTED
12D5Koo...    CONNECTING
```

Learning objectives:

* Lists/tables.
* Updating widgets.
* Application state.
* Periodic refresh.

---

# Stage 4 — Peer Details

Selecting a peer displays:

```text
Peer ID
Address
Status
RTT
Uptime
Bytes TX
Bytes RX
```

Learning objectives:

* Selection state.
* Dynamic views.
* Master/detail interfaces.

---

# Stage 5 — Event Stream

Add a live event log.

```text
┌─────────────────────────────────────────────┐
│ Events                                      │
├─────────────────────────────────────────────┤
│ PEER_CONNECTED 12D3...                      │
│ GET_BLOCK      bafy...                      │
│ BLOCK_RECEIVED bafy...                      │
│ BLOCK_STORED   bafy...                      │
└─────────────────────────────────────────────┘
```

Learning objectives:

* Channels.
* Asynchronous events.
* GUI updates from background operations.
* State synchronization.

---

# Stage 6 — Content Explorer

Implement:

```text
CID search
```

Then:

```text
CID
Size
Hash
Local status
Providers
```

Learning objectives:

* User input.
* Search.
* Data retrieval.
* Error states.

---

# Stage 7 — Network Topology

Only after the previous stages work.

Represent:

```text
Peers = vertices
TCP connections = edges
```

Graph:

```text
A ─── B
│   ╱
│  ╱
C ─── D
```

Learning objectives:

* Graph data structures.
* Rendering.
* Coordinate systems.
* Layout algorithms.
* Mouse interaction.
* Selection.
* Zooming/panning.

---

# Stage 8 — Interactive Topology

Allow:

* selecting nodes,
* dragging nodes,
* zooming,
* panning,
* inspecting peers,
* showing connection state.

Eventually:

```text
Click Peer
    │
    ▼
Peer information panel
```

---

# Stage 9 — Content/Network Correlation

This is where the GUI becomes particularly useful.

Show:

```text
CID
 │
 ├── Peer A
 ├── Peer C
 └── Peer F
```

Then retrieval:

```text
Local
  │
  └── GET_BLOCK
         │
         ▼
       Peer C
         │
         ▼
       BLOCK
         │
         ▼
       Local
```

The GUI becomes a visual representation of the distributed storage algorithm.

---

# 32. Suggested Final Layout

A possible final interface:

```text
┌──────────────────────────────────────────────────────────┐
│ P2P Explorer                         ● RUNNING            │
├──────────────┬───────────────────────────────────────────┤
│              │                                           │
│ NETWORK      │                                           │
│              │                                           │
│ PEERS        │              NETWORK GRAPH                │
│              │                                           │
│ CONTENT      │          ●──────────●                     │
│              │         /            \                    │
│ NODE         │        ●              ●                   │
│              │         \            /                    │
│ EVENTS       │          ─────●─────                      │
│              │                                           │
├──────────────┴───────────────────────────────────────────┤
│ Event Stream                                             │
│ 17:42:01 PEER_CONNECTED 12D3...                          │
│ 17:42:02 GET_BLOCK bafy...                               │
│ 17:42:02 BLOCK_RECEIVED                                  │
└──────────────────────────────────────────────────────────┘
```

---

# 33. Observability

The GUI should not attempt to replace Grafana.

A useful architecture is:

```text
                         P2P NODE
                            │
             ┌──────────────┴──────────────┐
             │                             │
             ▼                             ▼
       Desktop Explorer                Metrics
             │                             │
             │                         Prometheus
             │                             │
             │                          Grafana
             │
       Interactive state
```

The desktop UI handles:

* topology,
* peer inspection,
* CID exploration,
* live events,
* node control.

Grafana handles:

* historical metrics,
* throughput,
* latency,
* failure rates,
* resource usage,
* long-term trends.

---

# 34. Metrics Worth Exposing

The node should eventually expose metrics such as:

```text
active_peers
active_connections

bytes_sent_total
bytes_received_total

messages_sent_total
messages_received_total

block_requests_total
block_requests_failed_total

blocks_received_total
blocks_stored_total

connection_attempts_total
connection_failures_total

block_request_latency_seconds

storage_bytes_used
storage_blocks_total
```

This gives Grafana meaningful data.

---

# 35. GUI vs Grafana Responsibility

| Requirement           | Desktop UI |  Grafana |
| --------------------- | ---------: | -------: |
| P2P topology          |        Yes | Poor fit |
| Peer inspection       |        Yes | Poor fit |
| CID explorer          |        Yes |       No |
| Protocol events       |        Yes |  Limited |
| Start/stop node       |        Yes |       No |
| Connect peer          |        Yes |       No |
| Historical throughput |    Limited |      Yes |
| Historical latency    |    Limited |      Yes |
| Long-term metrics     |         No |      Yes |
| Alerts                |      Later |      Yes |
| Resource monitoring   |      Basic |      Yes |

The two systems complement each other.

---

# 36. Suggested Project Structure

A possible repository structure:

```text
project/
│
├── cmd/
│   ├── node/
│   │   └── main.go
│   │
│   └── explorer/
│       └── main.go
│
├── internal/
│   ├── p2p/
│   │   ├── peer.go
│   │   ├── connection.go
│   │   └── manager.go
│   │
│   ├── protocol/
│   │   ├── message.go
│   │   ├── encoder.go
│   │   └── decoder.go
│   │
│   ├── storage/
│   │   ├── block.go
│   │   └── store.go
│   │
│   ├── content/
│   │   └── cid.go
│   │
│   └── node/
│       └── node.go
│
├── pkg/
│   └── api/
│       ├── inspector.go
│       ├── controller.go
│       └── events.go
│
├── ui/
│   └── fyne/
│       ├── app.go
│       ├── network.go
│       ├── peers.go
│       ├── content.go
│       ├── node.go
│       └── events.go
│
└── docs/
    └── gui-srs.md
```

The exact structure can change.

The important separation is:

```text
P2P implementation
        ≠
GUI implementation
```

---

# 37. Important GUI Concepts to Learn

The project should be treated as a way to learn these concepts rather than merely learning Fyne APIs.

## 37.1 Event Loop

Understand:

```text
input
 ↓
event
 ↓
handler
 ↓
state change
 ↓
render
```

---

## 37.2 State

Understand that the GUI represents application state.

Example:

```text
selectedPeer = Peer A
nodeStatus = RUNNING
currentScreen = PEERS
```

---

## 37.3 Layout

Learn how UI elements are positioned.

Examples:

```text
Vertical
Horizontal
Grid
Split panes
Containers
Scrollable areas
```

---

## 37.4 Rendering

Eventually understand:

```text
Application state
       ↓
Widget tree
       ↓
Layout
       ↓
Rendering
       ↓
Window
       ↓
Operating system
```

---

## 37.5 Input

Learn how applications react to:

* mouse clicks,
* mouse movement,
* keyboard input,
* scrolling,
* dragging,
* resizing.

This becomes especially important for the network graph.

---

## 37.6 Concurrency

Your application contains two fundamentally different workloads:

```text
GUI
│
└── event/render loop

P2P system
│
├── network goroutines
├── TCP readers
├── TCP writers
├── peer manager
└── storage workers
```

Understanding how these interact is one of the technically valuable parts of this project.

---

# 38. Learning Progression

A useful progression is:

```text
GUI basics
     ↓
Widgets
     ↓
Layouts
     ↓
Callbacks
     ↓
Application state
     ↓
Asynchronous events
     ↓
Concurrency
     ↓
Custom rendering
     ↓
Graph visualization
```

Do not skip directly to graph rendering.

---

# 39. Development Philosophy

The GUI should evolve together with the P2P system.

For every major backend feature, consider:

> "How would I observe this happening?"

For example:

### Persistent TCP connection

Backend:

```text
ConnectionManager
```

GUI:

```text
Peer A ───────── Peer B
         TCP
```

### Block retrieval

Backend:

```text
GET_BLOCK
```

GUI:

```text
GET_BLOCK
    │
    ▼
Peer B
    │
    ▼
BLOCK_RECEIVED
```

### Peer failure

Backend:

```text
connection lost
```

GUI:

```text
Peer B
  ↓
CONNECTED
  ↓
DISCONNECTED
```

### Replication

Backend:

```text
Block X
Peer A
Peer B
```

GUI:

```text
          Block X
          /     \
       Peer A  Peer B
```

This creates a direct relationship between systems engineering and GUI development.

---

# 40. Testing Strategy

The GUI should not be tested only manually.

Separate:

```text
P2P logic
GUI logic
```

P2P tests should run without a GUI.

GUI tests should be able to operate using fake node state.

For example:

```go
type FakeNode struct {
    peers []PeerInfo
}
```

The GUI can then be tested with:

```text
Fake Node
    ↓
Application Model
    ↓
GUI
```

instead of requiring a real P2P network for every test.

---

# 41. Development Mode

A particularly useful feature is a simulated network.

For example:

```text
Fake Network

        A
       / \
      B---C
       \ /
        D
```

The GUI can then simulate:

```text
Peer connected
Peer disconnected
Block retrieved
Block failed
Node failed
Node recovered
```

This allows GUI development without requiring multiple physical machines.

---

# 42. Multi-Node Demonstration

Eventually run:

```text
node1
node2
node3
node4
```

on one machine using different ports:

```text
node1 :7001
node2 :7002
node3 :7003
node4 :7004
```

The desktop explorer connects to one node.

The UI should then display:

```text
              Node2
               │
               │
        Node1 ─┼── Node3
               │
               │
              Node4
```

This creates a practical demonstration of the P2P architecture.

---

# 43. Future Extensions

After the basic GUI works, possible extensions include:

## Network simulation

Allow:

```text
Create peer
Remove peer
Disconnect peer
Inject failure
Add latency
Limit bandwidth
```

This turns the GUI into a distributed-systems experimentation tool.

---

## Protocol inspector

Show raw protocol activity:

```text
OUT → GET_BLOCK
CID: bafy...

IN  ← BLOCK
CID: bafy...
SIZE: 4096
```

Potentially include encoded message sizes and timestamps.

---

## Connection inspector

Display:

```text
TCP
State
Local address
Remote address
Connection age
RTT
TX
RX
```

---

## Storage inspector

Display:

```text
Blocks
CID
Size
Created
Last Accessed
Reference Count
```

---

## Network replay

Record events:

```text
t=0    Peer A connected
t=2    GET_BLOCK
t=3    BLOCK_RECEIVED
t=5    Peer B disconnected
```

Then replay them in the GUI.

This could become an interesting distributed-systems debugging feature.

---

# 44. What Not to Do

Avoid turning the project into a generic dashboard.

Do not spend most of the project implementing:

```text
rounded cards
fancy gradients
animated buttons
login pages
settings screens
theme systems
```

Those do not contribute much to the purpose of the project.

Prioritize:

```text
network visualization
peer state
protocol events
content relationships
node state
```

---

# 45. Recommended First Milestone

The first meaningful milestone should be extremely small:

```text
┌─────────────────────────────────┐
│ P2P Explorer                    │
├─────────────────────────────────┤
│                                 │
│ Node: RUNNING                   │
│ ID: 12D3Koo...                  │
│                                 │
│ Peers: 2                        │
│                                 │
│ [ Refresh ]                     │
│                                 │
└─────────────────────────────────┘
```

Nothing more.

Once this works:

```text
GUI
 ↓
Node
 ↓
TCP
 ↓
Other nodes
```

the architecture has been proven.

---

# 46. Final Target

The eventual application should feel like a **debugger for a P2P storage network** rather than a conventional file-management application.

Conceptually:

```text
┌─────────────────────────────────────────────────────────┐
│ P2P EXPLORER                           ● NODE RUNNING   │
├────────────┬────────────────────────────────────────────┤
│            │                                            │
│ NETWORK    │                 P2P TOPOLOGY               │
│            │                                            │
│ PEERS      │                   ●────●                   │
│            │                  /      \                  │
│ CONTENT    │                 ●        ●                 │
│            │                  \      /                  │
│ NODE       │                   ──●──                     │
│            │                                            │
│ EVENTS     │                                            │
│            ├────────────────────────────────────────────┤
│            │ LIVE EVENTS                                │
│            │                                            │
│            │ PEER_CONNECTED 12D3...                    │
│            │ GET_BLOCK      bafy...                    │
│            │ BLOCK_RECEIVED bafy...                    │
│            │ PEER_DISCONNECTED 12D4...                 │
└────────────┴────────────────────────────────────────────┘
```

The GUI is therefore not an independent side project.

It becomes a **visual interface to the distributed system you are already building**.

---

# 47. Mental Model for Reviewing This Document Later

If this project is revisited after several months, remember the following five ideas first:

```text
1. The P2P node is the actual system.
             │
             ▼
2. The GUI observes and controls the node.
             │
             ▼
3. Node state is exposed through interfaces.
             │
             ▼
4. Events flow from networking → application state → GUI.
             │
             ▼
5. The GUI visualizes peers, content, and protocol activity.
```

The most important architectural boundary is:

```text
              APPLICATION
                   │
       ┌───────────┴───────────┐
       │                       │
   P2P SYSTEM                GUI
       │                       │
   TCP / Storage          Visualization
   Protocol               Interaction
   Peers                  Presentation
   Content                State
```

The GUI should never become responsible for implementing the distributed system.

---

# 48. Implementation Checklist

## Foundation

* [ ] Choose Fyne as initial GUI toolkit.
* [ ] Create separate `cmd/explorer`.
* [ ] Create basic application window.
* [ ] Add basic layout.
* [ ] Add Start/Stop controls.
* [ ] Connect GUI to node lifecycle.

## Node Integration

* [ ] Define `NodeInspector`.
* [ ] Define `NodeController`.
* [ ] Define `EventSource`.
* [ ] Expose node information.
* [ ] Expose peer information.
* [ ] Expose connection information.
* [ ] Expose network statistics.

## Peer UI

* [ ] Peer list.
* [ ] Peer selection.
* [ ] Peer details.
* [ ] Connection state.
* [ ] RTT.
* [ ] Traffic statistics.

## Event System

* [ ] Define event types.
* [ ] Create event channel.
* [ ] Receive networking events.
* [ ] Maintain application event history.
* [ ] Display live events.

## Content

* [ ] CID input.
* [ ] CID lookup.
* [ ] Block information.
* [ ] Local/remote status.
* [ ] Provider list.
* [ ] Retrieval history.

## Network Visualization

* [ ] Graph model.
* [ ] Peer nodes.
* [ ] Connection edges.
* [ ] Node selection.
* [ ] Zoom.
* [ ] Pan.
* [ ] Dragging.
* [ ] Connection state visualization.

## Observability

* [ ] Metrics endpoint.
* [ ] Prometheus integration.
* [ ] Grafana dashboard.
* [ ] Network throughput.
* [ ] Request latency.
* [ ] Connection failures.
* [ ] Block retrieval statistics.

## Testing

* [ ] Fake node implementation.
* [ ] Fake peer network.
* [ ] GUI development mode.
* [ ] Multi-node local test.
* [ ] Failure simulation.

---

# 49. Definition of Success

The project is successful when the following scenario can be demonstrated:

```text
1. Start four P2P nodes.

2. Establish persistent TCP connections.

3. Open the desktop explorer.

4. Observe the four-node topology.

5. Select a peer and inspect its connection.

6. Store a content-addressed block.

7. Observe the block being stored.

8. Request the block from another node.

9. Observe GET_BLOCK.

10. Observe BLOCK_RECEIVED.

11. Observe hash verification.

12. Observe the content provider relationship.

13. Disconnect a peer.

14. Observe the topology change.

15. Observe the corresponding protocol events.

16. Open Grafana and inspect the historical
    network/throughput/latency metrics.
```

At that point the GUI is no longer merely decoration.

It provides an observable, interactive representation of the distributed system itself.
