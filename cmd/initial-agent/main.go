package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func runModel(imgPath string) (string, error) {
	cmd := exec.Command("/users/imo059/3203/model/venv/bin/python", "/users/imo059/3203/model/classify.py", imgPath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Model failed: %w\nstderr: %s", err, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("Unexpected output format: %s", stdout.String())
	}

	return lines[len(lines)-1], nil
}

func main() {
	res, err := runModel("/share/inf3203/unlabeled_images/199.JPEG")
	if err != nil {
		fmt.Println("Error running model:", err)
		return
	}

	fmt.Println("Result:\n" + res)
}
