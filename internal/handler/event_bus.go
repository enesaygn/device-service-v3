// internal/handler/event_bus.go
package handler

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// EventBus manages event distribution
type EventBus struct {
	subscribers map[string][]chan Event
	events      chan Event
	mutex       sync.RWMutex
	logger      *zap.Logger
}

// Event represents a system event
type Event struct {
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan Event),
		events:      make(chan Event, 1000),
	}
}

// Start starts the event bus
func (eb *EventBus) Start() {
	for event := range eb.events {
		eb.distributeEvent(event)
	}
}

// Publish publishes an event
func (eb *EventBus) Publish(event Event) {
	select {
	case eb.events <- event:
	default:
		// Event bus is full, log warning
		if eb.logger != nil {
			eb.logger.Warn("Event bus full, dropping event",
				zap.String("event_type", event.Type),
			)
		}
	}
}

// Subscribe subscribes to events of a specific type
func (eb *EventBus) Subscribe(eventType string) <-chan Event {
	eb.mutex.Lock()
	defer eb.mutex.Unlock()

	subscriber := make(chan Event, 100)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], subscriber)
	return subscriber
}

// distributeEvent distributes an event to subscribers
func (eb *EventBus) distributeEvent(event Event) {
	eb.mutex.RLock()
	subscribers := eb.subscribers[event.Type]
	eb.mutex.RUnlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- event:
		default:
			// Subscriber is slow, skip
		}
	}
}

// DeviceEventHandler integrates with driver event system
type DeviceEventHandler struct {
	websocketHandler *WebSocketHandler
	logger           *zap.Logger
}

// NewDeviceEventHandler creates a new device event handler
func NewDeviceEventHandler(websocketHandler *WebSocketHandler, logger *zap.Logger) *DeviceEventHandler {
	return &DeviceEventHandler{
		websocketHandler: websocketHandler,
		logger:           logger,
	}
}

// OnDeviceConnected handles device connected events
func (deh *DeviceEventHandler) OnDeviceConnected(deviceID string) {
	deh.websocketHandler.BroadcastDeviceEvent(deviceID, "connected", map[string]interface{}{
		"status":  "online",
		"message": "Device connected successfully",
	})

	deh.logger.Info("Device connected event broadcasted", zap.String("device_id", deviceID))
}

// OnDeviceDisconnected handles device disconnected events
func (deh *DeviceEventHandler) OnDeviceDisconnected(deviceID string, reason string) {
	deh.websocketHandler.BroadcastDeviceEvent(deviceID, "disconnected", map[string]interface{}{
		"status": "offline",
		"reason": reason,
	})

	deh.logger.Info("Device disconnected event broadcasted",
		zap.String("device_id", deviceID),
		zap.String("reason", reason),
	)
}

// OnDeviceError handles device error events
func (deh *DeviceEventHandler) OnDeviceError(deviceID string, err error) {
	deh.websocketHandler.BroadcastDeviceEvent(deviceID, "error", map[string]interface{}{
		"status": "error",
		"error":  err.Error(),
	})

	deh.logger.Error("Device error event broadcasted",
		zap.String("device_id", deviceID),
		zap.Error(err),
	)
}

// OnOperationCompleted handles operation completed events
func (deh *DeviceEventHandler) OnOperationCompleted(deviceID string, operationID string, result interface{}) {
	// Convert operation ID to UUID if needed
	// This would typically use proper UUID parsing

	deh.logger.Info("Operation completed event",
		zap.String("device_id", deviceID),
		zap.String("operation_id", operationID),
	)
}

// OnStatusChanged handles device status change events
func (deh *DeviceEventHandler) OnStatusChanged(deviceID string, oldStatus, newStatus string) {
	deh.websocketHandler.BroadcastDeviceEvent(deviceID, "status_changed", map[string]interface{}{
		"old_status": oldStatus,
		"new_status": newStatus,
	})

	deh.logger.Info("Device status change event broadcasted",
		zap.String("device_id", deviceID),
		zap.String("old_status", oldStatus),
		zap.String("new_status", newStatus),
	)
}
