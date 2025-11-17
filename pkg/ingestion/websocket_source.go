package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// WebSocketSource ingests events from WebSocket connections
type WebSocketSource struct {
	addr       string
	path       string
	server     *http.Server
	upgrader   websocket.Upgrader
	logger     *zap.Logger
	clients    map[*websocket.Conn]bool
	clientsMu  sync.RWMutex
	broadcast  chan []byte
}

// NewWebSocketSource creates a new WebSocket source
func NewWebSocketSource(addr, path string, logger *zap.Logger) *WebSocketSource {
	return &WebSocketSource{
		addr: addr,
		path: path,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
		logger:    logger,
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan []byte, 1000),
	}
}

// Start begins accepting WebSocket connections
func (w *WebSocketSource) Start(ctx context.Context, output chan<- *stream.Event) error {
	w.logger.Info("Starting WebSocket source",
		zap.String("addr", w.addr),
		zap.String("path", w.path))

	mux := http.NewServeMux()
	mux.HandleFunc(w.path, func(rw http.ResponseWriter, r *http.Request) {
		w.handleConnection(rw, r, output)
	})

	w.server = &http.Server{
		Addr:    w.addr,
		Handler: mux,
	}

	go func() {
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			w.logger.Error("WebSocket server error", zap.Error(err))
		}
	}()

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		w.Stop()
	}()

	return nil
}

// handleConnection handles a new WebSocket connection
func (w *WebSocketSource) handleConnection(rw http.ResponseWriter, r *http.Request, output chan<- *stream.Event) {
	conn, err := w.upgrader.Upgrade(rw, r, nil)
	if err != nil {
		w.logger.Error("WebSocket upgrade error", zap.Error(err))
		return
	}

	w.clientsMu.Lock()
	w.clients[conn] = true
	w.clientsMu.Unlock()

	w.logger.Info("New WebSocket client connected",
		zap.String("remote_addr", r.RemoteAddr))

	defer func() {
		w.clientsMu.Lock()
		delete(w.clients, conn)
		w.clientsMu.Unlock()
		conn.Close()
		w.logger.Info("WebSocket client disconnected")
	}()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				w.logger.Error("WebSocket read error", zap.Error(err))
			}
			break
		}

		if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
			event := w.messageToEvent(message)
			select {
			case output <- event:
			default:
				w.logger.Warn("Output channel full, dropping WebSocket message")
			}
		}
	}
}

// messageToEvent converts a WebSocket message to a stream event
func (w *WebSocketSource) messageToEvent(message []byte) *stream.Event {
	var value interface{}
	if err := json.Unmarshal(message, &value); err != nil {
		// If not JSON, use raw bytes
		value = message
	}

	return &stream.Event{
		Value:     value,
		EventTime: time.Now(),
		Headers: map[string]string{
			"source": "websocket",
		},
	}
}

// Stop stops the WebSocket server
func (w *WebSocketSource) Stop() error {
	w.logger.Info("Stopping WebSocket source")

	// Close all client connections
	w.clientsMu.Lock()
	for conn := range w.clients {
		conn.Close()
	}
	w.clientsMu.Unlock()

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return w.server.Shutdown(ctx)
}

// Name returns the source name
func (w *WebSocketSource) Name() string {
	return fmt.Sprintf("websocket-%s", w.path)
}

// Partitions returns the number of partitions (1 for WebSocket)
func (w *WebSocketSource) Partitions() int32 {
	return 1
}

// BroadcastMessage sends a message to all connected clients
func (w *WebSocketSource) BroadcastMessage(message []byte) {
	w.clientsMu.RLock()
	defer w.clientsMu.RUnlock()

	for conn := range w.clients {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			w.logger.Error("Error broadcasting message", zap.Error(err))
		}
	}
}
