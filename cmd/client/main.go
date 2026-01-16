package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"

	"github.com/gorilla/websocket"
	"golang.design/x/clipboard"
)

var (
	lastSyncedContent string     // Stores the PLAINTEXT of the last sync
	mu                sync.Mutex // Mutex ensures thread-safety between Watcher and Network routines
	encryptionKey     []byte     // The 32-byte AES key derived from user provided password
)

var (
	serverURL = flag.String("serverAddr", "ws://localhost:8080/ws?room=default", "Server address to connect to")
	password  = flag.String("password", "", "Password to encrypt clipboard data (Required)")
)

func main() {
	// Parse the CLI commands and flags
	flag.Parse()

	// Validate the password and derive the key
	if *password == "" {
		log.Fatal("Error: --password is a required field for encryption.")
	}
	encryptionKey = deriveKey(*password)
	fmt.Printf(">> Security: AES-256 Key derived successfully.\n")

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
		for textBytes := range clipboardWatcher {
			text := string(textBytes)

			// echo cancellation:
			// If this text matches what we just decrypted from the network, ignore it.
			mu.Lock()
			if text == lastSyncedContent {
				mu.Unlock()
				continue
			}
			// Update local state
			lastSyncedContent = text
			mu.Unlock()

			// Encrypt data before sending to the server
			encryptedData, err := encrypt(textBytes, encryptionKey)
			if err != nil {
				log.Printf("Encryption error: %v", err)
				continue
			}

			// Send data to the WebSocket server
			log.Printf(">> [SENDING] Encrypted Blob: %d bytes", len(encryptedData))
			err = conn.WriteMessage(websocket.BinaryMessage, encryptedData)
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
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("Read error (disconnect?):", err)
				return
			}

			// Ignore any message which is not of the same binary message format.
			if messageType != websocket.BinaryMessage {
				log.Println("Warning: Received non-binary message, ignoring.")
				continue
			}

			decryptedBytes, err := decrypt(message, encryptionKey)
			if err != nil {
				log.Printf("Decryption failed (Wrong Password?): %v", err)
				continue
			}
			decryptedText := string(decryptedBytes)

			log.Printf(">> [RECEIVED] %d bytes", len(decryptedText))

			// Update state before writing to clipboard to prevent Watcher from triggering
			mu.Lock()
			lastSyncedContent = decryptedText
			mu.Unlock()

			// Write to system clipboard
			// This will trigger the watcher but it won't effectively update the clipboard
			// because of the updated lastSyncedContent.
			clipboard.Write(clipboard.FmtText, decryptedBytes)
		}
	}()

	keepalive()
}

// decrypt opens the data using AES-GCM
// Expects input: [Nonce (12b)] + [Ciphertext]
func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Split nonce and actual ciphertext
	nonce, actualCiphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	return gcm.Open(nil, nonce, actualCiphertext, nil)
}

// encrypt seals the data using AES-GCM
// Output format: [Nonce (12b)] + [Ciphertext]
func encrypt(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Create a random nonce value using the rand package.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// deriveKey turns a string password into a 32-byte key using SHA-256
func deriveKey(password string) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}

// keepalive keeps the client running until Ctrl+C
func keepalive() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	fmt.Println("\nStopping due to interrupt...")
}
