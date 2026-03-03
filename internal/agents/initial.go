package agents

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/IverMortensen/aika/internal/wal"
)

const (
	unclaimed uint8 = iota
	claimed
	reclaimed
)

type Task struct {
	img [32]byte
}

type ClaimedTask struct {
	img     string
	created int64
	TTL     int64
}

type WalEntry struct {
	entryType uint8
	data      []byte
}

type InitialBehavior struct {
	imageDir       string
	server         *http.Server
	wal            *wal.WAL
	imgIdx         atomic.Int64
	tasks          []Task
	claimedTasks   map[string]*ClaimedTask
	reclaimedTasks map[string]*ClaimedTask
}

func NewInitialBehavior(imgDir string, walPath string, serverAddress string) (*InitialBehavior, error) {
	ib := &InitialBehavior{
		imageDir: imgDir,
	}

	// Create/open the write ahead log
	wal, _, err := wal.Open(walPath)
	if err != nil {
		return &InitialBehavior{}, err
	}
	ib.wal = wal

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

	// Create claimed and unclaimed tasks
	ib.claimedTasks = make(map[string]*ClaimedTask)
	ib.reclaimedTasks = make(map[string]*ClaimedTask)

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
	// Get the next image from tasks
	i := ib.imgIdx.Add(1) - 1 // Atomic so each call gets unique index
	if int(i) >= len(ib.tasks) {
		log.Printf("i: %v  q: %v", i, len(ib.tasks))
		return "", false
	}
	img := string(bytes.Trim(ib.tasks[i].img[:], "\x00"))

	return img, true
}

// func printEntry(data []byte) error {
// 	ct := ClaimedTaskFromBytes(data)
// 	log.Printf("CT from WAL: %v", ct)
// 	return nil
// }

func (ib *InitialBehavior) handleClaim(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

	// Get the next image
	imgName, ok := ib.pop()
	log.Printf("Popped file: %v\n", imgName)
	if !ok {
		log.Printf("No more images.")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"image_path": "%s", "EOF":"%v"}`, "", true)
		// ib.wal.Replay(printEntry)
		return
	}
	imagePath := ib.imageDir + imgName

	now := time.Now().Unix()

	// Add task to claimed tasks
	claimedTask := ClaimedTask{
		img:     imgName,
		TTL:     now + 10, // Add 10 seconds of TTL
		created: now,
	}
	ib.claimedTasks[imgName] = &claimedTask
	log.Printf("Moved %v to claimed tasks", imgName)

	// Add claimed task to WAL entry
	entry := WalEntry{
		entryType: claimed,
		data:      claimedTask.toBytes(),
	}
	ib.wal.Write(entry.toBytes())

	// Send file path
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"image_path": "%s", "EOF":"%v"}`, imagePath, false)
}

func (ib *InitialBehavior) handleComplete(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

	// Check that method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

func (ct *ClaimedTask) toBytes() []byte {
	buf := make([]byte, 48) // 32 (img) + 8 (created) + 8 (TTL)
	copy(buf[:32], ct.img)
	binary.LittleEndian.PutUint64(buf[32:], uint64(ct.created))
	binary.LittleEndian.PutUint64(buf[40:], uint64(ct.TTL))
	return buf
}

func (w WalEntry) toBytes() []byte {
	return append([]byte{w.entryType}, w.data...)
}

func ClaimedTaskFromBytes(b []byte) ClaimedTask {
	img := string(bytes.Trim(b[:32], "\x00"))
	created := int64(binary.LittleEndian.Uint64(b[32:40]))
	ttl := int64(binary.LittleEndian.Uint64(b[40:48]))
	return ClaimedTask{
		img:     img,
		created: created,
		TTL:     ttl,
	}
}
