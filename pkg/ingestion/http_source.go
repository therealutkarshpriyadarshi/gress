package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/stream"
	"go.uber.org/zap"
)

// HTTPSource ingests events from HTTP POST requests
type HTTPSource struct {
	addr          string
	path          string
	server        *http.Server
	logger        *zap.Logger
	eventsIngested int64
}

// NewHTTPSource creates a new HTTP source
func NewHTTPSource(addr, path string, logger *zap.Logger) *HTTPSource {
	return &HTTPSource{
		addr:   addr,
		path:   path,
		logger: logger,
	}
}

// Start begins accepting HTTP requests
func (h *HTTPSource) Start(ctx context.Context, output chan<- *stream.Event) error {
	h.logger.Info("Starting HTTP source",
		zap.String("addr", h.addr),
		zap.String("path", h.path))

	mux := http.NewServeMux()
	mux.HandleFunc(h.path, func(w http.ResponseWriter, r *http.Request) {
		h.handleRequest(w, r, output)
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":          "healthy",
			"events_ingested": atomic.LoadInt64(&h.eventsIngested),
		})
	})

	h.server = &http.Server{
		Addr:         h.addr,
		Handler:      h.loggingMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		h.Stop()
	}()

	return nil
}

// handleRequest handles an HTTP POST request
func (h *HTTPSource) handleRequest(w http.ResponseWriter, r *http.Request, output chan<- *stream.Event) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Error reading request body", zap.Error(err))
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Parse JSON body
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		h.logger.Warn("Invalid JSON, treating as raw data")
		data = body
	}

	// Extract headers
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	// Create event
	event := &stream.Event{
		Value:     data,
		EventTime: time.Now(),
		Headers:   headers,
	}

	// Check if batch of events
	if dataMap, ok := data.(map[string]interface{}); ok {
		if events, ok := dataMap["events"].([]interface{}); ok {
			// Batch mode
			for _, e := range events {
				batchEvent := &stream.Event{
					Value:     e,
					EventTime: time.Now(),
					Headers:   headers,
				}
				select {
				case output <- batchEvent:
					atomic.AddInt64(&h.eventsIngested, 1)
				default:
					h.logger.Warn("Output channel full, dropping event")
				}
			}

			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":         "accepted",
				"events_ingested": len(events),
			})
			return
		}
	}

	// Single event mode
	select {
	case output <- event:
		atomic.AddInt64(&h.eventsIngested, 1)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "accepted",
		})
	default:
		h.logger.Warn("Output channel full, rejecting event")
		http.Error(w, "Service busy, try again later", http.StatusServiceUnavailable)
	}
}

// loggingMiddleware adds request logging
func (h *HTTPSource) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		h.logger.Debug("HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.Duration("duration", time.Since(start)))
	})
}

// Stop stops the HTTP server
func (h *HTTPSource) Stop() error {
	h.logger.Info("Stopping HTTP source")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.server.Shutdown(ctx)
}

// Name returns the source name
func (h *HTTPSource) Name() string {
	return fmt.Sprintf("http-%s", h.path)
}

// Partitions returns the number of partitions (1 for HTTP)
func (h *HTTPSource) Partitions() int32 {
	return 1
}

// GetEventsIngested returns the total number of events ingested
func (h *HTTPSource) GetEventsIngested() int64 {
	return atomic.LoadInt64(&h.eventsIngested)
}
