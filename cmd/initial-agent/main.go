package main

import (
	"flag"
	"log"

	"github.com/IverMortensen/aika/internal/agents"
)

func main() {
	// Parse all flags
	imageDir := flag.String("image-dir", "/share/inf3203/unlabeled_images/", "Path to image directory.")
	queuePath := flag.String("queue-path", "./data/queues/initial_queue.log", "Persistent queue path")
	serverAddress := flag.String("server-address", ":5000", "Server address")
	agentId := flag.String("agent-id", "initial-agent", "Initial agent's id")
	logFile := flag.String("log-file", "./data/logs/initial-agent.log", "Path to log file.")
	flag.Parse()

	// Create behavior of an initial agent
	behavior, err := agents.NewInitialBehavior(*imageDir, *queuePath, *serverAddress)
	if err != nil {
		log.Fatalf("Failed to create behavior for initial agent: %v", err)
	}

	// Set configuration
	cfg := &agents.Config{
		AgentId: *agentId,
		Name:    "Initial-agent",
		Type:    agents.Initial,
		LogFile: *logFile,
	}

	// Construct the agent
	a := agents.New(cfg, behavior)

	// Start agent
	if err := a.Start(); err != nil {
		log.Fatalf("Agent failed: %v", err)
	}
}
