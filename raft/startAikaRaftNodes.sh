#!/bin/bash

# Start Aika Raft-nodes on the IFI cluster

if [ -z "$1" ]; then
    echo "Usage: $0 <number_of_nodes>"
    exit 1
fi

TOTAL_NODES=$1

# ---------------------------
# Fetch available hosts
# ---------------------------
mapfile -t nodes < <(/share/ifi/available-nodes.sh | head -n "$TOTAL_NODES")

echo "Antall noder: ${#nodes[@]}"

if [ ${#nodes[@]} -eq 0 ]; then
    echo "Ingen noder tilgjengelige."
    exit 1
fi


# ---------------------------
# Find free port
# ---------------------------
check_port_availability() {
    local node="$1"
    local port

    while :; do
        port=$(shuf -i 49631-65535 -n 1)

        if ! ssh "$USER@$node" "nc -z localhost $port" \
            >/dev/null 2>&1; then
            echo "$port"
            return
        fi
    done
}


# ---------------------------
# Create host:port list
# ---------------------------
mkdir -p data
> data/activehostport.txt

for node in "${nodes[@]}"; do
    port=$(check_port_availability "$node")
    echo "$node:$port" >> data/activehostport.txt
done


mapfile -t NODES < data/activehostport.txt

REMOTE_PROJECT_PATH="/mnt/users/$USER/Aika/src"

echo "Cluster membership:"
printf '%s\n' "${NODES[@]}"

# ---------------------------
# Start ALL nodes
# ---------------------------
for NODE in "${NODES[@]}"; do

    HOST=$(echo "$NODE" | cut -d':' -f1)
    PORT=$(echo "$NODE" | cut -d':' -f2)

    # ---------------------------
    # Build peer list (exclude self)
    # ---------------------------
    PEERS=()

    for OTHER in "${NODES[@]}"; do
        if [ "$OTHER" != "$NODE" ]; then
            PEERS+=("\"$OTHER\"")
        fi
    done


    echo "Starting node $HOST:$PORT"

    ssh -n "$USER@$HOST" "
        cd $REMOTE_PROJECT_PATH &&
        nohup python3 inf3203_startupRaftNodes.py \
            $HOST \
            $PORT \
        > node.log 2>&1 < /dev/null &
    " &

done

wait

echo "All Raft nodes started."