#!/bin/bash

# =============================================================================
# Kill script for the Aika distributed image labelling system
# =============================================================================

# --- Kill processes on all nodes in parallel ---------------------------------
echo "Killing processes on all nodes..."

{
    while read -r node; do
        ssh -o ConnectTimeout=2 -o BatchMode=yes "$node" \
            "pgrep -u $USER -f inf_3203 | xargs -r kill -9" 2>/dev/null &
    done </share/compute-nodes.txt
    wait
} 2>/dev/null

wait
echo "Done."
