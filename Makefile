MODEL_DIR := ./model/
VENV_DIR := $(MODEL_DIR)/venv/
PYTHON := $(VENV_DIR)/bin/python
PIP := $(VENV_DIR)/bin/pip

WORKER_NAME ?= test_worker

# .PHONY build-initial initial build-worker worker build-final final

# Create the python venv for the image model
$(VENV_DIR):
	python3.12 -m venv $(VENV_DIR)
	$(PIP) install -r $(MODEL_DIR)/requirements.txt

build-initial:
	go build -o ./bin/inf_3203_initial_agent ./cmd/initial-agent/main.go

initial: build-initial
	./bin/inf_3203_initial_agent \
		-image-dir ./static/test_images/ \
		-wal-path ./data/wal/initial.wal \
		-server-address 0.0.0.0:5001 \
		-agent-id test_initial \
		-log-file ./data/logs/test_initial.log

build-worker: $(VENV_DIR)
	go build -o ./bin/inf_3203_worker_agent ./cmd/worker-agent/main.go

worker: build-worker
	./bin/inf_3203_worker_agent \
		-ia-address 0.0.0.0:5001 \
		-fa-address 0.0.0.0:6000 \
		-agent-id $(WORKER_NAME) \
		-log-file ./data/logs/$(WORKER_NAME).log

build-final:
	go build -o ./bin/inf_3203_final_agent ./cmd/final-agent/main.go

final: build-final
	./bin/inf_3203_final_agent \
		-output-path ./data/result.json \
		-wal-path ./data/wal/final.wal \
		-server-address 0.0.0.0:6000 \
		-agent-id test_initial \
		-log-file ./data/logs/test_final.log

build-lc:
	go build -o ./bin/inf_3203_local_controller ./cmd/local_controller/main.go

lc:
	./bin/inf_3203_local_controller \
	-config ./static/configs/lc-test.json

all: build-initial build-worker build-final build-lc

clean:
	rm -f data/logs/*.log
	rm -f data/wal/*.wal
	rm data/result.json
