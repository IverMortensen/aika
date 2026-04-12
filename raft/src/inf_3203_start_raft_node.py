import sys
from raft_node import RaftNode


def main():
    """Startup for all the Raft nodes, each node gets their own host + port, including a list of other RAFT nodes"""

    if len(sys.argv) < 3:
        print("Usage: python startChordNode.py <host> <port>")
        sys.exit(1)

    host = sys.argv[1]
    port = int(sys.argv[2])
    project_dir = sys.argv[3]

    raft_nodes_file = project_dir + "/data/activehostport.txt"
    worker_nodes_file = project_dir + "/data/worker_nodes.txt"

    # Fetch all other raft node addresses.
    otherRaftNodes = []
    with open(raft_nodes_file, "r") as file:
        for node_addr in file:
            # Do not include this node in the otherRaftNodes list
            if node_addr.strip().replace("/", "") != f"{host}:{port}":
                otherRaftNodes.append(node_addr.strip().replace("/", ""))

    thisRaftNode = RaftNode(
        host=host,
        port=port,
        otherRaftNodes=otherRaftNodes,
        workerNodesFile=worker_nodes_file,
        projectPath=project_dir,
    )
    thisRaftNode.running_loop()
    thisRaftNode.init()


if __name__ == "__main__":
    main()
