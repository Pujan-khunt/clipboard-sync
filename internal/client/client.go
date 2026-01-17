// Package client handles the core application logic for the clipboard synchronization
// agent. It uses WebSocket for signaling and WebRTC DataChannels for peer-to-peer
// clipboard data transfer, with end-to-end encryption via AES-256-GCM.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"

	"github.com/Pujan-khunt/clipboard-sync/internal/clipboard"
	"github.com/Pujan-khunt/clipboard-sync/internal/crypto"
	"github.com/Pujan-khunt/clipboard-sync/internal/signaling"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

// App represents the client application state and dependencies
type App struct {
	ServerURL string
	Password  string

	clipboard *clipboard.Manager
	key       []byte
	conn      *websocket.Conn

	// P2P WebRTC fields
	peerID    string                            // Unique identifier for this peer
	peers     map[string]*webrtc.PeerConnection // PeerConnection per remote peer
	dataChans map[string]*webrtc.DataChannel    // DataChannel per remote peer
	mu        sync.RWMutex                      // Protects peers and dataChans maps
	wsMu      sync.Mutex                        // Protects WebSocket writes
}

// NewApp creates a new instance of the client application.
func NewApp(serverURL, password, peerID string) *App {
	if peerID == "" {
		peerID = uuid.New().String()[:8] // Short UUID for readability
	}
	return &App{
		ServerURL: serverURL,
		Password:  password,
		peerID:    peerID,
		clipboard: clipboard.NewManager(),
		peers:     make(map[string]*webrtc.PeerConnection),
		dataChans: make(map[string]*webrtc.DataChannel),
	}
}

// getWebRTCConfig returns the WebRTC configuration with STUN servers
func getWebRTCConfig() webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
			{URLs: []string{"stun:stun1.l.google.com:19302"}},
		},
	}
}

// Run starts the main application loop. It connects to the signaling server,
// initializes the clipboard, and manages P2P connections.
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
	log.Println(">> Clipboard: System environment initialized.")

	// Parse server URL
	u, err := url.Parse(a.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	log.Printf(">> Network: Connecting to signaling server %s...", u.String())

	// Add peer id to query parameters
	q := u.Query()
	q.Set("peer_id", a.peerID)
	u.RawQuery = q.Encode()

	// Connect to the Signaling Server
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("signaling connection failed: %w", err)
	}
	a.conn = conn
	defer a.conn.Close()
	log.Printf(">> Network: Connected to signaling server (PeerID: %s)", a.peerID)

	// Use a context for graceful cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Announce presence to the room
	if err := a.sendSignal(&signaling.Message{
		Type:     signaling.TypeJoin,
		FromPeer: a.peerID,
	}); err != nil {
		return fmt.Errorf("failed to announce presence: %w", err)
	}

	// Start signaling handler and clipboard watcher
	go a.handleSignaling(ctx)
	go a.handleOutgoingClipboard(ctx)

	// Wait for interrupt
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	log.Println("Interrupt received. Closing p2p connection with all peers...")

	// Announce departure
	a.sendSignal(&signaling.Message{
		Type:     signaling.TypeLeave,
		FromPeer: a.peerID,
	})

	log.Println("p2p connections closed successfully.")

	return nil
}

// sendSignal sends a signaling message over WebSocket
func (a *App) sendSignal(msg *signaling.Message) error {
	data, err := msg.Marshal()
	if err != nil {
		return err
	}
	a.wsMu.Lock()
	defer a.wsMu.Unlock()
	return a.conn.WriteMessage(websocket.TextMessage, data)
}

// handleSignaling processes incoming signaling messages from WebSocket
func (a *App) handleSignaling(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, data, err := a.conn.ReadMessage()
		if err != nil {
			log.Println("Signaling read error:", err)
			return
		}

		msg, err := signaling.Unmarshal(data)
		if err != nil {
			log.Println("Invalid signaling message:", err)
			continue
		}

		// Ignore our own messages
		if msg.FromPeer == a.peerID {
			continue
		}

		// Handle message based on type
		switch msg.Type {
		case signaling.TypeJoin:
			log.Printf("[PEER JOIN] %s joined the room", msg.FromPeer)
			// Initiate connection to new peer (we send offer)
			go a.initiateConnection(msg.FromPeer)

		case signaling.TypeLeave:
			log.Printf("[PEER LEAVE] %s left the room", msg.FromPeer)
			a.closePeerConnection(msg.FromPeer)

		case signaling.TypeOffer:
			log.Printf("[OFFER] from %s", msg.FromPeer)
			go a.handleOffer(msg.FromPeer, msg.Payload)

		case signaling.TypeAnswer:
			log.Printf("[ANSWER] from %s", msg.FromPeer)
			go a.handleAnswer(msg.FromPeer, msg.Payload)

		case signaling.TypeCandidate:
			go a.handleCandidate(msg.FromPeer, msg.Payload)
		}
	}
}

// initiateConnection creates a new PeerConnection and sends an offer
func (a *App) initiateConnection(remotePeerID string) {
	pc, err := a.createPeerConnection(remotePeerID, true)
	if err != nil {
		log.Printf("Failed to create PeerConnection for %s: %v", remotePeerID, err)
		return
	}

	// Create DataChannel (initiator creates it)
	dc, err := pc.CreateDataChannel("clipboard", nil)
	if err != nil {
		log.Printf("Failed to create DataChannel: %v", err)
		return
	}
	a.setupDataChannel(remotePeerID, dc)

	// Create and send offer
	offer, err := pc.CreateOffer(nil)
	if err != nil {
		log.Printf("Failed to create offer: %v", err)
		return
	}

	if err := pc.SetLocalDescription(offer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		return
	}

	// Wait for ICE gathering to complete
	<-webrtc.GatheringCompletePromise(pc)

	offerJSON, _ := json.Marshal(pc.LocalDescription())
	a.sendSignal(&signaling.Message{
		Type:     signaling.TypeOffer,
		FromPeer: a.peerID,
		ToPeer:   remotePeerID,
		Payload:  string(offerJSON),
	})
}

// handleOffer processes an SDP offer from a remote peer
func (a *App) handleOffer(remotePeerID, payload string) {
	var offer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(payload), &offer); err != nil {
		log.Printf("Failed to parse offer: %v", err)
		return
	}

	pc, err := a.createPeerConnection(remotePeerID, false)
	if err != nil {
		log.Printf("Failed to create PeerConnection for %s: %v", remotePeerID, err)
		return
	}

	if err := pc.SetRemoteDescription(offer); err != nil {
		log.Printf("Failed to set remote description: %v", err)
		return
	}

	// Create and send answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Printf("Failed to create answer: %v", err)
		return
	}

	if err := pc.SetLocalDescription(answer); err != nil {
		log.Printf("Failed to set local description: %v", err)
		return
	}

	// Wait for ICE gathering to complete
	<-webrtc.GatheringCompletePromise(pc)

	answerJSON, _ := json.Marshal(pc.LocalDescription())
	a.sendSignal(&signaling.Message{
		Type:     signaling.TypeAnswer,
		FromPeer: a.peerID,
		ToPeer:   remotePeerID,
		Payload:  string(answerJSON),
	})
}

// handleAnswer processes an SDP answer from a remote peer
func (a *App) handleAnswer(remotePeerID, payload string) {
	a.mu.RLock()
	pc, exists := a.peers[remotePeerID]
	a.mu.RUnlock()

	if !exists {
		log.Printf("No PeerConnection for %s", remotePeerID)
		return
	}

	var answer webrtc.SessionDescription
	if err := json.Unmarshal([]byte(payload), &answer); err != nil {
		log.Printf("Failed to parse answer: %v", err)
		return
	}

	if err := pc.SetRemoteDescription(answer); err != nil {
		log.Printf("Failed to set remote description: %v", err)
	}
}

// handleCandidate processes an ICE candidate from a remote peer
func (a *App) handleCandidate(remotePeerID, payload string) {
	a.mu.RLock()
	pc, exists := a.peers[remotePeerID]
	a.mu.RUnlock()

	if !exists {
		return // Peer not yet created, candidate will come with offer/answer
	}

	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(payload), &candidate); err != nil {
		log.Printf("Failed to parse ICE candidate: %v", err)
		return
	}

	if err := pc.AddICECandidate(candidate); err != nil {
		log.Printf("Failed to add ICE candidate: %v", err)
	}
}

// createPeerConnection creates and registers a new WebRTC PeerConnection
func (a *App) createPeerConnection(remotePeerID string, isInitiator bool) (*webrtc.PeerConnection, error) {
	pc, err := webrtc.NewPeerConnection(getWebRTCConfig())
	if err != nil {
		return nil, err
	}

	// Register connection state handler
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("[P2P %s] Connection state: %s", remotePeerID, state.String())
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			a.closePeerConnection(remotePeerID)
		}
		if state == webrtc.PeerConnectionStateConnected {
			log.Printf(">> P2P: Direct connection established with %s", remotePeerID)
		}
	})

	// Handle incoming DataChannel (for non-initiator)
	if !isInitiator {
		pc.OnDataChannel(func(dc *webrtc.DataChannel) {
			log.Printf("[P2P %s] DataChannel '%s' received", remotePeerID, dc.Label())
			a.setupDataChannel(remotePeerID, dc)
		})
	}

	// Handle ICE candidates
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		candidateJSON, _ := json.Marshal(c.ToJSON())
		a.sendSignal(&signaling.Message{
			Type:     signaling.TypeCandidate,
			FromPeer: a.peerID,
			ToPeer:   remotePeerID,
			Payload:  string(candidateJSON),
		})
	})

	a.mu.Lock()
	a.peers[remotePeerID] = pc
	a.mu.Unlock()

	return pc, nil
}

// setupDataChannel configures event handlers for a DataChannel
func (a *App) setupDataChannel(remotePeerID string, dc *webrtc.DataChannel) {
	dc.OnOpen(func() {
		log.Printf(">> DataChannel: Connected to %s", remotePeerID)
		a.mu.Lock()
		a.dataChans[remotePeerID] = dc
		a.mu.Unlock()
	})

	dc.OnClose(func() {
		log.Printf(">> DataChannel: Closed with %s", remotePeerID)
		a.mu.Lock()
		delete(a.dataChans, remotePeerID)
		a.mu.Unlock()
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		// Received encrypted clipboard data from peer
		decrypted, err := crypto.Decrypt(msg.Data, a.key)
		if err != nil {
			log.Printf("Decryption failed (Wrong Password?): %v", err)
			return
		}
		log.Printf("[REMOTE PASTE] Received %d bytes from %s. Updating Clipboard.", len(decrypted), remotePeerID)
		a.clipboard.WriteSafely(decrypted)
	})
}

// closePeerConnection cleans up a peer connection
func (a *App) closePeerConnection(remotePeerID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Close and delete the data channel for the peer requesting it.
	if dc, exists := a.dataChans[remotePeerID]; exists {
		dc.Close()
		delete(a.dataChans, remotePeerID)
	}

	// Close and delete the peer connection for the peer requesting it.
	if pc, exists := a.peers[remotePeerID]; exists {
		pc.Close()
		delete(a.peers, remotePeerID)
	}
}

// handleOutgoingClipboard watches for clipboard changes and broadcasts to all peers
func (a *App) handleOutgoingClipboard(ctx context.Context) {
	updates := a.clipboard.Watch(ctx)
	log.Println(">> Clipboard: Clipboard watcher started.")

	for data := range updates {
		if a.clipboard.ShouldIgnore(data) {
			continue
		}

		log.Printf("[LOCAL COPY] %d bytes. Encrypting & sending to peers...", len(data))

		encrypted, err := crypto.Encrypt(data, a.key)
		if err != nil {
			log.Printf("Encryption error: %v", err)
			continue
		}

		// Broadcast to all connected peers via DataChannels
		a.mu.RLock()
		for peerID, dc := range a.dataChans {
			if dc.ReadyState() == webrtc.DataChannelStateOpen {
				if err := dc.Send(encrypted); err != nil {
					log.Printf("Failed to send to %s: %v", peerID, err)
				}
			}
		}
		a.mu.RUnlock()
	}
}
