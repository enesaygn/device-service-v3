// internal/handler/websocket_handler.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"device-service/internal/service"
	"device-service/internal/utils"
)

// WebSocketHandler manages WebSocket connections for real-time communication
type WebSocketHandler struct {
	upgrader         websocket.Upgrader
	connections      *ConnectionManager
	deviceService    *service.DeviceService
	operationService *service.OperationService
	logger           *utils.ServiceLogger
	eventBus         *EventBus
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(
	deviceService *service.DeviceService,
	operationService *service.OperationService,
	logger *zap.Logger,
) *WebSocketHandler {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// In production, implement proper origin checking
			return true
		},
	}

	handler := &WebSocketHandler{
		upgrader:         upgrader,
		connections:      NewConnectionManager(),
		deviceService:    deviceService,
		operationService: operationService,
		logger:           utils.NewServiceLogger(logger, "websocket-handler"),
		eventBus:         NewEventBus(),
	}

	// Start event bus
	go handler.eventBus.Start()

	return handler
}

// RegisterRoutes registers WebSocket routes
func (h *WebSocketHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Device-specific WebSocket connections
	router.GET("/devices/:device_id", h.HandleDeviceConnection)

	// General device events WebSocket
	router.GET("/events", h.HandleEventConnection)

	// Operation status WebSocket
	router.GET("/operations", h.HandleOperationConnection)

	// Branch-wide events
	router.GET("/branches/:branch_id", h.HandleBranchConnection)
}

// HandleDeviceConnection handles device-specific WebSocket connections
func (h *WebSocketHandler) HandleDeviceConnection(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id is required"})
		return
	}

	// Upgrade connection
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket connection", zap.Error(err))
		return
	}

	// Create client
	client := &Client{
		ID:          uuid.New().String(),
		Connection:  conn,
		Send:        make(chan []byte, 256),
		Type:        "device",
		DeviceID:    &deviceID,
		UserAgent:   c.Request.UserAgent(),
		RemoteAddr:  c.Request.RemoteAddr,
		ConnectedAt: time.Now(),
	}

	// Register client
	h.connections.Register(client)
	h.logger.Info("Device WebSocket client connected",
		zap.String("client_id", client.ID),
		zap.String("device_id", deviceID),
		zap.String("remote_addr", client.RemoteAddr),
	)

	// Send initial device status
	go h.sendInitialDeviceStatus(client, deviceID)

	// Start client goroutines
	go h.handleClientRead(client)
	go h.handleClientWrite(client)
}

// HandleEventConnection handles general event WebSocket connections
func (h *WebSocketHandler) HandleEventConnection(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket connection", zap.Error(err))
		return
	}

	client := &Client{
		ID:          uuid.New().String(),
		Connection:  conn,
		Send:        make(chan []byte, 256),
		Type:        "events",
		UserAgent:   c.Request.UserAgent(),
		RemoteAddr:  c.Request.RemoteAddr,
		ConnectedAt: time.Now(),
	}

	h.connections.Register(client)
	h.logger.Info("Event WebSocket client connected",
		zap.String("client_id", client.ID),
	)

	go h.handleClientRead(client)
	go h.handleClientWrite(client)
}

// HandleOperationConnection handles operation status WebSocket connections
func (h *WebSocketHandler) HandleOperationConnection(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket connection", zap.Error(err))
		return
	}

	client := &Client{
		ID:          uuid.New().String(),
		Connection:  conn,
		Send:        make(chan []byte, 256),
		Type:        "operations",
		UserAgent:   c.Request.UserAgent(),
		RemoteAddr:  c.Request.RemoteAddr,
		ConnectedAt: time.Now(),
	}

	h.connections.Register(client)
	h.logger.Info("Operation WebSocket client connected",
		zap.String("client_id", client.ID),
	)

	go h.handleClientRead(client)
	go h.handleClientWrite(client)
}

// HandleBranchConnection handles branch-wide WebSocket connections
func (h *WebSocketHandler) HandleBranchConnection(c *gin.Context) {
	branchID := c.Param("branch_id")
	if branchID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "branch_id is required"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket connection", zap.Error(err))
		return
	}

	client := &Client{
		ID:          uuid.New().String(),
		Connection:  conn,
		Send:        make(chan []byte, 256),
		Type:        "branch",
		BranchID:    &branchID,
		UserAgent:   c.Request.UserAgent(),
		RemoteAddr:  c.Request.RemoteAddr,
		ConnectedAt: time.Now(),
	}

	h.connections.Register(client)
	h.logger.Info("Branch WebSocket client connected",
		zap.String("client_id", client.ID),
		zap.String("branch_id", branchID),
	)

	go h.handleClientRead(client)
	go h.handleClientWrite(client)
}

// handleClientRead handles reading messages from WebSocket client
func (h *WebSocketHandler) handleClientRead(client *Client) {
	defer func() {
		h.connections.Unregister(client)
		client.Connection.Close()
	}()

	// Set read deadline and pong handler
	client.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Connection.SetPongHandler(func(string) error {
		client.Connection.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := client.Connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("WebSocket read error",
					zap.Error(err),
					zap.String("client_id", client.ID),
				)
			}
			break
		}

		// Parse message
		var message WebSocketMessage
		if err := json.Unmarshal(messageBytes, &message); err != nil {
			h.logger.Error("Failed to parse WebSocket message",
				zap.Error(err),
				zap.String("client_id", client.ID),
			)
			continue
		}

		// Handle message
		h.handleClientMessage(client, &message)
	}
}

// handleClientWrite handles writing messages to WebSocket client
func (h *WebSocketHandler) handleClientWrite(client *Client) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.Connection.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Connection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.Connection.WriteMessage(websocket.TextMessage, message); err != nil {
				h.logger.Error("WebSocket write error",
					zap.Error(err),
					zap.String("client_id", client.ID),
				)
				return
			}

		case <-ticker.C:
			client.Connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleClientMessage handles incoming client messages
func (h *WebSocketHandler) handleClientMessage(client *Client, message *WebSocketMessage) {
	switch message.Type {
	case "subscribe":
		h.handleSubscription(client, message)
	case "unsubscribe":
		h.handleUnsubscription(client, message)
	case "device_command":
		h.handleDeviceCommand(client, message)
	case "ping":
		h.sendMessage(client, &WebSocketMessage{
			Type:      "pong",
			Timestamp: time.Now(),
		})
	default:
		h.logger.Warn("Unknown message type",
			zap.String("type", message.Type),
			zap.String("client_id", client.ID),
		)
	}
}

// handleSubscription handles client subscription requests
func (h *WebSocketHandler) handleSubscription(client *Client, message *WebSocketMessage) {
	if client.Subscriptions == nil {
		client.Subscriptions = make(map[string]bool)
	}

	if data, ok := message.Data.(map[string]interface{}); ok {
		if topic, ok := data["topic"].(string); ok {
			client.Subscriptions[topic] = true
			h.logger.Info("Client subscribed to topic",
				zap.String("client_id", client.ID),
				zap.String("topic", topic),
			)

			// Send subscription confirmation
			h.sendMessage(client, &WebSocketMessage{
				Type: "subscription_confirmed",
				Data: map[string]interface{}{
					"topic": topic,
				},
				Timestamp: time.Now(),
			})
		}
	}
}

// handleUnsubscription handles client unsubscription requests
func (h *WebSocketHandler) handleUnsubscription(client *Client, message *WebSocketMessage) {
	if client.Subscriptions == nil {
		return
	}

	if data, ok := message.Data.(map[string]interface{}); ok {
		if topic, ok := data["topic"].(string); ok {
			delete(client.Subscriptions, topic)
			h.logger.Info("Client unsubscribed from topic",
				zap.String("client_id", client.ID),
				zap.String("topic", topic),
			)
		}
	}
}

// handleDeviceCommand handles device command messages
func (h *WebSocketHandler) handleDeviceCommand(client *Client, message *WebSocketMessage) {
	if client.DeviceID == nil {
		h.sendError(client, "device_command only available on device connections")
		return
	}

	data, ok := message.Data.(map[string]interface{})
	if !ok {
		h.sendError(client, "invalid command data")
		return
	}

	command, ok := data["command"].(string)
	if !ok {
		h.sendError(client, "command is required")
		return
	}

	// Execute device command
	go h.executeDeviceCommand(client, *client.DeviceID, command, data)
}

// executeDeviceCommand executes a device command
func (h *WebSocketHandler) executeDeviceCommand(client *Client, deviceID, command string, data map[string]interface{}) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	var result interface{}

	switch command {
	case "connect":
		err = h.deviceService.ConnectDevice(ctx, deviceID)
		result = map[string]interface{}{"connected": err == nil}

	case "disconnect":
		err = h.deviceService.DisconnectDevice(ctx, deviceID)
		result = map[string]interface{}{"disconnected": err == nil}

	case "test":
		var testResult *service.TestResult
		testResult, err = h.deviceService.TestDevice(ctx, deviceID)
		result = testResult

	case "status":
		var health *service.DeviceHealth
		health, err = h.deviceService.GetDeviceHealth(ctx, deviceID)
		result = health

	default:
		h.sendError(client, fmt.Sprintf("unknown command: %s", command))
		return
	}

	// Send response
	response := &WebSocketMessage{
		Type: "command_response",
		Data: map[string]interface{}{
			"command": command,
			"success": err == nil,
			"result":  result,
		},
		Timestamp: time.Now(),
	}

	if err != nil {
		response.Data.(map[string]interface{})["error"] = err.Error()
	}

	h.sendMessage(client, response)
}

// sendInitialDeviceStatus sends initial device status to client
func (h *WebSocketHandler) sendInitialDeviceStatus(client *Client, deviceID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	device, err := h.deviceService.GetDevice(ctx, deviceID)
	if err != nil {
		h.sendError(client, fmt.Sprintf("failed to get device: %v", err))
		return
	}

	health, err := h.deviceService.GetDeviceHealth(ctx, deviceID)
	if err != nil {
		h.logger.Error("Failed to get device health", zap.Error(err))
	}

	message := &WebSocketMessage{
		Type: "initial_status",
		Data: map[string]interface{}{
			"device": device,
			"health": health,
		},
		Timestamp: time.Now(),
	}

	h.sendMessage(client, message)
}

// sendMessage sends a message to a client
func (h *WebSocketHandler) sendMessage(client *Client, message *WebSocketMessage) {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal WebSocket message", zap.Error(err))
		return
	}

	select {
	case client.Send <- messageBytes:
	default:
		h.logger.Warn("Client send channel full, dropping message",
			zap.String("client_id", client.ID),
		)
	}
}

// sendError sends an error message to a client
func (h *WebSocketHandler) sendError(client *Client, errorMsg string) {
	message := &WebSocketMessage{
		Type: "error",
		Data: map[string]interface{}{
			"error": errorMsg,
		},
		Timestamp: time.Now(),
	}
	h.sendMessage(client, message)
}

// BroadcastDeviceEvent broadcasts device events to relevant clients
func (h *WebSocketHandler) BroadcastDeviceEvent(deviceID string, eventType string, data interface{}) {
	message := &WebSocketMessage{
		Type: "device_event",
		Data: map[string]interface{}{
			"device_id":  deviceID,
			"event_type": eventType,
			"data":       data,
		},
		Timestamp: time.Now(),
	}

	h.broadcastToDeviceClients(deviceID, message)
	h.broadcastToEventClients(message)
}

// BroadcastOperationEvent broadcasts operation events to relevant clients
func (h *WebSocketHandler) BroadcastOperationEvent(operationID uuid.UUID, deviceID string, eventType string, data interface{}) {
	message := &WebSocketMessage{
		Type: "operation_event",
		Data: map[string]interface{}{
			"operation_id": operationID.String(),
			"device_id":    deviceID,
			"event_type":   eventType,
			"data":         data,
		},
		Timestamp: time.Now(),
	}

	h.broadcastToOperationClients(message)
	h.broadcastToDeviceClients(deviceID, message)
}

// broadcastToDeviceClients broadcasts to clients connected to a specific device
func (h *WebSocketHandler) broadcastToDeviceClients(deviceID string, message *WebSocketMessage) {
	clients := h.connections.GetDeviceClients(deviceID)
	h.broadcastToClients(clients, message)
}

// broadcastToEventClients broadcasts to all event clients
func (h *WebSocketHandler) broadcastToEventClients(message *WebSocketMessage) {
	clients := h.connections.GetEventClients()
	h.broadcastToClients(clients, message)
}

// broadcastToOperationClients broadcasts to all operation clients
func (h *WebSocketHandler) broadcastToOperationClients(message *WebSocketMessage) {
	clients := h.connections.GetOperationClients()
	h.broadcastToClients(clients, message)
}

// broadcastToClients broadcasts message to specified clients
func (h *WebSocketHandler) broadcastToClients(clients []*Client, message *WebSocketMessage) {
	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal broadcast message", zap.Error(err))
		return
	}

	for _, client := range clients {
		select {
		case client.Send <- messageBytes:
		default:
			h.logger.Warn("Client send channel full during broadcast",
				zap.String("client_id", client.ID),
			)
		}
	}
}

// GetConnectionStats returns connection statistics
func (h *WebSocketHandler) GetConnectionStats() *ConnectionStats {
	return h.connections.GetStats()
}
