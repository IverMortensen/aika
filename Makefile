
build-initial:
	go build -o ./bin/inf_3203_initial_agent ./cmd/initial-agent/main.go

initial: build-initial
	./bin/inf_3203_initial_agent \
		-image-dir /share/inf3203/unlabeled_images/ \
		-queue-path ./data/queues/initial_queue.log \
		-server-address 0.0.0.0:5000 \
		-agent-id initial_3333 \
		-log-file ./data/logs/initial-3333.log

