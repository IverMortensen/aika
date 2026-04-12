#!/bin/bash

if [ "$#" -eq 0 ]; then
    echo "Usage: $0 <node1> <node2> ..."
    exit 1
fi

for node in "$@"; do
    echo "Killing inf_3203 processes on $node..."
    ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$node" \
        "pkill -u imo059 -f inf_3203 && echo 'Done' || echo 'No matching processes found'"
done
