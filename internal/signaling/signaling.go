// Package signaling defines the message types used for WebRTC signaling
// over the WebSocket connection. These messages coordinate peer discovery
// and connection establishment without carrying clipboard data.
package signaling

import "encoding/json"

// Message types for signaling protocol
const (
	TypeJoin      = "join"      // Announce presence to room
	TypeLeave     = "leave"     // Announce departure from room
	TypeOffer     = "offer"     // WebRTC SDP offer
	TypeAnswer    = "answer"    // WebRTC SDP answer
	TypeCandidate = "candidate" // ICE candidate
)

// Message represents a signaling message sent over WebSocket.
// The server broadcasts these messages to other peers in the same room.
type Message struct {
	Type     string `json:"type"`              // Message type (join, leave, offer, answer, candidate)
	FromPeer string `json:"from"`              // Sender's peer ID
	ToPeer   string `json:"to,omitempty"`      // Target peer ID (empty = broadcast to all)
	Payload  string `json:"payload,omitempty"` // SDP or ICE candidate JSON
}

// Marshal serializes a signaling message to JSON bytes.
func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Unmarshal deserializes JSON bytes into a signaling message.
func Unmarshal(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
