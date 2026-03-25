package agents

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const processTTL = 24 * time.Hour

type AgentType string

const (
	Initial AgentType = "initial"
	Worker  AgentType = "worker"
	Final   AgentType = "final"
)

type Agent struct {
	config    *Config
	behaviour Behaviour
	logFile   *os.File
}

// Configurations common to all agents
type Config struct {
	AgentId string
	Name    string
	Type    AgentType
	LogFile string
}

// Methods that all agents must implement
type Behaviour interface {
	Run(ctx context.Context) error
}

/* Functions that all agents use */

func New(config *Config, behaviour Behaviour) *Agent {
	// Construct and return the agent
	return &Agent{
		config:    config,
		behaviour: behaviour,
	}
}

func (a *Agent) Start() error {
	// Set up logging
	if err := a.setUpLogging(); err != nil {
		return fmt.Errorf("failed to set up logging: %w", err)
	}

	// Create a context to be able to cancel the process
	// If the TTL expires the process is canceled automatically
	ctx, cancel := context.WithTimeout(context.Background(), processTTL)
	defer cancel()

	// Create a signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Run the agent
	errChan := make(chan error, 1)
	go func() {
		errChan <- a.behaviour.Run(ctx)
	}()

	// Wait for any signals
	select {
	case err := <-errChan:
		cancel()
		log.Printf("[%s] Stopped: %v", a.config.Name, err)
	case sig := <-sigChan:
		log.Printf("[%s] Received signal %v, shutting down...", a.config.Name, sig)
		cancel()
		err := <-errChan // Wait for run to finish
		log.Printf("[%s] Stopped: %v", a.config.Name, err)
	case <-ctx.Done():
		log.Printf("[%s] TTL expired, shutting down...", a.config.Name)
		cancel()
		<-errChan
	}

	// Close log file
	if a.logFile != nil {
		a.logFile.Close()
	}

	return nil
}

func (a *Agent) setUpLogging() error {
	// No log file
	if a.config.LogFile == "" {
		return nil
	}

	// Try to create/open log file
	f, err := os.OpenFile(a.config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
	if err != nil {
		return err
	}

	// Set logger to output to log file
	a.logFile = f
	log.SetOutput(f)

	return nil
}
