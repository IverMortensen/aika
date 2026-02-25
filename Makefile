
build-initial:
	go build -o ./bin/inf_3203_initial_agent ./cmd/initial-agent/main.go

initial: build-initial
	./bin/inf_3203_initial_agent \
		-image-dir /share/inf3203/unlabeled_images/ \
		-queue-path ./data/queues/initial_queue.log \
		-server-address 0.0.0.0:5000 \
		-agent-id test_initial \
		-log-file ./data/logs/test_initial.log

build-worker:
	go build -o ./bin/inf_3203_worker_agent ./cmd/worker-agent/main.go

worker: build-worker
	./bin/inf_3203_worker_agent \
		-ia-address 0.0.0.0:5000 \
		-fa-address 0.0.0.0:6000 \
		-agent-id test_worker \
		-log-file ./data/logs/test_worker.log
