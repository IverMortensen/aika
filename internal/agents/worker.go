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
)

type WorkerBehavior struct {
	iaAddress string
	faAddress string
}

func NewWorkerBehavior(iaAddress string, faAddress string) (*WorkerBehavior, error) {
	wb := &WorkerBehavior{
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
		imgPath, err := wb.getImgPath()
		if err != nil {
			log.Printf("Failed to get image path from initial server: %v", err)
			return err
		} else if err == io.EOF { // No more images
			break
		}

		//	Do the work
		res, err := runModel(imgPath)
		if err != nil {
			log.Printf("Failed to run model: %v", err)
			return err
		}

		//	Send result to final agent
		wb.postLabel(res, imgPath)
		log.Printf("Result: %v", res)
	}

	return nil
}

func (wb *WorkerBehavior) getImgPath() (string, error) {
	resp, err := http.Get("http://" + wb.iaAddress + "/claim")
	if err != nil {
		return "", fmt.Errorf("No response: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Unexpected status code: %v", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("Failed to decode response: %v", err)
	}

	// Check if there are more images to process
	if result["eof"] == "true" {
		return "", io.EOF
	}

	imgPath, ok := result["image_path"]
	if !ok {
		return "", fmt.Errorf("No 'image_path' in response.")
	}

	return imgPath, nil
}

// Send the label and the path of a classified image to a final agent
func (wb *WorkerBehavior) postLabel(imgPath string, label string) error {
	fa_url := "http://" + wb.faAddress + "/submit"

	// Format message containing path and label into json
	data := map[string]string{"image_path": imgPath, "label": label}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("Failed to encode result: %v", err)
	}

	// Send message
	resp, err := http.Post(fa_url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("Failed to send label: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected status code: %v", resp.StatusCode)
	}

	return nil
}

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

func tmp() {
	res, err := runModel("/share/inf3203/unlabeled_images/199.JPEG")
	if err != nil {
		fmt.Println("Error running model:", err)
		return
	}

	fmt.Println("Result:\n" + res)
}
