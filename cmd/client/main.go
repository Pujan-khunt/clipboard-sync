package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"

	"github.com/gorilla/websocket"
	"golang.design/x/clipboard"
)

var (
	lastSyncedContent string
	mu                sync.Mutex // Mutex ensures thread-safety between Watcher and Network routines
)

var serverURL = flag.String("serverAddr", "ws://localhost:8080/ws?room=default", "Server address to connect to")

func main() {
	// Parse the CLI commands and flags
	flag.Parse()

	// Initialize the clipboard process.
	// Init can fail if the dependencies are not installed.
	err := clipboard.Init()
	if err != nil {
		log.Fatalf("Failed to initialize clipboard: %v", err)
	}
	fmt.Println(">> Clipboard Watcher Started. Copy text anywhere to see it here.")

	// Safely check if the server url is valid.
	url, err := url.Parse(*serverURL)
	if err != nil {
		log.Fatalf("Invalid URL: %v", err)
	}
	log.Printf("Connecting to %s", url.String())

	// Connect to the websocket server
	conn, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
	if err != nil {
		log.Fatal("Connection failed:", err)
	}
	defer conn.Close()
	fmt.Println("Connected! Synchronizing...")

	// Start the watcher (listener)
	// we receive a channel that will receive bytes whenever the clipboard changes with text(FmtText) data.
	clipboardWatcher := clipboard.Watch(context.Background(), clipboard.FmtText)

	// Handle incoming clipboard changes in a separate goroutine
	go func() {
		for text := range clipboardWatcher {
			clipboardData := string(text)

			// Ignore the changes that were caused by the client itself updating clipboard from changes from the server.
			mu.Lock()
			if clipboardData == lastSyncedContent {
				mu.Unlock()
				continue
			}
			// Update state to know this is the "latest" version
			lastSyncedContent = clipboardData
			mu.Unlock()

			// Send data to the WebSocket server
			log.Printf(">> [SENDING] %d bytes", len(text))
			err := conn.WriteMessage(websocket.TextMessage, []byte(text))
			if err != nil {
				log.Println("Write error:", err)
				return
			}
		}
	}()

	// Handle clipboard changes from the network
	go func() {
		for {
			// Block until a message is received from the server
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("Read error (disconnect?):", err)
				return
			}
			data := string(message)

			log.Printf(">> [RECEIVED] %d bytes", len(data))

			// Update state before writing to clipboard to prevent Watcher from triggering
			mu.Lock()
			lastSyncedContent = data
			mu.Unlock()

			// Write to system clipboard
			// This will trigger the watcher but it won't effectively update the clipboard
			// because of the updated lastSyncedContent.
			clipboard.Write(clipboard.FmtText, message)
		}
	}()

	// Keep the program alive until Ctrl+C
	keepalive()
}

func keepalive() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Println("\nStopping due to interrupt...")
}
