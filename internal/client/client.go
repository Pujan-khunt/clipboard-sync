// Package client handles the core application logic for the clipboard synchronization
// agent. It orchestrates the interaction between the system clipboard, the
// cryptographic layer, and the WebSocket network connection.
package client

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"

	"github.com/Pujan-khunt/clipboard-sync/internal/clipboard"
	"github.com/Pujan-khunt/clipboard-sync/internal/crypto"
	"github.com/gorilla/websocket"
)

// App represents the client application state and dependencies
type App struct {
	ServerURL string
	Password  string

	clipboard *clipboard.Manager
	key       []byte
	conn      *websocket.Conn
}

// NewApp creates a new instance of the client application.
func NewApp(serverURL, password string) *App {
	return &App{
		ServerURL: serverURL,
		Password:  password,
		clipboard: clipboard.NewManager(),
	}
}

// Run starts the main application loop. It connects to the server,
// initializes the clipboard, and starts the sync goroutines.
func (a *App) Run() error {
	// Setup crypto
	if a.Password == "" {
		return fmt.Errorf("password is required for encryption")
	}
	a.key = crypto.DeriveKey(a.Password)
	log.Println(">> Security: AES-256 Key derived.")

	// Setup clipboard
	if err := a.clipboard.Init(); err != nil {
		return fmt.Errorf("clipboard init failed: %w", err)
	}
	log.Println(">> Clipboard: Watcher started.")

	// Connect to the Server
	u, err := url.Parse(a.ServerURL)
	if err != nil {
		fmt.Errorf("Invalid server URL: %w", err)
	}
	log.Printf(">> Network: Connecting to %s...", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	a.conn = conn
	defer a.conn.Close()
	log.Printf(">> Network: Connected & Secure")

	// Use a context for graceful cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start synchronization loops
	go a.handleOutgoing(ctx)
	go a.handleIncoming(ctx)

	// Wait for interrupt
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Printf("Interrupt received. Stopping...")
	return nil
}

func (a *App) handleOutgoing(ctx context.Context) {
	updates := a.clipboard.Watch(ctx)
	for data := range updates {
		if a.clipboard.ShouldIgnore(data) {
			continue
		}

		log.Printf("[LOCAL COPY] %d bytes. Encrypting & Sending...", len(data))

		encrypted, err := crypto.Encrypt(data, a.key)
		if err != nil {
			log.Printf("Encryption error: %v", err)
			continue
		}

		if err := a.conn.WriteMessage(websocket.BinaryMessage, encrypted); err != nil {
			log.Println("Network Write error", err)
			return
		}
	}
}

func (a *App) handleIncoming(ctx context.Context) {
	for {
		msgType, data, err := a.conn.ReadMessage()
		if err != nil {
			log.Println("Network read error:", err)
			return
		}
		if msgType != websocket.BinaryMessage {
			log.Println("Message received in a format which is not binary. Ignoring it.")
			continue
		}

		decrypted, err := crypto.Decrypt(data, a.key)
		if err != nil {
			log.Printf("Decryption failed (Wrong Password?): %v", err)
			continue
		}

		log.Printf("[REMOTE PASTE] Received %d bytes. Updating Clipboard.", len(decrypted))
		a.clipboard.WriteSafely(decrypted)
	}
}
