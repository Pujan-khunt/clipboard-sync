# Clipboard Synchronization Tool

An end-to-end encrypted, cross-platform and realtime clipboard synchronization tool.

## Project Structure
- `cmd/` - Entry points for the applications.
  - `client/main.go` - parses flags and starts the client app.
  - `server/main.go` - parses flags and starts the server.
- `internal/` - Private application logic.
  - `client/` - Core client application logic and main loop.
  - `clipboard/` - System clipboard access and echo cancellation logic.
  - `crypto/` - AES-256-GCM encryption and key derivation.
  - `wsserver/` - WebSocket hub and room management logic.
  - `utils` - Utility functions.

## Usage

### 1. Installation

1. Clone the repository:
```bash
git clone https://github.com/Pujan-khunt/clipboard-sync
cd clipboard-sync
```

2. Download dependencies:
```bash
go mod download
```

3. Build the binaries:
```bash
# Build both server and client
go build -o bin/server cmd/server/main.go
go build -o bin/client cmd/client/main.go

# Or run directly without building
go run cmd/server/main.go
go run cmd/client/main.go
```

### 2. Running the Server

The server acts as a WebSocket signaling hub that manages rooms and broadcasts clipboard updates to connected clients.

```bash
# Start the server on default port 8080
./bin/server

# Or start it on a custom port 38213
./bin/server --addr :38213
```

The server:
- Exposes WebSocket endpoint at `/ws`
- Manages rooms for grouping devices
- Broadcasts clipboard changes to all clients in the same room

**Server Options:**
- Port: `--addr :<port>` (default: 8080)
- WebSocket path: `/ws`
- Default room: `"default"`

### 3. Running the Client

The client monitors the system clipboard and synchronizes changes with other clients in the same room.

```bash
# Start with default room (uses ws://localhost:8080/ws?room=default)
./bin/client

# Output:
# >> Clipboard Watcher Started. Copy text anywhere to see it here.
# >> [CAPTURED] Size: 15 bytes | Content: "Hello World"
```

### 4. Connecting Devices (Room-Based Synchronization)

Devices are grouped into "rooms" for synchronization. Multiple rooms can be used to separate device groups.

```bash
# Connect to a specific room (replace SERVER_IP with actual server IP)
./bin/client ws://SERVER_IP:8080/ws?room=my_devices

# Connect to a different room for separate synchronization
./bin/client ws://SERVER_IP:8080/ws?room=work_devices
```

**URL Parameters:**
- `room`: Room identifier (alphanumeric, defaults to "default")
- All clients in the same room receive clipboard updates from each other

### Examples

#### Example 1: Local Development Testing
```bash
# Terminal 1: Start server
./bin/server

# Terminal 2: Start first client
./bin/client ws://localhost:8080/ws?room=local_test

# Terminal 3: Start second client
./bin/client ws://localhost:8080/ws?room=local_test

# Copy text in one terminal - it appears in both clients
```

#### Example 2: Multi-Device Synchronization Over Network
```bash
# On server machine (ensure port 8080 is accessible)
./bin/server

# On laptop (connect to server's IP)
./bin/client ws://192.168.1.100:8080/ws?room=home_office

# On desktop (connect to same room)
./bin/client ws://192.168.1.100:8080/ws?room=home_office

# Copy on laptop -> appears on desktop
# Copy on desktop -> appears on laptop
```

#### Example 3: Multiple Rooms for Separate Groups
```bash
# Room for personal devices
./bin/client ws://SERVER_IP:8080/ws?room=personal

# Room for work devices
./bin/client ws://SERVER_IP:8080/ws?room=work
```

### Current Limitations

- **No encryption**: Clipboard content is transmitted in plaintext over the network
- **No TLS/HTTPS**: Uses plain HTTP/WebSocket connections
- **No authentication**: Anyone with the room name can join and receive/send clipboard data
- **No room passwords**: Rooms are not secured
- **Placeholder network logic**: Client currently has placeholder code for simulating network updates
- **No automatic reconnection**: If connection is lost, client must be restarted
- **No binary data support**: Only text clipboard content is synchronized
- **Single room per client**: Each client instance can only join one room at a time

### Platform Notes

- **Linux**: Full clipboard support via X11 and Wayland(via XWayland)
- **macOS**: Full clipboard support via native APIs
- **Windows**: Full clipboard support via Win32 APIs
- The clipboard library (`golang.design/x/clipboard`) handles platform-specific differences automatically

## Dependencies
1. [golang.design/x/clipboard](https://pkg.go.dev/golang.design/x/clipboard) for accessing the system clipboard while maintaining cross-platform support.
2. [gorilla/websocket](https://pkg.go.dev/github.com/gorilla/websocket) for using WebSocket for realtime synchronization between devices.
