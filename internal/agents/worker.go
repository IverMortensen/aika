package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

const backoffDuration time.Duration = 2 * time.Second
const maxBackoffDuration time.Duration = 60 * time.Second

type WorkerBehavior struct {
	iaAddress string
	faAddress string
	client    *http.Client
}

func NewWorkerBehavior(iaAddress string, faAddress string) (*WorkerBehavior, error) {
	wb := &WorkerBehavior{
		client:    &http.Client{Timeout: 10 * time.Second},
		iaAddress: iaAddress,
		faAddress: faAddress,
	}

	return wb, nil
}

func (wb *WorkerBehavior) Run(ctx context.Context) error {

	// Work loop:
	//	Request work from initial agent
	//	 If no work: break
	// TODO: Find a way to handle the errors in the loop.
	// Should probably say something to the initial agent
	for {
		// Get an image path from initial agent
		var imgPath string
		var eof bool
		err := wb.retryWithBackoff(func() error { // Retry on error
			var e error
			imgPath, e = wb.getImgPath()
			if e == io.EOF { // Don't want to retry if there are no more images
				eof = true
				return nil
			}
			return e
		})
		if eof {
			log.Printf("No more images. Shutting down...")
			break
		}
		if err != nil {
			log.Printf("Failed to get image path from initial server: %v", err)
			return err
		}
		log.Printf("Received image: %v", imgPath)

		// Do the work
		res, err := runModel(imgPath)
		if err != nil {
			log.Printf("Failed to run model: %v", err)
			return err
		}
		log.Printf("Result: %v", res)

		// Send result to final agent (blocking)
		wb.retryWithBackoff(func() error {
			return wb.postLabel(imgPath, res)
		})

		// Send task complete to initial agent (blocking)
		wb.retryWithBackoff(func() error {
			return wb.postComplete(imgPath)
		})
	}

	return nil
}

func (wb *WorkerBehavior) getImgPath() (string, error) {
	ia_url := "http://" + wb.iaAddress + "/claim"

	// Get image from initial agent
	resp, err := http.Get(ia_url)
	if err != nil {
		return "", fmt.Errorf("No response: %v", err)
	}
	defer resp.Body.Close()

	// Check if there are unclaimed tasks/images left
	if resp.StatusCode == http.StatusNoContent {
		return "", fmt.Errorf("No tasks available, retrying...")
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Unexpected status code: %v", resp.StatusCode)
	}

	// Decode json response
	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("Failed to decode response: %v", err)
	}

	// Check if all images have been processed
	if result["EOF"] == "true" {
		return "", io.EOF
	}

	// Check if an image path was given
	imgPath, ok := result["image_path"]
	if !ok {
		return "", fmt.Errorf("No 'image_path' in response.")
	}

	return imgPath, nil
}

// Send the label and the path of a classified image to a final agent
func (wb *WorkerBehavior) postLabel(imgPath, label string) error {
	log.Printf("POST /submit %v", imgPath)
	return wb.postJSON("http://"+wb.faAddress+"/submit", map[string]string{"image_path": imgPath, "label": label})
}

// Send task complete to initial agent
func (wb *WorkerBehavior) postComplete(imgPath string) error {
	log.Printf("POST /complete %v", imgPath)
	return wb.postJSON("http://"+wb.iaAddress+"/complete", map[string]string{"image_path": imgPath})
}

func (wb *WorkerBehavior) postJSON(url string, data map[string]string) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to encode body: %v", err)
	}
	resp, err := wb.client.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to post to %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}
	return nil
}

func runModel(imgPath string) (string, error) {
	cmd := exec.Command("./model/venv/bin/python", "./model/classify.py", imgPath)

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

// TODO: Add retry to calls to final agent as well
func (wb *WorkerBehavior) retryWithBackoff(fn func() error) error {
	backoff := backoffDuration
	for {
		err := fn()
		if err == nil {
			return nil
		}
		log.Printf("Failed, retrying in %v: %v", backoff, err)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoffDuration {
			backoff = maxBackoffDuration
		}
	}
}
