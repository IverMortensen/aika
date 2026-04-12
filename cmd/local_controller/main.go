package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
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

type config struct {
	ClusterControllers []string      `json:"cluster_controllers"`
	Agents             []agentConfig `json:"agents"`
}

func main() {
	configFilePath := flag.String("config", "", "Path to config file describing what agents to run")
	logFile := flag.String("log-file", "./data/logs/local-controller", "Path to log file")
	statusPort := flag.String("status-port", "7000", "Port for the status HTTP server")
	flag.Parse()
	if *configFilePath == "" {
		log.Fatal("--config is required")
	}

	// Set up logging
	err := setUpLogging(*logFile)
	if err != nil {
		log.Printf("Failed to set up logging: %v", err)
	}

	// Open config file
	configFile, err := os.ReadFile(*configFilePath)
	if err != nil {
		log.Printf("Failed to open config file: %v", err)
	}

	// Extract content of config file
	var cfg config
	if err := json.Unmarshal(configFile, &cfg); err != nil {
		log.Printf("Failed to decode config file: %v", err)
		return
	}
	if len(cfg.Agents) < 1 {
		log.Printf("No agent configurations in config file.")
		return
	}

	// Start status server
	go startStatusServer(*statusPort)

	// Start agents
	var wg sync.WaitGroup
	for _, config := range cfg.Agents {
		wg.Add(1)
		go func(c agentConfig) {
			defer wg.Done()
			supervise(c)
		}(config)
	}
	wg.Wait()
}

func startStatusServer(port string) {
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	log.Printf("[local-controller] Status server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("[local-controller] Status server failed: %v", err)
	}
}

// Start and supervise a single agent
func supervise(config agentConfig) {
	log.Printf("Monitoring agent: %v", config.Binary)
	for {
		cmd := exec.Command(config.Binary, config.Flags...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Start the agent
		cmd.Run()

		// Agent failed, wait a bit and restart
		log.Printf("'%s' failed, restarting in %v", config.Binary, restartDelay)
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
