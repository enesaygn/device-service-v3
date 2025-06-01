// internal/handler/websocket_types.go
package handler

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a WebSocket client
type Client struct {
	ID            string          `json:"id"`
	Connection    *websocket.Conn `json:"-"`
	Send          chan []byte     `json:"-"`
	Type          string          `json:"type"` // device, events, operations, branch
	DeviceID      *string         `json:"device_id,omitempty"`
	BranchID      *string         `json:"branch_id,omitempty"`
	UserAgent     string          `json:"user_agent"`
	RemoteAddr    string          `json:"remote_addr"`
	ConnectedAt   time.Time       `json:"connected_at"`
	Subscriptions map[string]bool `json:"subscriptions,omitempty"`
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// ConnectionManager manages WebSocket connections
type ConnectionManager struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	mutex      sync.RWMutex
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager() *ConnectionManager {
	manager := &ConnectionManager{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}

	go manager.run()
	return manager
}

// run starts the connection manager
func (cm *ConnectionManager) run() {
	for {
		select {
		case client := <-cm.register:
			cm.mutex.Lock()
			cm.clients[client.ID] = client
			cm.mutex.Unlock()

		case client := <-cm.unregister:
			cm.mutex.Lock()
			if _, ok := cm.clients[client.ID]; ok {
				delete(cm.clients, client.ID)
				close(client.Send)
			}
			cm.mutex.Unlock()
		}
	}
}

// Register registers a new client
func (cm *ConnectionManager) Register(client *Client) {
	cm.register <- client
}

// Unregister unregisters a client
func (cm *ConnectionManager) Unregister(client *Client) {
	cm.unregister <- client
}

// GetDeviceClients returns clients connected to a specific device
func (cm *ConnectionManager) GetDeviceClients(deviceID string) []*Client {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var clients []*Client
	for _, client := range cm.clients {
		if client.DeviceID != nil && *client.DeviceID == deviceID {
			clients = append(clients, client)
		}
	}
	return clients
}

// GetEventClients returns all event clients
func (cm *ConnectionManager) GetEventClients() []*Client {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var clients []*Client
	for _, client := range cm.clients {
		if client.Type == "events" {
			clients = append(clients, client)
		}
	}
	return clients
}

// GetOperationClients returns all operation clients
func (cm *ConnectionManager) GetOperationClients() []*Client {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var clients []*Client
	for _, client := range cm.clients {
		if client.Type == "operations" {
			clients = append(clients, client)
		}
	}
	return clients
}

// GetBranchClients returns clients connected to a specific branch
func (cm *ConnectionManager) GetBranchClients(branchID string) []*Client {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var clients []*Client
	for _, client := range cm.clients {
		if client.BranchID != nil && *client.BranchID == branchID {
			clients = append(clients, client)
		}
	}
	return clients
}

// GetStats returns connection statistics
func (cm *ConnectionManager) GetStats() *ConnectionStats {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	stats := &ConnectionStats{
		TotalConnections: len(cm.clients),
		ByType:           make(map[string]int),
		Clients:          make([]*Client, 0, len(cm.clients)),
	}

	for _, client := range cm.clients {
		stats.ByType[client.Type]++
		stats.Clients = append(stats.Clients, client)
	}

	return stats
}

// ConnectionStats represents connection statistics
type ConnectionStats struct {
	TotalConnections int            `json:"total_connections"`
	ByType           map[string]int `json:"by_type"`
	Clients          []*Client      `json:"clients"`
}
