package main

import (
	"flag"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", ":8080", "Port to listen on")

// Specify parameters for converting a standard HTTP GET request into a WebSocket connection.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections (for development)
	},
}

// Hub manages all active "rooms" (groups of devices)
// Map Key: Room ID (string), Value: List of all connected devices in the room
type Hub struct {
	rooms map[string]map[*websocket.Conn]bool
	mu    sync.Mutex // Protects the map from concurrent access
}

var hub = Hub{
	rooms: make(map[string]map[*websocket.Conn]bool),
}

func main() {
	// Parse the CLI flags
	flag.Parse()

	// Define the main websocket endpoint
	http.HandleFunc("/ws", handleConnections)

	// Start the server
	log.Printf("Server started on %s\n", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade initial GET request to a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Identify the "Room" (Device Group)
	// Example URL: ws://localhost:8080/ws?room=my_devices
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		roomID = "default"
	}

	// Register the new client
	hub.mu.Lock()
	// Initialize an empty room if not already created.
	if hub.rooms[roomID] == nil {
		hub.rooms[roomID] = make(map[*websocket.Conn]bool)
	}
	hub.rooms[roomID][conn] = true
	hub.mu.Unlock()

	log.Printf("New device connected to room: %s", roomID)

	// Cleanup when the client disconnects
	defer func() {
		hub.mu.Lock()
		delete(hub.rooms[roomID], conn)
		hub.mu.Unlock()
		conn.Close()
		log.Printf("Device disconnected from room: %s", roomID)
	}()

	// The read loop (wait for clipboard updates)
	for {
		// ReadMessage will block until the data arrives or the connection closes
		// in which case it will return a non-nil error.
		messageType, msg, err := conn.ReadMessage() // messageType will be either websocket.TextMessage(for text) or websocket.BinaryMessage(for images)
		if err != nil {
			break
		}

		// Broadcast to every device in the same room except the sender.
		broadcastToRoom(roomID, conn, messageType, msg)
	}
}

func broadcastToRoom(roomID string, sender *websocket.Conn, messageType int, msg []byte) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	for client := range hub.rooms[roomID] {
		// Send message to everyone in the same room, EXCEPT the sender.
		if client == sender {
			continue
		}

		// Send the data
		err := client.WriteMessage(messageType, msg)
		if err != nil {
			log.Printf("Error writing to client: %v", err)
			client.Close()
			delete(hub.rooms[roomID], client)
		}
	}
}
