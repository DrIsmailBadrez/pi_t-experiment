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

# Create nodes with specific IP addresses manually assigned
def add_node_with_ip(node_id, ip_address, subnet_mask="255.255.255.0", role=None, index=None):
    node = request.RawPC(node_id)
    # Set resources for the node: 4 cores, 8 threads, and 8GB of RAM
    node.cores = 4
    node.threads = 8
    node.ram = 8192
    iface = node.addInterface("if1")
    iface.component_id = "eth1"
    # Set IPv4 address
    iface.addAddress(rspec.IPv4Address(ip_address, subnet_mask))

    # Install required packages (excluding Go from apt, using Snap instead)
    node.addService(rspec.Execute(shell="bash", command="sudo apt update && sudo apt install -y git prometheus snapd"))

    # Install the latest version of Go via Snap
    node.addService(rspec.Execute(shell="bash", command="sudo snap install go --classic"))

    # Install yq via Snap
    node.addService(rspec.Execute(shell="bash", command="sudo snap install yq"))

    # Path and version of Prometheus
    node.addService(rspec.Execute(shell="bash", command="which prometheus"))
    node.addService(rspec.Execute(shell="bash", command="/usr/bin/prometheus --version"))

    # Clone the repo and set up safe directory handling
    node.addService(rspec.Execute(shell="bash", command="sudo mkdir -p /home/Ismail/pi_t-experiment && sudo chown -R $USER /home/Ismail/pi_t-experiment && "
                                                       "cd /home/Ismail && git clone https://github.com/DrIsmailBadrez/pi_t-experiment.git && "
                                                       "cd /home/Ismail/pi_t-experiment && git config --global --add safe.directory /home/Ismail/pi_t-experiment"))

    # Command to run the services based on role and index
    if role and index is not None:
        command = """
        cd /home/Ismail/pi_t-experiment && sudo chmod +x bin/runNode.sh && pwd && ls /home/Ismail/pi_t-experiment/config &&
        sudo ./bin/runNode.sh %s %d
        """ % (role, index)
        node.addService(rspec.Execute(shell="bash", command=command))

    return node, iface

# Assign IP addresses
bulletin_board_ip = "192.168.1.1"
relay_ips = ["192.168.1.2", "192.168.1.3", "192.168.1.4", "192.168.1.5", "192.168.1.6", "192.168.1.7"]
client_ips = ["192.168.1.8", "192.168.1.9", "192.168.1.10", "192.168.1.11", "192.168.1.12", "192.168.1.13"]

# Create and configure nodes for the bulletin board
bulletin_board, bulletin_board_iface = add_node_with_ip("bulletin_board", bulletin_board_ip, role="bulletin_board", index=0)

# Create relays
relays = []
relay_ifaces = []
for i, relay_ip in enumerate(relay_ips):
    relay, relay_iface = add_node_with_ip("relay%d" % (i + 1), relay_ip, role="relay", index=i + 1)
    relays.append(relay)
    relay_ifaces.append(relay_iface)

# Create clients
clients = []
client_ifaces = []
for i, client_ip in enumerate(client_ips):
    client, client_iface = add_node_with_ip("client%d" % (i + 1), client_ip, role="client", index=i + 1)
    clients.append(client)
    client_ifaces.append(client_iface)

# Create a LAN to connect all the nodes
lan = request.LAN("lan")
# Add bulletin board to LAN
lan.addInterface(bulletin_board_iface)
# Add relays to LAN
for iface in relay_ifaces:
    lan.addInterface(iface)
# Add clients to LAN
for iface in client_ifaces:
    lan.addInterface(iface)

# Print the generated RSpec
portal.context.printRequestRSpec()
