// Package clipboard provides a thread-safe wrapper around the system clipboard.
// It handles initialization, watching for changes, and most importantly,
// "Echo Cancellation" to prevent infinite loops when synchronizing data.
package clipboard

import (
	"context"
	"sync"

	"golang.design/x/clipboard"
)

// Manager handles the local clipboard state and prevents infinite echo loops.
type Manager struct {
	lastContent string
	mu          sync.Mutex
}

// NewManager creates a thread-safe clipboard manager.
func NewManager() *Manager {
	return &Manager{}
}

// Init initializes the system clipboard.
func (m *Manager) Init() error {
	return clipboard.Init()
}

// Watch returns a channel that emits text whenever the user copies text data.
func (m *Manager) Watch(ctx context.Context) <-chan []byte {
	return clipboard.Watch(ctx, clipboard.FmtText)
}

// WriteSafely writes to the system clipboard and updates the internal state
// so that the Watcher knows to ignore the specific update (Echo cancellation).
func (m *Manager) WriteSafely(content []byte) {
	text := string(content)

	m.mu.Lock()
	m.lastContent = text
	m.mu.Unlock()

	clipboard.Write(clipboard.FmtText, content)
}

// ShouldIgnore checks if the given text matches the last thing we wrote programmatically.
// If it matches, it means the "change" event was triggered by us, and it should be ignored.
func (m *Manager) ShouldIgnore(content []byte) bool {
	text := string(content)

	m.mu.Lock()
	defer m.mu.Unlock()

	if text == m.lastContent {
		return true
	}

	m.lastContent = text
	return false
}
