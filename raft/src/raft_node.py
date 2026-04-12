from flask import Flask, jsonify, request
import requests
from enum import Enum
import time
import random
import threading
import os
import json
import subprocess
from concurrent.futures import ThreadPoolExecutor

AGENTS_PER_NODE = 5


class RaftStates(Enum):
    FOLLOWER = "follower"
    CANDIDATE = "candidate"
    LEADER = "leader"


class RaftNode:
    """RAFT protocol is implemented as the cluster controller."""

    def __init__(
        self,
        host: str,
        port: int,
        otherRaftNodes: list,
        workerNodesFile: str,
        projectPath: str,
    ):
        self.projectPath = projectPath

        # Load the worker nodes
        self.worker_nodes = []
        with open(workerNodesFile, "r") as f:
            self.worker_nodes = [line.strip() for line in f if line.strip()]

        # Persistent state on all servers
        self.currentTerm = 0
        self.votedFor = None
        self.log = []
        self.state = RaftStates.FOLLOWER  # Every Raft server initializes as follower
        self.leader = None
        self.lock = threading.Lock()

        self.votesRecived = 0

        self.cluster_size = len(otherRaftNodes) + 1
        self.otherRaftNodes: list = otherRaftNodes

        # Volatile state on all servers
        self.commitIndex = 0  # Index of highest log entry known to be committed (init to 0, increases monotonically)
        self.lastApplied = 0  # Index of highest log entry applied to state machine (init to 0, increases monotonically)

        # Volatile state on leaders (Reinitialized after election)
        self.nextIndex = {}  # Index of the next log entry to send to that server (init to leader last log index + 1)
        self.matchIndex = {}  # Index of highes log entry known to be replicated on server (inited to 0, increases monotonically)

        self.lastLogIndex = 0
        self.lastLogTerm = 0

        self.last_heartbeat = time.time()
        self.timeout_ms = random.uniform(
            2.300, 4.600
        )  # asumes that no two raft node has the same timeout

        # Cluster addresses
        self.port = port
        self.host = host
        self.address = f"{host}:{port}"
        self.alive = True
        self.app = Flask(__name__)
        self.setup_routes()

    def setup_routes(self):
        @self.app.route("/heartbeat", methods=["POST"])
        def handle_heartbeat():
            if self.alive is False:
                return jsonify({"error": "Node is dead"}), 503

            data = request.json or {}
            lc_id = data.get("lc_id", "unknown")

            # Only the leader should be receiving heartbeats from LCs,
            # redirect to leader if we are not
            if self.state != RaftStates.LEADER:
                if self.leader is not None:
                    return jsonify({"redirect": self.leader}), 307
                return jsonify({"error": "No leader elected yet"}), 503

            return jsonify({"status": "ok", "leader": self.address}), 200

        @self.app.route("/list-raft-nodes", methods=["GET"])
        def list_raft_nodes():
            return jsonify({"nodes": self.otherRaftNodes})

        @self.app.route("/raft-node-info", methods=["GET"])
        def raft_node_info():
            return jsonify(
                {
                    "state": self.state.value,
                    "currentTerm": self.currentTerm,
                    "votedFor": self.votedFor,
                    "commitIndex": self.commitIndex,
                    "lastApplied": self.lastApplied,
                    "nextIndex": self.nextIndex,
                    "matchIndex": self.matchIndex,
                    "otherRaftNodes": self.otherRaftNodes,
                    "server_id": f"{self.host}:{self.port}",
                    "votesRecived": self.votesRecived,
                    "leader": self.leader,
                    "alive": self.alive,
                }
            )

        @self.app.route("/raft-state-info", methods=["GET"])
        def raft_state_info():
            return jsonify(
                {"ID": f"{self.host}:{self.port}", "state": self.state.value}
            )

        @self.app.route("/raft-myVote", methods=["GET"])
        def raft_my_vote_info():
            return jsonify(
                {
                    "server_id": f"{self.address}",
                    "votesRecieved": self.votesRecived,
                    "votedFor": self.votedFor,
                    "term": self.currentTerm,
                    "ID": f"{self.host}:{self.port}",
                    "state": self.state.value,
                    "alive": self.alive,
                    "log": self.log,
                    "commitIndex": self.commitIndex,
                }
            )

        ### TEST DEBUG API KALL

        @self.app.route("/raft-change-status", methods=["POST"])
        def change_raft_node_status():
            '''
            Changes the state of a RAFT node

            state = 1: Follower
            state = 2: Candidate
            state = 3: Leader

            How to use: "curl -X POST http://c0-0:0000/raft-change-status?state=1"'''

            state = request.args.get("state", type=int)

            if state == 1:
                self.state = RaftStates.FOLLOWER
                self.votesRecived = 0

            elif state == 2:
                self.state = RaftStates.CANDIDATE

            elif state == 3:
                self.state = RaftStates.LEADER

            else:
                return {"400": "Bad Request"}

            return {"success": True, "new_state": self.state.value}, 200

        @self.app.route("/raft-kill-node", methods=["POST"])
        def kill_node():
            """Alives or unalives a raft node.

            alive_status = 0: Dead
            alive_status = 1: Alive

            How to use "curl -X POST http://c0-0:0000/raft-kill-node?alive=0"
            """

            alive = request.args.get("alive", type=int)

            if alive == 0:
                self.alive = False
                self.votesRecived = 0
                self.votedFor = None

            elif alive == 1:
                self.alive = True
                self.votesRecived = 0
                self.votedFor = None
                self.state = RaftStates.FOLLOWER

            else:
                return {"400": "Bad Request"}

            return {"success": True, "alive_status": self.alive}

        # Lederen sender data til dette API kallet
        @self.app.route("/appendEntries", methods=["POST"])
        def handle_append_entries():

            if self.alive is False:
                return

            data = request.json
            leader_term = data.get("term")
            prev_log_index = data.get("prevLogIndex", -1)
            prev_log_term = data.get("prevLogTerm", -1)
            leader_id = data.get("leaderId")
            entries = data.get("entries", [])
            leader_commit = data.get("leaderCommit", 0)

            with self.lock:
                # Check the leader's term is outdated
                if leader_term < self.currentTerm:
                    return jsonify({"term": self.currentTerm, "success": False}), 200

                self.currentTerm = leader_term
                self.leader = leader_id
                self.state = RaftStates.FOLLOWER

                # Valid leader, reset the election timer
                self.reset_election_timer()

                # Consistency check, do we have prev_log_index with correct term?
                if prev_log_index >= 0:
                    if (
                        prev_log_index >= len(self.log)
                        or self.log[prev_log_index]["term"] != prev_log_term
                    ):
                        return jsonify(
                            {"term": self.currentTerm, "success": False}
                        ), 200

                # Append new entries (overwrite in case of conflict)
                for i, entry in enumerate(entries):
                    index = prev_log_index + 1 + i
                    if index < len(self.log):
                        if self.log[index]["term"] != entry["term"]:
                            self.log = self.log[:index]  # Remove log conflict
                            self.log.append(entry)

                    else:
                        self.log.append(entry)

                # Update commitIndex
                if leader_commit > self.commitIndex:
                    self.commitIndex = min(leader_commit, len(self.log) - 1)

                return jsonify({"term": self.currentTerm, "success": True}), 200

        @self.app.route("/requestVote", methods=["POST"])
        def send_vote():

            if self.alive is False:
                return

            data = request.json
            term = data.get("term")
            candidateID = data.get("candidateID")
            # lastLogTerm = data.get("lastLogTerm")

            # 1. If the sender has an older term, reject the vote
            with self.lock:
                if term < self.currentTerm:
                    return jsonify({"term": self.currentTerm, "grantVote": False})

                if term > self.currentTerm:
                    self.currentTerm = term
                    self.state = RaftStates.FOLLOWER
                    self.votedFor = None
                    self.reset_election_timer()

                if term == self.currentTerm and (
                    self.votedFor is None or self.votedFor == candidateID
                ):  # and lastLogTerm >= self.lastLogTerm:
                    self.votedFor = candidateID
                    self.reset_election_timer()
                    return jsonify({"term": self.currentTerm, "grantVote": True})

                return jsonify({"term": self.currentTerm, "grantVote": False})

        @self.app.route("/execute", methods=["POST"])
        def client_request():
            """Endpoint for client to"""

            if self.alive is False:
                return

            data = request.json

            with self.lock:
                if self.state == RaftStates.LEADER:
                    return self.process_client_command(data)

                # Redirect client request to leader
                elif self.leader is not None:
                    try:
                        response = requests.post(
                            f"http://{self.leader}/execute", json=data, timeout=1.0
                        )
                        return (
                            response.content,
                            response.status_code,
                            response.headers.items(),
                        )
                    except Exception:
                        return jsonify({"error": "Leader unreachable"}), 503

                else:
                    return jsonify({"error": "Leader unknown, try again later"}), 503

    def ensure_lcs_running(self):
        """Continuously ensures all LCs are running while this node is leader."""

        LC_BINARY = f"{self.projectPath}/bin/inf_3203_local_controller"
        CC_ADDRESSES = [self.address] + self.otherRaftNodes
        INITIAL_PORT = "36234"
        FINAL_PORT = "35235"
        FIRST_NODE = self.worker_nodes[0]
        INITIAL_LISTEN = f"0.0.0.0:{INITIAL_PORT}"
        FINAL_LISTEN = f"0.0.0.0:{FINAL_PORT}"
        INITIAL_ADDRESS = f"{FIRST_NODE}:{INITIAL_PORT}"
        FINAL_ADDRESS = f"{FIRST_NODE}:{FINAL_PORT}"
        INITIAL_AGENT_BINARY = f"{self.projectPath}/bin/inf_3203_initial_agent"
        WORKER_AGENT_BINARY = f"{self.projectPath}/bin/inf_3203_worker_agent"
        FINAL_AGENT_BINARY = f"{self.projectPath}/bin/inf_3203_final_agent"

        os.makedirs(f"{self.projectPath}/data/lc_configs", exist_ok=True)

        while self.state == RaftStates.LEADER and self.alive:
            launch_tasks = []
            worker_counter = 0

            for i, node in enumerate(self.worker_nodes):
                if self.is_lc_running(node):
                    continue  # Healthy, skip

                # Build config for this node
                agents = []
                if i == 0:
                    agents.append({
                        "binary": INITIAL_AGENT_BINARY,
                        "flags": [
                            "-image-dir", "/share/inf3203/unlabeled_images/",
                            "-wal-path", f"{self.projectPath}/data/wal/initial.wal",
                            "-server-address", INITIAL_LISTEN,
                            "-agent-id", "initial-agent",
                            "-log-file", f"{self.projectPath}/data/logs/initial-agent.log",
                        ],
                    })
                    agents.append({
                        "binary": FINAL_AGENT_BINARY,
                        "flags": [
                            "-output-path", f"{self.projectPath}/data/result.json",
                            "-wal-path", f"{self.projectPath}/data/wal/final.wal",
                            "-server-address", FINAL_LISTEN,
                            "-agent-id", "final-agent",
                            "-log-file", f"{self.projectPath}/data/logs/final-agent.log",
                        ],
                    })

                while len(agents) < AGENTS_PER_NODE:
                    agents.append({
                        "binary": WORKER_AGENT_BINARY,
                        "flags": [
                            "-ia-address", INITIAL_ADDRESS,
                            "-fa-address", FINAL_ADDRESS,
                            "-agent-id", f"worker-{worker_counter}",
                            "-log-file", f"{self.projectPath}/data/logs/worker-{worker_counter}.log",
                            "-model-path", f"{self.projectPath}/model",
                        ],
                    })
                    worker_counter += 1

                config_path = f"{self.projectPath}/data/lc_configs/lc_{node}.json"
                with open(config_path, "w") as f:
                    json.dump({"cluster_controllers": CC_ADDRESSES, "agents": agents}, f, indent=4)

                launch_tasks.append((node, config_path))

            if launch_tasks:
                print(f"Starting {len(launch_tasks)} LC(s)...", flush=True)
                with ThreadPoolExecutor(max_workers=len(launch_tasks)) as executor:
                    futures = {
                        executor.submit(self._start_lc, node, config_path, LC_BINARY): node
                        for node, config_path in launch_tasks
                    }
                    for future, node in futures.items():
                        try:
                            future.result()
                            print(f"Started LC on {node}", flush=True)
                        except Exception as e:
                            print(f"Failed to start LC on {node}: {e}", flush=True)

            time.sleep(10)  # Wait before next health check cycle

    def _start_lc(self, node, config_path, lc_binary):
        cmd = (
            f"ssh -n -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=no {node} "
            f"'mkdir -p {self.projectPath}/data/logs && "
            f"nohup {lc_binary} "
            f"--config {config_path} "
            f"--log-file {self.projectPath}/data/logs/lc_{node}.log "
            f"> {self.projectPath}/data/logs/lc_{node}_stdout.log 2>&1 < /dev/null &'"
        )
        try:
            subprocess.run(cmd, shell=True, timeout=10)
        except subprocess.TimeoutExpired:
            print(f"SSH timeout starting LC on {node}", flush=True)

    def is_lc_running(self, node):
        """Check if a local controller is already running on a node."""
        try:
            response = requests.get(f"http://{node}:7000/status", timeout=2)
            return response.status_code == 200
        except Exception:
            return False

    def process_client_command(self, data):

        entry = {"term": self.currentTerm, "command": data}
        self.log.append(entry)
        self.lastLogIndex = len(self.log) - 1
        self.lastLogTerm = entry["term"]

        return jsonify({"status": "success", "index": self.lastLogIndex}), 200

    def reset_election_timer(self):

        self.last_heartbeat = time.time()
        self.timeout_ms = random.uniform(2.300, 4.600)

    def follower_loop(self):
        """If the timer runs out we start an election"""

        # Election timer loop
        if time.time() - self.last_heartbeat > self.timeout_ms:
            """If the timer runs out, change the state to candidate
                increase the term, vote on myself
            """
            self.become_candidate()

    def become_candidate(self):

        with self.lock:
            now = time.time()
            if now - self.last_heartbeat < self.timeout_ms:
                return

            self.state = RaftStates.CANDIDATE
            self.currentTerm += 1
            self.votedFor = self.address
            self.votesRecived = 1
            self.reset_election_timer()

        with ThreadPoolExecutor(max_workers=len(self.otherRaftNodes)) as executor:
            # Start all requests in the background
            futures = [
                executor.submit(self.send_request_vote, node_addr)
                for node_addr in self.otherRaftNodes
            ]

    def send_request_vote(self, node_addr):
        try:
            # Lagre term lokalt så vi ikke bruker en term som endrer seg midt i kallet
            with self.lock:
                current_term = self.currentTerm

            payload = {
                "term": current_term,
                "candidateID": self.address,
                "lastLogIndex": self.lastLogIndex,
                "lastLogTerm": self.lastLogTerm,
            }

            response = requests.post(
                f"http://{node_addr}/requestVote", json=payload, timeout=0.1
            )

            if response.status_code == 200:
                data = response.json()
                voter_term = data.get("term")
                vote_granted = data.get("grantVote")

                with self.lock:
                    # VIKTIG: Hvis vi ser en høyere term, må vi gi opp med en gang
                    if voter_term > self.currentTerm:
                        self.currentTerm = voter_term
                        self.state = RaftStates.FOLLOWER
                        self.votedFor = None
                        return

                    if (
                        self.state == RaftStates.CANDIDATE
                        and vote_granted
                        and current_term == self.currentTerm
                    ):
                        self.votesRecived += 1
                        if self.votesRecived >= (self.cluster_size // 2) + 1:
                            self.become_leader()
        except Exception:
            pass  # Node utilgjengelig

    def become_follower(self, new_term):
        with self.lock:
            self.state = RaftStates.FOLLOWER
            self.votesRecived = 0
            self.votedFor = None
            self.currentTerm = new_term

    def become_leader(self):
        self.state = RaftStates.LEADER
        self.nextIndex = {node: len(self.log) for node in self.otherRaftNodes}
        self.matchIndex = {node: -1 for node in self.otherRaftNodes}

        threading.Thread(target=self.heartbeat_loop, daemon=True).start()
        threading.Thread(target=self.ensure_lcs_running, daemon=True).start()

    def heartbeat_loop(self):

        run = True

        while run:
            with self.lock:
                if self.state != RaftStates.LEADER:
                    return

                current_term = self.currentTerm
                leader_id = self.address
                commit_index = self.commitIndex

            with ThreadPoolExecutor(max_workers=len(self.otherRaftNodes)) as executor:
                for node_addr in self.otherRaftNodes:
                    executor.submit(
                        self.send_heartbeat, node_addr, current_term, commit_index
                    )

                if self.alive == False:
                    run = False
                    break

                time.sleep(0.05)

    def send_heartbeat(self, node_addr, term, commit_index):

        with self.lock:
            prev_idx = self.nextIndex[node_addr] - 1
            prev_term = self.log[prev_idx]["term"] if prev_idx >= 0 else -1
            # fetch all entries from nextIndex
            entries_to_send = self.log[self.nextIndex[node_addr] :]

        payload = {
            "term": term,
            "leaderId": self.address,
            "prevLogIndex": prev_idx,
            "prevLogTerm": prev_term,
            "entries": entries_to_send,
            "leaderCommit": commit_index,
        }

        try:
            response = requests.post(
                f"http://{node_addr}/appendEntries", json=payload, timeout=0.2
            )

            if response.status_code == 200:
                data = response.json()
                follower_term = data.get("term")
                success = data.get("success")

                with self.lock:
                    if follower_term > self.currentTerm:
                        self.currentTerm = follower_term
                        self.state = RaftStates.FOLLOWER
                        self.votedFor = None
                        self.reset_election_timer()

                    if success:
                        self.matchIndex[node_addr] = prev_idx + len(entries_to_send)
                        self.nextIndex[node_addr] = self.matchIndex[node_addr] + 1
                        self.update_commit_index()
                    else:
                        # Consistency error, go one step back in the log and try next heartbeat
                        self.nextIndex[node_addr] = max(
                            0, self.nextIndex[node_addr] - 1
                        )

        # If a raft node does not answer we need to try and start the raft node again via RPC
        except Exception as e:
            pass

    def update_commit_index(self):

        for n in range(len(self.log) - 1, self.commitIndex, -1):
            if self.log[n]["term"] == self.currentTerm:
                count = 1
                for node in self.otherRaftNodes:
                    if self.matchIndex[node] >= n:
                        count += 1

                if count >= (self.cluster_size // 2) + 1:
                    self.commitIndex = n
                    break

    def candidate_loop(self):
        if time.time() - self.last_heartbeat > self.timeout_ms:
            self.become_candidate()

    def running_loop(self):
        """The raft nodes does different things depending on which state they are in
        This method is trying to keep the different states cleanly separated.
        """

        def run():
            while True:
                # If the node is simulated crashed continue to next iteration
                if self.alive == False:
                    time.sleep(0.1)
                    continue

                if self.state == RaftStates.FOLLOWER:
                    self.follower_loop()

                if self.state == RaftStates.CANDIDATE:
                    self.candidate_loop()

                time.sleep(0.05)

        threading.Thread(target=run, daemon=True).start()

    def init(self):
        """The node is always running, if alive is set to False,
        the node is for all intents and purposes not shutdown and can be considered killed / crashed"""

        self.app.run(host=self.host, port=self.port, debug=False, use_reloader=False)
