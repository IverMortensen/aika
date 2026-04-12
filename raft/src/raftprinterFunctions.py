'''Print all raft nodes, used for debugging purposes.'''


'''Må åpne filen hente ut alle aktive raft noder og printe deres status ved bruk av curl eller api kall eller whatever.'''

import requests



def AllRaftNodeStatus():
    '''USED FOR DEBUGGING, find out if the nodes are follower, leader or candidate. '''

    with open("../data/activehostport.txt", "r") as file:
        for node_addr in file:

            response = requests.get(f"http://{node_addr.strip()}/raft-state-info", timeout=2)
            print(response.json())




if __name__ == "__main__":
    AllRaftNodeStatus()