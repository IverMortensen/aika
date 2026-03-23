package main

import (
	"flag"
	"log"

	"github.com/IverMortensen/aika/internal/agents"
)

func main() {
	// Parse all flags
	outputPath := flag.String("output-path", "./data/result.json", "Path to output json file")
	walPath := flag.String("wal-path", "./data/wal/final.wal", "Write ahead log path")
	serverAddress := flag.String("server-address", ":6000", "Server address")
	agentId := flag.String("agent-id", "final-agent", "Final agent's id")
	logFile := flag.String("log-file", "./data/logs/final-agent.log", "Path to log file.")
	flag.Parse()

	// Create behavior of an final agent
	behavior, err := agents.NewFinalBehavior(*outputPath, *walPath, *serverAddress)
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
