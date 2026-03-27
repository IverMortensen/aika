#!/bin/bash

# =============================================================================
# Kill script for the Aika distributed image labelling system
# =============================================================================

# --- Directories -------------------------------------------------------------
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATA_DIR="$PROJECT_DIR/data"

# --- Read node lists ---------------------------------------------------------
mapfile -t WORKER_NODES <"$DATA_DIR/worker_nodes.txt"
mapfile -t CC_ENDPOINTS <"$DATA_DIR/activehostport.txt"

# --- Kill processes on all nodes in parallel ---------------------------------
echo "Killing processes on all nodes..."

for node in "${WORKER_NODES[@]}"; do
    ssh "$node" "pgrep -u $USER -f inf_3203 | xargs -r kill -9" 2>/dev/null &
done

for ENDPOINT in "${CC_ENDPOINTS[@]}"; do
    HOST=$(echo "$ENDPOINT" | cut -d':' -f1)
    ssh "$HOST" "pgrep -u $USER -f inf_3203 | xargs -r kill -9" 2>/dev/null &
done

wait
echo "Done."
