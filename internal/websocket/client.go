package websocket

import (
	"encoding/json"
	"log"
	"sync"
)

const (
	// TextMessage denotes a text data message
	TextMessage = 1
	// CloseMessage denotes a close control message
	CloseMessage = 8
)

// Connection represents a websocket connection interface
type Connection interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
	SetReadDeadline(deadline interface{}) error
	SetWriteDeadline(deadline interface{}) error
}

// Client represents a websocket client
type Client struct {
	ID         string
	WorkflowID uint
	Conn       Connection
	Send       chan []byte
	Hub        *Hub
	mu         sync.Mutex
}

// Hub maintains the set of active clients
type Hub struct {
	clients         map[*Client]bool
	register        chan *Client
	unregister      chan *Client
	broadcast       chan []byte
	workflowClients map[uint][]*Client // workflowID -> clients
	mu              sync.RWMutex
}

// NewHub creates a new websocket hub
func NewHub() *Hub {
	return &Hub{
		clients:         make(map[*Client]bool),
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		broadcast:       make(chan []byte),
		workflowClients: make(map[uint][]*Client),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient adds a new client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true

	// Add to workflow-specific clients
	h.workflowClients[client.WorkflowID] = append(h.workflowClients[client.WorkflowID], client)

	log.Printf("Client %s registered for workflow %d", client.ID, client.WorkflowID)
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.Send)

		// Remove from workflow-specific clients
		if clients, exists := h.workflowClients[client.WorkflowID]; exists {
			for i, c := range clients {
				if c.ID == client.ID {
					h.workflowClients[client.WorkflowID] = append(clients[:i], clients[i+1:]...)
					break
				}
			}
		}

		log.Printf("Client %s unregistered from workflow %d", client.ID, client.WorkflowID)
	}
}

// broadcastMessage sends a message to all clients
func (h *Hub) broadcastMessage(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.Send <- message:
		default:
			h.unregisterClient(client)
		}
	}
}

// BroadcastToWorkflow sends a message to all clients subscribed to a specific workflow
func (h *Hub) BroadcastToWorkflow(workflowID uint, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.workflowClients[workflowID]
	for _, client := range clients {
		select {
		case client.Send <- message:
		default:
			h.unregisterClient(client)
		}
	}
}

// SendEvent sends an event to workflow clients
func (h *Hub) SendEvent(workflowID uint, event *WSEvent) error {
	message, err := json.Marshal(event)
	if err != nil {
		return err
	}

	h.BroadcastToWorkflow(workflowID, message)
	return nil
}

// GetWorkflowClients returns the number of clients for a workflow
func (h *Hub) GetWorkflowClients(workflowID uint) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.workflowClients[workflowID])
}

// WritePump handles writing messages to the websocket connection
func (c *Client) WritePump() {
	defer c.Conn.Close()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(CloseMessage, []byte{})
				return
			}

			c.mu.Lock()
			err := c.Conn.WriteMessage(TextMessage, message)
			c.mu.Unlock()

			if err != nil {
				log.Printf("Error writing to client %s: %v", c.ID, err)
				return
			}
		}
	}
}

// ReadPump handles reading messages from the websocket connection
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading from client %s: %v", c.ID, err)
			break
		}

		// Handle incoming messages if needed
		log.Printf("Received message from client %s: %s", c.ID, string(message))
	}
}
