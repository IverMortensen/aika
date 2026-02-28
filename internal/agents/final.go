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

type FinalBehavior struct {
	queue  *queue.PersistentQueue
	server *http.Server
}

func NewFinalBehavior(queuePath string, serverAddress string) (*FinalBehavior, error) {
	fb := &FinalBehavior{}

	// Create queue
	// pq := queue.NewPersistentQueue()
	// fb.queue = queue

	// Create server
	mux := http.NewServeMux()
	mux.HandleFunc("/submit", fb.handleSubmit)

	fb.server = &http.Server{
		Addr:    serverAddress,
		Handler: mux,
	}

	return fb, nil
}

func (fb *FinalBehavior) Run(ctx context.Context) error {
	// Start server
	go func() {
		if err := fb.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Failed to start server: %v", err)
		}
	}()

	// Wait for context to cancel
	<-ctx.Done()

	// Shutdown server and close the queue
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fb.server.Shutdown(shutdownCtx)
	fb.queue.Close()

	return nil
}

func (fb *FinalBehavior) handleSubmit(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data map[string]string
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "JSON decode error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	imgPath, ok := data["image_path"]
	if !ok {
		http.Error(w, "Missing required field 'image_path'", http.StatusBadRequest)
		return
	}
	label, ok := data["label"]
	if !ok {
		http.Error(w, "Missing required field 'label'", http.StatusBadRequest)
		return
	}

	// Store label and image path in queue
	log.Printf("Storing {%v:%v}\n", imgPath, label)
	fmt.Printf("Storing {%v:%v}\n", imgPath, label)
}
