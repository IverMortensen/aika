package agents

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

type Task struct {
	img [32]byte
}

type InitialBehavior struct {
	imageDir string
	server   *http.Server
	imgIdx   atomic.Int64
	tasks    []Task
	// clamedTasks
	// reclaimedTasks
}

func NewInitialBehavior(imgDir string, walPath string, serverAddress string) (*InitialBehavior, error) {
	ib := &InitialBehavior{
		imageDir: imgDir,
	}

	// Fetch image names
	imgNames, err := fetchImgNames(imgDir)
	if err != nil {
		return &InitialBehavior{}, err
	}

	// Create and fill the tasks queue
	tasks := make([]Task, len(imgNames))
	for i, name := range imgNames {
		copy(tasks[i].img[:], name)
	}
	ib.tasks = tasks

	// Create the server
	mux := http.NewServeMux()
	mux.HandleFunc("/claim", ib.handleClaim)
	mux.HandleFunc("/complete", ib.handleComplete)

	ib.server = &http.Server{
		Addr:    serverAddress,
		Handler: mux,
	}

	return ib, nil
}

func (ib *InitialBehavior) Run(ctx context.Context) error {
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

	return nil
}

func fetchImgNames(imgDir string) ([]string, error) {
	f, err := os.Open(imgDir)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	return names, nil
}

func (ib *InitialBehavior) pop() (string, bool) {
	i := ib.imgIdx.Add(1) - 1
	if int(i) >= len(ib.tasks) {
		log.Printf("i: %v  q: %v", i, len(ib.tasks))
		return "", false
	}

	img := string(bytes.Trim(ib.tasks[i].img[:], "\x00"))

	return img, true
}

func (ib *InitialBehavior) handleClaim(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

	// Get the next file
	eof := false
	imagePath := ""
	image_name, ok := ib.pop()
	log.Printf("Popped file: %v\n", image_name)
	if !ok {
		log.Printf("No more images.")
		eof = true
	} else {
		imagePath = ib.imageDir + image_name
	}

	// Add image to claimed

	// Send file path
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"image_path": "%s", "EOF":"%v"}`, imagePath, eof)
}

func (ib *InitialBehavior) handleComplete(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

	// Check that method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}
