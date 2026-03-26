#!/bin/bash

# =============================================================================
# Startup script for the Aika distributed image labelling system
# =============================================================================

# --- Configuration -----------------------------------------------------------
MAX_LOAD=0.4

# --- Directories -------------------------------------------------------------
BIN_DIR="./bin"
CMD_DIR="./cmd"

# --- Clean up ----------------------------------------------------------------
echo "Cleaning up..."

rm -f data/logs/*.log
rm -f data/wal/*.wal
rm -f data/result.json

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

    # Run all SSH checks in parallel, collect results, then sort
    for H in "${filtered[@]}"; do
        ssh -o ConnectTimeout=1 -o ConnectionAttempts=1 -x "$H" \
            "cat /proc/loadavg /proc/sys/kernel/hostname | tr '\n' ' ' | awk -v max_load=$MAX_LOAD '\$1+0 < max_load {printf \"%s %s\n\", \$1, \$6}'" 2>/dev/null &
    done | sort -n | awk '{print $2}' | sed 's/.ifi.uit.no//'
    wait
}

readarray -t AVAILABLE_NODES < <(get_available_nodes)

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
