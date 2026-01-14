package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"golang.design/x/clipboard"
)

var lastSyncedContent string

func main() {
	// Initialize the clipboard process.
	// Init can fail if the dependencies are not installed.
	err := clipboard.Init()
	if err != nil {
		log.Fatalf("Failed to initialize clipboard: %v", err)
	}
	fmt.Println(">> Clipboard Watcher Started. Copy text anywhere to see it here.")

	// Start the watcher (listener)
	// we receive a channel that will receive bytes whenever the clipboard changes with text(FmtText) data.
	ch := clipboard.Watch(context.Background(), clipboard.FmtText)

	// Handle incoming clipboard changes in a separate goroutine
	go func() {
		for data := range ch {
			text := string(data)

			// Ignore the operation if the last copied content is the same as the currently copied content.
			if text == lastSyncedContent {
				continue
			}

			// Update the last synced content to match the currently copied text.
			lastSyncedContent = text

			// Currently its being logged to stdout, later it will be sent to the websocket server.
			log.Printf(">> [CAPTURED] Size: %d bytes | Content: %q\n", len(text), text)
		}
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			fakeNetworkData := fmt.Sprintf("Message from Phone at %v", time.Now().Format("15:04:05"))
			log.Println(">> [NETWORK] Received update from other device. Writing to clipboard...")
			updateClipboard(fakeNetworkData, clipboard.FmtText)
		}
	}()

	// Keep the program alive until Ctrl+C
	keepalive()
}

func updateClipboard(text string, format clipboard.Format) {
	// Update state first to effectively pause the watcher logic for this specific string.
	lastSyncedContent = text

	// Write to system clipboard
	clipboard.Write(format, []byte(text))
}

func keepalive() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Println("\nStopping due to interrupt...")
}
