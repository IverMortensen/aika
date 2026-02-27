package main

import (
	"flag"
	"log"

	"github.com/IverMortensen/aika/internal/agents"
)

func main() {
	// Parse all flags
	queuePath := flag.String("queue-path", "./data/queues/final.log", "Persistent queue path")
	serverAddress := flag.String("server-address", ":6000", "Server address")
	agentId := flag.String("agent-id", "final-agent", "Final agent's id")
	logFile := flag.String("log-file", "./data/logs/final-agent.log", "Path to log file.")
	flag.Parse()

	// Create behavior of an final agent
	behavior, err := agents.NewFinalBehavior(*queuePath, *serverAddress)
	if err != nil {
		log.Fatalf("Failed to create behavior for final agent: %v", err)
	}

	// Set configuration
	cfg := &agents.Config{
		AgentId: *agentId,
		Name:    "Final-agent",
		Type:    agents.Final,
		LogFile: *logFile,
	}

	// Construct the agent
	a := agents.New(cfg, behavior)

	// Start agent
	if err := a.Start(); err != nil {
		log.Fatalf("Agent failed: %v", err)
	}
}
