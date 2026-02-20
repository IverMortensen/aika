package agents

import (
	"fmt"

	"github.com/IverMortensen/aika/internal/queue"
	"github.com/IverMortensen/aika/internal/server"
)

type InitialBehavior struct {
	imageDir string
	queue    *queue.Queue
	server   *server.Server
}

func NewInitialBehavior(imageDir string, queuePath string, serverAddress string) (*InitialBehavior, error) {
	// Create the persistent queue
	pq, err := queue.NewPersistentQueue()
	if err != nil {
		return nil, fmt.Errorf("Failed to create queue: %w", err)
	}

	// Create the server
	server, err := server.NewServer(serverAddress)
	if err != nil {
		return nil, fmt.Errorf("Failed to create server: %w", err)
	}

	return &InitialBehavior{
		imageDir: imageDir,
		queue:    pq,
		server:   server,
	}, nil
}

func (ib *InitialBehavior) Run() error {
	return nil
}
