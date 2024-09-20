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

# Function to create a node with specific resources (cores, threads, RAM) and fetch its IP
def add_node_with_ip(node_id, node_type, role, index):
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
    node.addService(RSpec.Execute(shell="bash", command="sudo git clone https://github.com/DrIsmailBadrez/pi_t-experiment.git && "
                                                        "cd pi_t-experiment && sudo git config --global --add safe.directory $HOME/pi_t-experiment"))

    # Command to run the services based on role and index
    command = "cd pi_t-experiment && sudo chmod +x bin/runNode.sh && sudo ./bin/runNode.sh %s %d" % (role, index)
    node.addService(RSpec.Execute(shell="bash", command=command))

    # Fetch the node's IP address
    iface = node.addInterface()
    ip_address = iface.getIPAddress()

    # Return the node and its IP address
    return node, ip_address

# Bulletin Board Node
bb_node, bb_ip = add_node_with_ip("bulletin_board", "pc3000", "bulletin_board", 0)

# Relay Nodes (6 Relays)
relays = []
relay_ips = []
for i in range(1, 7):
    relay_node, relay_ip = add_node_with_ip("relay%d" % i, "pc3000", "relay", i)
    relays.append(relay_node)
    relay_ips.append(relay_ip)

# Client Nodes (6 Clients)
clients = []
client_ips = []
for i in range(1, 7):
    client_node, client_ip = add_node_with_ip("client%d" % i, "pc3000", "client", i)
    clients.append(client_node)
    client_ips.append(client_ip)

# Print the generated RSpec
portal.context.printRequestRSpec(request)

# Set environment variables for IP addresses
request.addService(RSpec.Execute(shell="bash", command="export BB_IP=%s" % bb_ip))
request.addService(RSpec.Execute(shell="bash", command="export RELAY_IPS=%s" % ','.join(relay_ips)))
request.addService(RSpec.Execute(shell="bash", command="export CLIENT_IPS=%s" % ','.join(client_ips)))

# After deployment, generate the config.yml file
request.addService(RSpec.Execute(shell="bash", command="sudo python3 /local/repository/config_generator.py"))
