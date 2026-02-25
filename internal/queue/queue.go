package queue

import (
	"fmt"
	"log"
	"os"
	"sync"
)

type PersistentQueue struct {
	dirPath       string
	dirFile       *os.File // TODO: Remove!
	queueFile     string
	processedFile string
	fileChan      chan string
	processed     map[string]bool
	mu            sync.Mutex
	closeOnce     sync.Once
}

func NewPersistentQueue(dirPath string) (*PersistentQueue, error) {
	log.Println("Creating queue...")
	queue := PersistentQueue{
		dirPath: dirPath,
	}
	// TODO: Don't do this:
	dir, err := os.Open(dirPath)
	if err != nil {
		log.Printf("Failed to open dir for queue: %v", err)
	}
	queue.dirFile = dir

	// Load processed files

	// Create new queue if it doesn't exist

	// Start feeding channel from file queue

	return &queue, nil
}

// Pop a value from the queue
func (pq *PersistentQueue) Pop() (string, error) {
	// TODO: Don't do this:
	file, err := pq.dirFile.Readdirnames(1)
	if err != nil {
		return "", fmt.Errorf("Failed to get next file in queue: %v", err)
	}
	log.Printf("Popped file: %v", file[0])

	return file[0], nil
}

// Close the queue
func (pq *PersistentQueue) Close() error {
	return nil
}
