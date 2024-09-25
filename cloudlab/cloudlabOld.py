"""
Pi_t experiment deploying 6 clients, 6 relays, and 1 bulletin board across
13 nodes. Each node runs a specific service, either a client, relay,
or bulletin board.

Instructions:
All nodes are configured to install required dependencies and automatically
clone the pi_t-experiment repository to run their designated tasks.
"""
import geni.portal as portal
import geni.rspec.pg as RSpec

# Create a request object to start building the RSpec
request = RSpec.Request()

# Function to create a node with specific resources (cores, threads, RAM)
def add_node(node_id, node_type, role, index):
    node = request.RawPC(node_id)
    node.hardware_type = node_type
    node.disk_image = "urn:publicid:IDN+emulab.net+image+emulab-ops//UBUNTU20-64-STD"

    # Set resources for the node: 4 cores, 8 threads, and 8GB of RAM
    node.cores = 4
    node.threads = 8
    node.ram = 8192  # 8GB of RAM

    # Install required packages and clone the GitHub repo
    node.addService(RSpec.Execute(shell="bash", command="sudo apt update && sudo apt install -y git golang prometheus snapd"))

    # Install yq via Snap
    node.addService(RSpec.Execute(shell="bash", command="sudo snap install yq"))

    # Clone the repo and set up safe directory handling
    node.addService(RSpec.Execute(shell="bash", command="cd $HOME && git clone https://github.com/DrIsmailBadrez/pi_t-experiment.git && "
                                                       "cd $HOME/pi_t-experiment && git config --global --add safe.directory $HOME/pi_t-experiment"))

    # Command to run the services based on role and index
    command = "cd $HOME/pi_t-experiment && ls && sudo chmod +x bin/runNode.sh && sudo ./bin/runNode.sh %s %d" % (role, index)
    node.addService(RSpec.Execute(shell="bash", command=command))

    return node

# Bulletin Board Node
add_node("bulletin_board", "pc3000", "bulletin_board", 0)

# Relay Nodes (1 Relay)
for i in range(1, 2):
    add_node("relay%d" % i, "pc3000", "relay", i)

# Client Nodes (1 Client)
for i in range(1, 2):
    add_node("client%d" % i, "pc3000", "client", i)

# Print the generated RSpec
portal.context.printRequestRSpec(request)
