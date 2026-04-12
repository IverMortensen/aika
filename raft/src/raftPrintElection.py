"""Print all raft nodes, used for debugging purposes."""

"""Må åpne filen hente ut alle aktive raft noder og printe deres status ved bruk av curl eller api kall eller whatever."""

import requests
import time
import os
from rich.console import Console
from rich.console import Group
from rich.panel import Panel
from rich.table import Table
from rich.live import Live

path = "../../data/activehostport.txt"


def AllRaftNodeStatus():
    """USED FOR DEBUGGING, find out if the nodes are follower, leader or candidate."""

    last_modified = os.path.getmtime(path)

    with open(path, "r") as file:
        # Keep the script running during the RAFT protocol
        while True:
            current_mtime = os.path.getmtime(path)

            if current_mtime == os.path.getmtime(path):
                print("File changed!")

            print("\033[H\033[J", end="")

            print("=== RAFT CLUSTER STATUS ===")

            file.seek(0)
            for node_addr in file:
                r = requests.get(f"http://{node_addr.strip()}/raft-myVote")
                print(node_addr.strip(), "->", r.json()["state"])

            time.sleep(0.5)


def build_log(node_data_list):
    matrix = Table(title="Raft replicated logs", border_style="white")

    max_log_size = 0
    for data in node_data_list:
        if data and "log" in data:
            max_log_size = max(max_log_size, len(data["log"]))

    start_row = max(0, max_log_size - 10)

    # Add a column for each node
    matrix.add_column("index", style="dim", width=5)
    for data in node_data_list:
        name = data.get("server_id", "Unknown")
        matrix.add_column(name, justify="center")

    # Fill inn rows
    for i in range(start_row, max_log_size):
        row_cells = [f"#{i}"]
        for data in node_data_list:
            log = data.get("log", [])
            if i < len(log):
                entry = log[i]
                term = entry.get("term", "?")
                cmd = entry.get("command", "?")
                style = "green" if i <= data.get("commitIndex", -1) else "white"
                row_cells.append(f"[{style}]T{term}: {cmd}[/]")
            else:
                row_cells.append("[dim]-[/]")
        matrix.add_row(*row_cells)

    return matrix


console = Console()

tick = 0  # global timer


def build_table(nodes, tick):
    table = Table(title=f"RAFT Cluster Status — Tick {tick}")

    table.add_column("Server", style="cyan")
    table.add_column("State")
    table.add_column("Term")
    table.add_column("Voted For")
    table.add_column("Votes")

    node_status = ""

    for node in nodes:
        try:
            r = requests.get(f"http://{node}/raft-myVote", timeout=1)
            data = r.json()

            if data["alive"] == False:
                node_status = "DEAD"
            else:
                node_status = data["state"]

            table.add_row(
                data["server_id"],
                node_status,
                str(data["term"]),
                str(data["VotedFor"]),
                str(data["votesRecived"]),
            )

        except Exception:
            table.add_row(node, "DEAD", "-", "-", "-")

    return table


def get_all_data(nodes):
    results = []
    for node in nodes:
        try:
            # Vi setter en lav timeout så dashboardet ikke lagger
            r = requests.get(f"http://{node}/raft-myVote", timeout=0.3)
            data = r.json()
            # Vi legger til et flagg som bekrefter at vi fikk kontakt
            results.append(data)
        except:
            # Her havner vi ved "Connection Refused" eller "Timeout"
            results.append(
                {
                    "server_id": node,
                    "alive": False,
                    "log": [],
                    "state": "DEAD (NO SIGNAL)",
                    "term": "-",
                    "commitIndex": "-",
                }
            )
    return results


def update_display(nodes, tick):
    all_node_data = get_all_data(nodes)

    # Tabell 1: Oversikt
    overview_table = Table(title=f"RAFT CLUSTER STATUS — Tick {tick}")
    overview_table.add_column("Server")
    overview_table.add_column("State")
    overview_table.add_column("Term")
    overview_table.add_column("CommitIdx")
    overview_table.add_column("Voted For")
    overview_table.add_column("Votes")

    node_status = ""

    for data in all_node_data:
        if data["alive"] == False:
            node_status = "DEAD LOGICAL"

        if data["state"] == "DEAD (NO SIGNAL)":
            node_status = "DEAD (NO SIGNAL)"

        else:
            node_status = data["state"]

        overview_table.add_row(
            data["server_id"],
            node_status,
            str(data.get("term", "-")),
            str(data.get("commitIndex", "-")),
            str(data.get("votedFor", "-")),
            str(data.get("votesRecieved", "-")),
        )

    log_matrix = build_log(all_node_data)

    return Group(overview_table, log_matrix)


def show_raft_grid():

    with open(path) as file:
        nodes = [line.strip() for line in file]

    tick = 0

    with Live(
        update_display(nodes, tick), console=console, refresh_per_second=2
    ) as live:
        while True:
            tick += 1
            live.update(update_display(nodes, tick))
            time.sleep(3)  # halve sekund mellom oppdateringer


if __name__ == "__main__":
    show_raft_grid()

