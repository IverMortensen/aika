package main

import (
	"flag"
	"log"

	"github.com/IverMortensen/aika/internal/agents"
)

func main() {
	// Parse all flags
	iaAddress := flag.String("ia-address", ":5000", "Address of the initial agent's server.")
	faAddress := flag.String("fa-address", ":6000", "Address of the final agent's server.")
	agentId := flag.String("agent-id", "worker-agent", "Worker agent's id")
	logFile := flag.String("log-file", "./data/logs/worker-agent.log", "Path to log file.")
	flag.Parse()

	// Create behavior of an initial agent
	behavior, err := agents.NewWorkerBehavior(*iaAddress, *faAddress)
	if err != nil {
		log.Fatalf("Failed to create behavior for worker agent: %v", err)
	}

	// Set configuration
	cfg := &agents.Config{
		AgentId: *agentId,
		Name:    "Worker-agent",
		Type:    agents.Worker,
		LogFile: *logFile,
	}

	// Construct the agent
	a := agents.New(cfg, behavior)

	// Start agent
	if err := a.Start(); err != nil {
		log.Fatalf("Agent failed: %v", err)
	}
}
