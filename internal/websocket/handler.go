package websocket

import (
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

// Handler handles websocket connections
type Handler struct {
	Hub *Hub
}

// NewHandler creates a new websocket handler
func NewHandler(hub *Hub) *Handler {
	return &Handler{
		Hub: hub,
	}
}

// HandleWebSocket handles websocket connection upgrades
func (h *Handler) HandleWebSocket(c *fiber.Ctx) error {
	// Extract workflow ID from params
	workflowIDStr := c.Params("workflowId")
	workflowID, err := strconv.ParseUint(workflowIDStr, 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid workflow ID",
		})
	}

	// Use Fiber's websocket middleware
	return websocket.New(func(wsConn *websocket.Conn) {
		// Create client with Fiber websocket connection
		client := &Client{
			ID:         generateClientID(),
			WorkflowID: uint(workflowID),
			Conn:       &FiberConnAdapter{Conn: wsConn},
			Send:       make(chan []byte, 256),
			Hub:        h.Hub,
		}

		// Register client
		h.Hub.register <- client

		log.Printf("WebSocket client connected: %s for workflow %d", client.ID, client.WorkflowID)

		// Start write pump
		go client.WritePump()

		// Read pump (blocking)
		client.ReadPump()

		log.Printf("WebSocket client disconnected: %s", client.ID)
	})(c)
}

// FiberConnAdapter adapts Fiber's websocket connection to our Connection interface
type FiberConnAdapter struct {
	Conn *websocket.Conn
}

// WriteMessage writes a message to the websocket connection
func (a *FiberConnAdapter) WriteMessage(messageType int, data []byte) error {
	// Fiber's websocket uses TextMessage constant
	return a.Conn.WriteMessage(websocket.TextMessage, data)
}

// ReadMessage reads a message from the websocket connection
func (a *FiberConnAdapter) ReadMessage() (messageType int, p []byte, err error) {
	mt, p, err := a.Conn.ReadMessage()
	return mt, p, err
}

// Close closes the websocket connection
func (a *FiberConnAdapter) Close() error {
	return a.Conn.Close()
}

// SetReadDeadline sets the read deadline (not supported in Fiber's websocket)
func (a *FiberConnAdapter) SetReadDeadline(deadline interface{}) error {
	return nil
}

// SetWriteDeadline sets the write deadline (not supported in Fiber's websocket)
func (a *FiberConnAdapter) SetWriteDeadline(deadline interface{}) error {
	return nil
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return "client-" + strconv.FormatInt(int64(makeTimestamp()), 10)
}

// makeTimestamp generates a timestamp for unique IDs
func makeTimestamp() int64 {
	return time.Now().UnixNano()
}
