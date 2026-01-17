# Clipboard Synchronization Tool (P2P Edition)

An end-to-end encrypted, cross-platform, peer-to-peer clipboard synchronization tool using WebRTC.

## Architecture

```
┌─────────────┐                     ┌─────────────┐
│   Client A  │◄────DataChannel────►│   Client B  │
│  (WebRTC)   │    (encrypted P2P)  │  (WebRTC)   │
└──────┬──────┘                     └──────┬──────┘
       │                                   │
       │ WebSocket (signaling only)        │
       │ ┌───────────────────────────────┐ │
       └─►       Signaling Server        ◄─┘
         │  (offers, answers, ICE)       │
         └───────────────────────────────┘
```

**Key Points:**
- Clipboard data transfers **directly between devices** (P2P) via WebRTC DataChannels
- Server only handles signaling for peer discovery and connection establishment
- All clipboard content is encrypted with AES-256-GCM before transmission
- NAT traversal handled via STUN servers

## Project Structure
- `cmd/` - Entry points for the applications
  - `client/main.go` - P2P clipboard sync client
  - `server/main.go` - Signaling server
- `internal/` - Private application logic
  - `client/` - WebRTC peer connection management and clipboard sync
  - `clipboard/` - System clipboard access and echo cancellation
  - `crypto/` - AES-256-GCM encryption and key derivation
  - `signaling/` - WebRTC signaling message types
  - `wsserver/` - WebSocket hub for signaling broadcast
  - `utils/` - Utility functions

## Usage

### 1. Installation

```bash
git clone https://github.com/Pujan-khunt/clipboard-sync
cd clipboard-sync
go mod download
go build -o bin/server cmd/server/main.go
go build -o bin/client cmd/client/main.go
```

### 2. Running the Signaling Server

The server acts as a matchmaker for peer discovery. **No clipboard data flows through it.**

```bash
./bin/server                    # Default port 8080
./bin/server --port :38213      # Custom port
```

### 3. Running Clients

Each client connects to the signaling server, then establishes direct P2P connections with other peers in the same room.

```bash
# Required: password for E2E encryption
./bin/client -password=mysecret

# Full options
./bin/client \
  -server=ws://your-server:8080/ws?room=myroom \
  -password=mysecret \
  -peerID=device1  # Optional, auto-generated if not specified
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-server` | Signaling server URL with room | `ws://localhost:8080/ws?room=default` |
| `-password` | E2E encryption password (required) | - |
| `-peerID` | Unique device identifier | Auto-generated UUID |

### 4. Multi-Device Synchronization

```bash
# Terminal 1: Start signaling server
./bin/server

# Terminal 2: Client on device A
./bin/client -password=test123 -server=ws://SERVER_IP:8080/ws?room=home

# Terminal 3: Client on device B
./bin/client -password=test123 -server=ws://SERVER_IP:8080/ws?room=home

# Copy text on either device → appears on the other
# Even if the server is stopped, P2P sync continues!
```

## Security

- **End-to-End Encryption**: All clipboard content encrypted with AES-256-GCM
- **Zero-Knowledge Server**: Server never sees clipboard data
- **Direct P2P Transfer**: Data travels directly between devices
- **Password-Based**: Devices must share the same password to decrypt

## NAT Traversal

The client uses Google's public STUN servers for NAT traversal:
- `stun:stun.l.google.com:19302`
- `stun:stun1.l.google.com:19302`

For restrictive firewalls (symmetric NAT), you may need a TURN server.

## Platform Support

- **Linux**: Full clipboard support via X11/XWayland
- **macOS**: Full clipboard support via native APIs
- **Windows**: Full clipboard support via Win32 APIs

## Dependencies

- [pion/webrtc](https://github.com/pion/webrtc) - WebRTC implementation for Go
- [golang.design/x/clipboard](https://pkg.go.dev/golang.design/x/clipboard) - Cross-platform clipboard access
- [gorilla/websocket](https://pkg.go.dev/github.com/gorilla/websocket) - WebSocket for signaling
