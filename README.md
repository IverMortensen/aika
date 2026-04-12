# Aika - Distributed Image Labelling System
INF-3203 Assignment — UiT The Arctic University of Norway

## Overview

This project implements an Aika-inspired distributed scheduling system for workers
and tasks, used to label and organise 1,200,000 images across a cluster. The system
distributes image classification workloads across multiple nodes with fault tolerance,
load balancing, and consensus via the Raft protocol.

The system is composed of three main components:

- **Cluster Controller** — Manages the cluster using Raft consensus. The leader periodically polls Local Controllers to detect node failures and initiate recovery.
- **Local Controller** — Runs on each worker node and supervises the agent processes on that node, restarting them if they crash.
- **Agents** — Process images in a pipeline. The initial agent distributes image tasks, worker agents perform feature extraction and labelling, and the final agent collects results.

## Requirements

- Go (for compiling agents and local controller)
- Python 3.12 (for the cluster controller and image model)
- SSH access to the cluster nodes

### Cluster

This system is designed for the UiT IFI cluster and expects the cluster-specific layout:
- A list of available nodes at `/share/compute-nodes.txt`
- The unlabelled image dataset at `/share/inf3203/unlabeled_images`

If deployed on a different cluster, these paths would need to be updated to match the
target environment.

## Usage

### Start the system
Needs to be ran from the **base project directory**

```bash
./scripts/startup.sh
```

This will:
1. Clean up any leftover files from previous runs
2. Compile the Go binaries
3. Set up the Python venv for the image model (skipped if it already exists)
4. Find available cluster nodes
5. Assign nodes to Cluster Controllers and worker nodes
6. Start the Cluster Controller / Raft nodes
7. Wait for the CC nodes to become responsive

### Stop the system

```bash
./scripts/cleanup.sh
```

Kills all Aika processes (prefixed `inf_3203_`) started by the current user across the entire cluster.

### Kill specific nodes

```bash
./scripts/kill_nodes.sh <hostname1> <hostname2> ...
```

Kills all Aika processes on the specified nodes only.
