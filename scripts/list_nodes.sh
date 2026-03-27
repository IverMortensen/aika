nodes="./data/worker_nodes.txt"
mapfile -t WORKER_NODES <$nodes

for node in "${WORKER_NODES[@]}"; do
    echo "=== $node ==="
    ssh "$node" "ps aux | grep inf_3203"
done
