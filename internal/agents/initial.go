package agents

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IverMortensen/aika/internal/wal"
)

// TODO: Find reasonable values bellow
const taskTTL int64 = 5        // seconds
const reclaimIterval int64 = 6 // seconds
const expiredQueueSize int64 = 100

const (
	claimed uint8 = iota
	reclaimed
	complete
)

type Task struct {
	img [32]byte
}

type ClaimedTask struct {
	img     string
	created int64
	TTL     int64 // TODO: TTL does not need to be in the struct
}

type WalEntry struct {
	entryType uint8
	data      []byte
}

type InitialBehavior struct {
	imageDir     string
	server       *http.Server
	wal          *wal.WAL
	imgIdx       atomic.Int64
	tasks        []Task
	mu           sync.Mutex
	claimedTasks map[string]*ClaimedTask
	expiredQueue chan *ClaimedTask
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

	// Create claimed and reclaimed tasks
	ib.claimedTasks = make(map[string]*ClaimedTask)
	ib.expiredQueue = make(chan *ClaimedTask, expiredQueueSize)

	// Replay the WAL
	ib.replayWAL()

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

	// Run periodic function for reclaiming expired tasks
	stop := make(chan struct{})
	go ib.periodicExpireClaimedTasks(time.Duration(reclaimIterval)*time.Second, stop)

	// Wait for context to cancel
	<-ctx.Done()

	// Shutdown server and close the queue
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	close(stop)
	defer cancel()
	ib.server.Shutdown(shutdownCtx)

	return nil
}

func (ib *InitialBehavior) replayWAL() error {
	log.Printf("Replaying WAL...")
	var maxIdx int64

	// Replay runs the provided function once per entry in the WAL
	err := ib.wal.Replay(func(data []byte) error {
		if len(data) == 0 {
			return fmt.Errorf("empty entry")
		}
		entryType := data[0]
		payload := data[1:]

		switch entryType {
		case claimed:
			ct := claimedTaskFromBytes(payload)
			ib.claimedTasks[ct.img] = &ct
			maxIdx++ // Update task array index
		case reclaimed:
			ct := claimedTaskFromBytes(payload)
			ib.claimedTasks[ct.img] = &ct
		case complete:
			delete(ib.claimedTasks, string(payload))
		}
		return nil
	})
	if err != nil {
		return err
	}

	ib.imgIdx.Store(maxIdx) // Set the index of the task array
	log.Printf("Finished replaying WAL")

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

// NOTE: Remove this
func printEntry(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty entry")
	}
	entryType := data[0]
	payload := data[1:]

	switch entryType {
	case claimed:
		ct := claimedTaskFromBytes(payload)
		fmt.Printf("CLAIMED: img=%s created=%d TTL=%d\n", ct.img, ct.created, ct.TTL)
	case complete:
		fmt.Printf("COMPLETE: img=%s\n", string(payload))
	case reclaimed:
		ct := claimedTaskFromBytes(payload)
		fmt.Printf("RECLAIMED: img=%s created=%d TTL=%d\n", ct.img, ct.created, ct.TTL)
	default:
		fmt.Printf("UNKNOWN entry type: %d\n", entryType)
	}
	return nil
}

func (ib *InitialBehavior) handleClaim(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)

	var imgName string
	entryType := claimed

	// Try to get a task from main task array
	imgName, ok := ib.pop()
	if !ok {
		// Task array empty
		select {
		// Try to get a task from expired task queue
		case task := <-ib.expiredQueue:
			imgName = task.img
			entryType = reclaimed
		default:
			// No more tasks
			log.Printf("No more images.")
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"image_path": "%s", "EOF":"%v"}`, "", true)
			return
		}
	}
	log.Printf("CLAIM: %v %v\n", imgName, r.Host)

	imagePath := ib.imageDir + imgName
	now := time.Now().Unix()

	// Add task to claimed tasks
	claimedTask := ClaimedTask{
		img:     imgName,
		TTL:     taskTTL, // Add 10 seconds of TTL
		created: now,
	}

	ib.mu.Lock()
	ib.claimedTasks[imgName] = &claimedTask
	ib.mu.Unlock()
	log.Printf("Moved %v to claimed tasks", imgName)

	// Add claimed task to WAL entry
	entry := WalEntry{
		entryType: entryType,
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
	imgName := filepath.Base(imgPath)
	if imgName == "" {
		http.Error(w, "Missing image in image path", http.StatusBadRequest)
		return
	}

	// Add WAL entry that task is complete
	entry := WalEntry{
		entryType: complete,
		data:      []byte(imgName),
	}
	ib.wal.Write(entry.toBytes())

	// Remove task from claimed tasks
	ib.mu.Lock()
	delete(ib.claimedTasks, imgName)
	ib.mu.Unlock()
}

// Periodic function for expiring claimed tasks to be claimed again
func (ib *InitialBehavior) periodicExpireClaimedTasks(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ib.expireClaimedTasks()
		case <-stop:
			return
		}
	}
}

// Move any claimed tasks that have expired to the expiredQueue so they can be reclaimed
func (ib *InitialBehavior) expireClaimedTasks() error {
	ib.mu.Lock()
	defer ib.mu.Unlock()

	// NOTE: Remove all the prints
	ib.wal.Replay(printEntry) // NOTE: Remove this

	now := time.Now().Unix()
	fmt.Println("Claimed tasks:")
	for key, task := range ib.claimedTasks {
		fmt.Printf("%v", task)
		if now-task.created >= task.TTL {
			select {
			case ib.expiredQueue <- task:
			default:
				// Channel full, leave task in claimedTasks
				continue
			}

			fmt.Printf(" TTL Expired")
			log.Printf("CLAIM EXPIRED: %v", key)
			delete(ib.claimedTasks, key)
		}
		fmt.Printf("\n")
	}

	return nil
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

func claimedTaskFromBytes(b []byte) ClaimedTask {
	img := string(bytes.Trim(b[:32], "\x00"))
	created := int64(binary.LittleEndian.Uint64(b[32:40]))
	ttl := int64(binary.LittleEndian.Uint64(b[40:48]))
	return ClaimedTask{
		img:     img,
		created: created,
		TTL:     ttl,
	}
}
