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
    node.cores = 4
    node.threads = 8
    node.ram = 8192

    # Install required packages and clone the GitHub repo
    node.addService(RSpec.Execute(shell="bash", command="sudo apt update && sudo apt install -y git golang prometheus snapd"))
    node.addService(RSpec.Execute(shell="bash", command="sudo snap install yq"))

    # Create the necessary directory and clone the repository
    node.addService(RSpec.Execute(shell="bash", command="mkdir -p /home/Ismail/pi_t-experiment && cd /home/Ismail/ && git clone https://github.com/DrIsmailBadrez/pi_t-experiment.git && cd /home/Ismail/pi_t-experiment && git config --global --add safe.directory /home/Ismail/pi_t-experiment"))

    return node

# Create nodes for bulletin_board, relays, and clients
bulletin_board = add_node("bulletin_board", "pc3000", "bulletin_board", 0)

# Relay nodes (assuming you want 6 relays)
relays = [add_node("relay%d" % i, "pc3000", "relay", i) for i in range(1, 7)]

# Client nodes (assuming you want 6 clients)
clients = [add_node("client%d" % i, "pc3000", "client", i) for i in range(1, 7)]

# Function to create a startup script to collect IP addresses and generate config files
def create_startup_script(role, index):
    return """
#!/bin/bash

# Wait for the node to fully boot and get an IP address
sleep 30

# Get the IP address of this node
ip_addr=$(hostname -I | awk '{print $1}')

# Create configuration content dynamically
config_content=\"
l1: 3
l2: 2
x: 25
tau: 0.8
d: 2
delta: 1e-5
chi: 1.0
vis: true
scrapeInterval: 1000
dropAllOnionsFromClient: 1
prometheusPath: '/opt/homebrew/bin/prometheus'

metrics:
  host: 'localhost'
  port: 8200
bulletin_board:
  host: '$ip_addr'
  port: 8080
clients:
  - id: 1
    host: '$ip_addr'
    port: 8101
    prometheus_port: 9101
relays:
  - id: 1
    host: '$ip_addr'
    port: 8081
    prometheus_port: 9201
\"

# Write the config file to the appropriate directory
mkdir -p /home/Ismail/pi_t-experiment/config
echo "$config_content" > /home/Ismail/pi_t-experiment/config/config.yml

# Run the node's role-specific service
cd /home/Ismail/pi_t-experiment
sudo chmod +x bin/runNode.sh
sudo ./bin/runNode.sh %s %d
""" % (role, index)

# Add the startup script to each node
for index, node in enumerate([bulletin_board] + relays + clients):
    # Determine the role based on the node type
    if node == bulletin_board:
        role = "bulletin_board"
    elif node in relays:
        role = "relay"
    else:
        role = "client"

    # Add the startup script to the node
    node.addService(RSpec.Execute(shell="bash", command=create_startup_script(role, index)))

# Print the generated RSpec
portal.context.printRequestRSpec(request)
