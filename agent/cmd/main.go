package main

import (
	"os"
	"strconv"
	"fmt"
	"github.com/Tuma78/agent/internal"
)


func main() {
	serverURL := os.Getenv("ORCHESTRATOR_URL")
	if serverURL == "" {
		fmt.Println("ORCHESTRATOR_URL is not set")
		os.Exit(1)
	}

	workersStr := os.Getenv("COMPUTING_POWER")
	workers, err := strconv.Atoi(workersStr)
	if err != nil || workers <= 0 {
		workers = 1 
	}

	agent := agent.NewAgent(serverURL, workers)
	agent.Run()
}