"""
Pi_t experiment deploying 6 clients, 6 relays, and 1 bulletin board across
13 nodes. Each node runs a specific service, either a client, relay,
or bulletin board.

Instructions:
All nodes are configured to install required dependencies and automatically
clone the pi_t-experiment repository to run their designated tasks.
"""
import geni.portal as portal
import geni.rspec.pg as rspec

# Create a Request object to start building the RSpec.
request = portal.context.makeRequestRSpec()

# Define a function to create nodes with a specified IP address
def add_node_with_ip(node_id, ip_address, subnet_mask="255.255.255.0"):
    # Create a RawPC node
    node = request.RawPC(node_id)
    # Add an interface to the node
    iface = node.addInterface("if1")
    # Specify the component id and the IPv4 address
    iface.component_id = "eth1"
    iface.addAddress(rspec.IPv4Address(ip_address, subnet_mask))
    return node, iface

# Create nodes for bulletin_board, relays, and clients with specific IPs
bulletin_board_ip = "192.168.1.1"
relay_ip = "192.168.1.2"
client_ip = "192.168.1.3"

# Add the bulletin_board, relay, and client nodes with specified IPs and unique interfaces
bulletin_board, bulletin_board_iface = add_node_with_ip("bulletin_board", bulletin_board_ip)
relay, relay_iface = add_node_with_ip("relay1", relay_ip)
client, client_iface = add_node_with_ip("client1", client_ip)

# Create a LAN to connect all the nodes
lan = request.LAN("lan")
lan.addInterface(bulletin_board_iface)
lan.addInterface(relay_iface)
lan.addInterface(client_iface)

# Function to create a startup script that uses the manually specified IP addresses
def create_startup_script(bulletin_board_ip, relay_ip, client_ip, role, index):
    return """
#!/bin/bash

# Create configuration content dynamically based on the node role
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
  host: '%s'
  port: 8080
clients:
  - id: 1
    host: '%s'
    port: 8101
    prometheus_port: 9101
relays:
  - id: 1
    host: '%s'
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
""" % (bulletin_board_ip, client_ip, relay_ip, role, index)

# Add the startup script to each node
for index, node in enumerate([bulletin_board, relay, client]):
    if node == bulletin_board:
        role = "bulletin_board"
    elif node == relay:
        role = "relay"
    else:
        role = "client"

    # Add the startup script to the node with the correct IPs
    node.addService(rspec.Execute(shell="bash", command=create_startup_script(bulletin_board_ip, relay_ip, client_ip, role, index)))

# Print the generated RSpec
portal.context.printRequestRSpec()
