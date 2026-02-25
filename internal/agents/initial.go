package agents

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/IverMortensen/aika/internal/queue"
)

type InitialBehavior struct {
	imagePath string
	queue     *queue.PersistentQueue
	server    *http.Server
}

func NewInitialBehavior(imageDir string, queuePath string, serverAddress string) (*InitialBehavior, error) {
	ib := &InitialBehavior{imagePath: imageDir}

	// Create the persistent queue
	pq, err := queue.NewPersistentQueue(imageDir)
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

	// Get the next file
	image, err := ib.queue.Pop()
	if err != nil {
		log.Printf("Failed to read next image name: %v", err)
	}

	// Mark file as in progress

	// Send file name
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"image_name": "%s"}`, image)
}

func (ib *InitialBehavior) Run(ctx context.Context) error {
	// Get the image directory
	_, err := os.Open(ib.imagePath)
	if err != nil {
		log.Printf("Failed to read image directory: %v", err)
	}

	// Add images to queue

	// Start server
	go func() {
		if err := ib.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Failed to start server: %v", err)
		}
	}()

	// Wait for context to cancel
	<-ctx.Done()

	// Shutdown server and close the queue
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ib.server.Shutdown(shutdownCtx)
	ib.queue.Close()

	return nil
}
