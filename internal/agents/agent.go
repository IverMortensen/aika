package agents

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type AgentType string

const (
	Initial AgentType = "initial"
	Worker  AgentType = "worker"
	Final   AgentType = "final"
)

// Life cycle of all agents
type Agent struct {
	id        string
	config    *Config
	behaviour Behaviour
	ctx       context.Context
	cancel    context.CancelFunc
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
	// Create a context to be able to cancel the process
	ctx, cancel := context.WithCancel(context.Background())

	// Construct and return the agent
	return &Agent{
		id:        config.AgentId,
		config:    config,
		behaviour: behaviour,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (a *Agent) Start() error {
	// Set up logging
	if err := a.setUpLogging(); err != nil {
		return fmt.Errorf("failed to set up logging: %w", err)
	}

	// Create a signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the agent
	errChan := make(chan error, 1)
	go func() {
		errChan <- a.behaviour.Run(a.ctx)
	}()

	// Wait for any signals
	select {
	case err := <-errChan:
		log.Printf("[%s] Stopped: %v", a.config.Name, err)
	case sig := <-sigChan:
		log.Printf("[%s] Received signal %v, shutting down...", a.config.Name, sig)
		a.cancel()
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
	log.SetOutput(f)

	return nil
}
