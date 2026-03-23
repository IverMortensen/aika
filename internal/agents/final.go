package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/IverMortensen/aika/internal/wal"
)

type faWalEntry struct {
	ImgPath string `json:"img_path"`
	Label   string `json:"label"`
}

// TODO: Find good values for these:
const flushTreeInterval = 5
const dedupMapSize = 100
const treeSize = 100

type FinalBehavior struct {
	server     *http.Server
	wal        *wal.WAL
	dedupMap   map[string]string
	tree       map[string][]string
	mu         sync.RWMutex
	outputPath string
}

func NewFinalBehavior(outputPath string, walPath string, serverAddress string) (*FinalBehavior, error) {
	fb := &FinalBehavior{
		outputPath: outputPath,
	}

	// Create/open the write ahead log
	wal, _, err := wal.Open(walPath)
	if err != nil {
		return &FinalBehavior{}, err
	}
	fb.wal = wal

	// Create deduplication map and in-memory tree
	fb.dedupMap = make(map[string]string, dedupMapSize)
	fb.tree = make(map[string][]string, treeSize)

	// Replay WAL
	if err := fb.replayWAL(); err != nil {
		log.Printf("Error occurred while replaying WAL: %v", err)
	}

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

	// Run periodic function for writing in-memory results tree to file
	stop := make(chan struct{})
	go fb.periodicFlushTree(time.Duration(flushTreeInterval)*time.Second, stop)

	// Wait for context to cancel
	<-ctx.Done()

	// Clean shutdown of final agent
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	close(stop)
	fb.flushTree() // Final flush before shutdown
	defer cancel()
	fb.server.Shutdown(shutdownCtx)

	return nil
}

func (fb *FinalBehavior) replayWAL() error {
	return fb.wal.Replay(func(data []byte) error {
		var entry faWalEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return err
		}
		if _, exists := fb.dedupMap[entry.ImgPath]; !exists {
			fb.dedupMap[entry.ImgPath] = entry.Label
			fb.tree[entry.Label] = append(fb.tree[entry.Label], entry.ImgPath)
		}
		return nil
	})
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

	// Check if already submitted
	fb.mu.RLock()
	_, exists := fb.dedupMap[imgPath]
	fb.mu.RUnlock()
	if exists {
		log.Printf("Duplicate received: %v", imgPath)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Write submit to WAL
	if err := fb.appendToWAL(imgPath, label); err != nil {
		log.Printf("WAL write failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	log.Printf("SUBMIT: {%v:%v}\n", imgPath, label)
	fmt.Printf("SUBMIT: {%v:%v}\n", imgPath, label)

	// Store submit in dedup map and in-memory results tree
	fb.mu.Lock()
	if _, exists := fb.dedupMap[imgPath]; !exists {
		fb.dedupMap[imgPath] = label
		fb.tree[label] = append(fb.tree[label], imgPath)
	}
	fb.mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func (fb *FinalBehavior) appendToWAL(imgPath string, label string) error {
	// TODO: Final agent uses json while initial agent uses bytes. Find a common encoding
	entry := faWalEntry{ImgPath: imgPath, Label: label}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return fb.wal.Write(data)
}

// Periodic function for storing results to output file
// NOTE: Pattern used in initial agent. Make a common function
func (fb *FinalBehavior) periodicFlushTree(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fb.flushTree()
		case <-stop:
			return
		}
	}
}

// Flush the in-memory tree to file
func (fb *FinalBehavior) flushTree() {
	// Make a copy and write the copy to disk to not hold the lock while writing
	fb.mu.RLock()
	snapshot := make(map[string][]string, len(fb.tree))
	for k, v := range fb.tree {
		cp := make([]string, len(v))
		copy(cp, v)
		snapshot[k] = cp
	}
	fb.mu.RUnlock()

	// Encode, format and write the data to disk
	// Temp file is used so the actual file never has partial written data
	data, _ := json.MarshalIndent(snapshot, "", "  ")
	tmp := fb.outputPath + ".tmp"
	os.WriteFile(tmp, data, 0644)
	os.Rename(tmp, fb.outputPath)
}
