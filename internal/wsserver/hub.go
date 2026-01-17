// Package wsserver implements the WebSocket signaling server for P2P clipboard sync.
// It functions as a matchmaker that accepts connections, manages rooms for device
// discovery, and broadcasts signaling messages (offers, answers, ICE candidates)
// between clients. No clipboard data flows through this server.
package wsserver

import (
	"log"
	"net/http"
	"sync"

	"github.com/Pujan-khunt/clipboard-sync/internal/signaling"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Hub manages rooms and client connections.
type Hub struct {
	rooms map[string]map[string]*websocket.Conn // stores all the connected clients within the same room.
	mu    sync.Mutex                            // Protects the map from concurrent access.
}

// NewHub creates a new thread-safe hub.
func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]map[string]*websocket.Conn),
	}
}

func (h *Hub) HandleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade the connection from HTTP GET request to a WebSocket connection.
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// Identify the room
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "default"
	}

	// Identify the peer
	peerID := r.URL.Query().Get("peer_id")
	if peerID == "" {
		log.Println("Connection rejected: Missing peer_id")
		ws.Close()
		return
	}

	// Register the client with their peer id
	h.mu.Lock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[string]*websocket.Conn)
	}
	h.rooms[roomID][peerID] = ws
	h.mu.Unlock()

	log.Printf("[Room: %s] Peer: %s connected", roomID)

	// Cleanup on exit
	defer func() {
		h.mu.Lock()
		if _, ok := h.rooms[roomID][peerID]; ok {
			delete(h.rooms[roomID], peerID)
			// Cleanup empty rooms
			if len(h.rooms[roomID]) == 0 {
				delete(h.rooms, roomID)
			}
		}
		h.mu.Unlock()
		ws.Close()
		log.Printf("[Room: %s] Device disconnected", roomID)
	}()

	// Watch for changes from client and broadcast them.
	for {
		messageType, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		h.broadcast(roomID, ws, messageType, msg)
	}
}

func (h *Hub) broadcast(roomID string, sender *websocket.Conn, messageType int, msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Try and parse the signalling message to check for specific target requirements.
	signallingMsg, err := signaling.Unmarshal(msg)

	// If parsing succeeds and ToPeer is set, then only send the message to that peer.
	if err == nil && signallingMsg.ToPeer != "" {
		if targetConn, exists := h.rooms[roomID][signallingMsg.ToPeer]; exists {
			if err := targetConn.WriteMessage(messageType, msg); err != nil {
				log.Printf("Write error to %s: %v", signallingMsg.ToPeer, err)
				targetConn.Close()
				delete(h.rooms[roomID], signallingMsg.ToPeer)
			}

		}
		return
	}

	// If no specific target, send to everyone (except sender)
	for _, client := range h.rooms[roomID] {
		if client == sender {
			continue
		}
		if err := client.WriteMessage(messageType, msg); err != nil {
			log.Printf("Write error: %v", err)
			client.Close()
		}
	}
}
