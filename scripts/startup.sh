#!/bin/bash

# =============================================================================
# Startup script for the Aika distributed image labelling system
# =============================================================================

# --- Configuration -----------------------------------------------------------
NUM_CC_NODES=3 # Should be odd for Raft majority voting
MAX_LOAD=0.4

# --- Directories -------------------------------------------------------------
PROJECT_DIR="/mnt/users/imo059/3203/aika/"
BIN_DIR="$PROJECT_DIR/bin"
CMD_DIR="$PROJECT_DIR/cmd"
DATA_DIR="$PROJECT_DIR/data"
RAFT_DIR="$PROJECT_DIR/raft"

# --- Clean up ----------------------------------------------------------------
echo "Cleaning up..."

rm -f "$DATA_DIR/logs/*.log"
rm -f "$DATA_DIR/wal/*.wal"
rm -f "$DATA_DIR/lc_configs/*.json"
rm -f "$DATA_DIR/result.json"
rm -f "$DATA_DIR/activehostport.txt"
rm -f "$DATA_DIR/worker_nodes.txt"

echo "Done."
echo ""

# --- Compile -----------------------------------------------------------------
echo "Compiling binaries..."

go build -o "$BIN_DIR/inf_3203_initial_agent" "$CMD_DIR/initial-agent/main.go"
go build -o "$BIN_DIR/inf_3203_worker_agent" "$CMD_DIR/worker-agent/main.go"
go build -o "$BIN_DIR/inf_3203_final_agent" "$CMD_DIR/final-agent/main.go"
go build -o "$BIN_DIR/inf_3203_local_controller" "$CMD_DIR/local_controller/main.go"

echo "Done."
echo ""

# --- Find available nodes ----------------------------------------------------
echo "Finding available nodes..."

get_available_nodes() {
    local hosts
    hosts="$(shuf /share/compute-nodes.txt)"

    local filtered=()
    for H in $hosts; do
        if echo "$H" | grep -qE '^c6-'; then
            continue
        fi
        if grep -qF "$H" /share/exclude-nodes.txt 2>/dev/null; then
            continue
        fi
        filtered+=("$H")
    done

    for H in "${filtered[@]}"; do
        ssh -o ConnectTimeout=1 -o ConnectionAttempts=1 -x "$H" \
            "cat /proc/loadavg /proc/sys/kernel/hostname | tr '\n' ' ' | awk -v max_load=$MAX_LOAD '\$1+0 < max_load {printf \"%s %s\n\", \$1, \$6}'" 2>/dev/null &
    done | sort -n | awk '{print $2}' | sed 's/.ifi.uit.no//'
    wait
}

mapfile -t AVAILABLE_NODES < <(get_available_nodes)

if [ ${#AVAILABLE_NODES[@]} -eq 0 ]; then
    echo "No available nodes found, exiting."
    exit 1
fi

echo "Found ${#AVAILABLE_NODES[@]} available nodes:"
for node in "${AVAILABLE_NODES[@]}"; do
    echo "    $node"
done
echo "Done."
echo ""

# --- Split nodes into CC and worker nodes ------------------------------------
echo "Splitting nodes..."

CC_NODES=("${AVAILABLE_NODES[@]:0:$NUM_CC_NODES}")
WORKER_NODES=("${AVAILABLE_NODES[@]:$NUM_CC_NODES}")

if [ ${#WORKER_NODES[@]} -eq 0 ]; then
    echo "Not enough nodes for worker nodes, exiting."
    exit 1
fi

echo "CC nodes:"
for node in "${CC_NODES[@]}"; do
    echo "    $node"
done
echo "Worker nodes:"
for node in "${WORKER_NODES[@]}"; do
    echo "    $node"
done
echo "Done."
echo ""

# --- Assign ports to CC nodes ------------------------------------------------
echo "Assigning ports to CC nodes..."

mkdir -p "$DATA_DIR"
>"$DATA_DIR/activehostport.txt"

check_port_availability() {
    local node="$1"
    while :; do
        local port
        port=$(shuf -i 49631-65535 -n 1)
        if ! ssh "$USER@$node" "nc -z localhost $port" >/dev/null 2>&1; then
            echo "$port"
            return
        fi
    done
}

for node in "${CC_NODES[@]}"; do
    port=$(check_port_availability "$node")
    echo "$node:$port" >>"$DATA_DIR/activehostport.txt"
    echo "    $node:$port"
done

printf "%s\n" "${WORKER_NODES[@]}" >"$DATA_DIR/worker_nodes.txt"

echo "Done."
echo ""

# --- Start CC/Raft nodes -----------------------------------------------------
echo "Starting CC/Raft nodes..."

mapfile -t CC_ENDPOINTS <"$DATA_DIR/activehostport.txt"

for ENDPOINT in "${CC_ENDPOINTS[@]}"; do
    HOST=$(echo "$ENDPOINT" | cut -d':' -f1)
    PORT=$(echo "$ENDPOINT" | cut -d':' -f2)

    echo "    Starting CC on $HOST:$PORT"

    ssh -n "$USER@$HOST" "
        mkdir -p $DATA_DIR/logs &&
        cd $RAFT_DIR/src &&
        nohup python3 inf_3203_start_raft_node.py \
            $HOST \
            $PORT \
            $PROJECT_DIR \
        > $DATA_DIR/logs/cc_${HOST}.log 2>&1 < /dev/null &
    " &
done
wait

echo "Done."
echo ""

# --- Wait for CC nodes to start ----------------------------------------------
echo "Waiting for CC nodes to start..."
sleep 5
echo "Done."
echo ""

# --- Check CC nodes are responsive -------------------------------------------
echo "Checking CC nodes are responsive..."

for ENDPOINT in "${CC_ENDPOINTS[@]}"; do
    HOST=$(echo "$ENDPOINT" | cut -d':' -f1)
    PORT=$(echo "$ENDPOINT" | cut -d':' -f2)

    if ! curl --max-time 5 "http://$HOST:$PORT/raft-node-info" >/dev/null 2>&1; then
        echo "    Warning: CC at $HOST:$PORT is not responsive"
    else
        echo "    CC at $HOST:$PORT is responsive"
    fi
done

echo "Done."
echo ""
