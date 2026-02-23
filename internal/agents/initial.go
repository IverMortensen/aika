package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/IverMortensen/aika/internal/queue"
)

type InitialBehavior struct {
	imageDir string
	queue    *queue.Queue
	server   *http.Server
}

func NewInitialBehavior(imageDir string, queuePath string, serverAddress string) (*InitialBehavior, error) {
	ib := &InitialBehavior{imageDir: imageDir}

	// Create the persistent queue
	pq, err := queue.NewPersistentQueue()
	if err != nil {
		return nil, fmt.Errorf("Failed to create queue: %w", err)
	}
	ib.queue = pq

	// Create the server
	mux := http.NewServeMux()
	mux.HandleFunc("/claim", ib.handleClaim)

	ib.server = &http.Server{
		Addr:    serverAddress,
		Handler: mux,
	}

	return ib, nil
}

func (ib *InitialBehavior) handleClaim(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %v", r)

	w.Header().Set("Content-type", "application/json")

	json.NewEncoder(w).Encode(`{"res":"RESPONSE!!"}`)
}

func (ib *InitialBehavior) Run(ctx context.Context) error {
	// Add images to queue

	// Start server
	go func() {
		if err := ib.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Handle requests

	// Wait for context to cancel
	<-ctx.Done()

	// Shutdown server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ib.server.Shutdown(shutdownCtx)

	return nil
}
