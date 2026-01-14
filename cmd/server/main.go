package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

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
	// Define the main websocket endpoint
	http.HandleFunc("/ws", handleConnections)

	// Start the server
	log.Println("Server started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade initial GET request to a WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
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
	if hub.rooms[roomID] == nil {
		hub.rooms[roomID] = make(map[*websocket.Conn]bool)
	}
	hub.rooms[roomID][ws] = true
	hub.mu.Unlock()

	log.Printf("New device connected to room: %s", roomID)

	// Cleanup when the client disconnects
	defer func() {
		hub.mu.Lock()
		delete(hub.rooms[roomID], ws)
		hub.mu.Unlock()
		ws.Close()
		log.Printf("Device disconnected from room: %s", roomID)
	}()

	// The read loop (wait for clipboard updates)
	for {
		// ReadMessage will return a non-nil error when the WebSocket disconnects,
		// effectively ending this infinite loop and running the cleanup function.
		messageType, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}

		// Broadcast to every device in the same room except the sender.
		broadcastToRoom(roomID, ws, messageType, msg)
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
