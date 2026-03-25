package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

const restartDelay = 1 * time.Second

type agentConfig struct {
	Binary string   `json:"binary"`
	Flags  []string `json:"flags"`
}

func main() {
	configFilePath := flag.String("config", "", "Path to config file describing what agents to run")
	logFile := flag.String("log-file", "./data/logs/local-controller", "Path to log file")
	flag.Parse()
	if *configFilePath == "" {
		log.Fatal("--config is required")
	}

	// Set up logging
	setUpLogging(*logFile)

	// Open config file
	configFile, err := os.ReadFile(*configFilePath)
	if err != nil {
		log.Printf("Failed to open config file: %v", err)
	}

	// Extract agent configurations
	var agentConfigs []agentConfig
	if err := json.Unmarshal(configFile, &agentConfigs); err != nil {
		log.Printf("Failed to decode config file: %v", err)
		return
	}
	if len(agentConfigs) < 1 {
		log.Printf("No agent configurations in config file.")
		return
	}

	// Start agents
	var wg sync.WaitGroup
	for _, config := range agentConfigs {
		wg.Add(1)
		go func(c agentConfig) {
			defer wg.Done()
			supervise(c)
		}(config)
	}
	wg.Wait()
}

func supervise(config agentConfig) {
	for {
		cmd := exec.Command(config.Binary, config.Flags...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Start the agent
		err := cmd.Run()
		if err == nil { // Agent exits voluntary
			log.Printf("'%s' complete", config.Binary)
			return
		}

		// Agent failed, wait a bit and restart
		log.Printf("'%s' failed, restarting in %vs", config.Binary, restartDelay)
		time.Sleep(restartDelay)
	}
}

func setUpLogging(logFile string) error {
	// No log file
	if logFile == "" {
		return nil
	}

	// Try to create/open log file
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
	if err != nil {
		return err
	}

	// Set logger to output to log file
	log.SetOutput(f)

	return nil
}
