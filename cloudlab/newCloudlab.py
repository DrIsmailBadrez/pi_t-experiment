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

# Function to create a node with specific resources (cores, threads, RAM) without fetching IP at creation
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
    node.addService(RSpec.Execute(shell="bash", command="sudo git clone https://github.com/DrIsmailBadrez/pi_t-experiment.git && "
                                                        "cd pi_t-experiment && sudo git config --global --add safe.directory $HOME/pi_t-experiment"))

    # Return the node (without IP fetching at this stage)
    return node

# Bulletin Board Node
bb_node = add_node("bulletin_board", "pc3000", "bulletin_board", 0)

# Relay Nodes (6 Relays)
relays = []
for i in range(1, 7):
    relay_node = add_node("relay%d" % i, "pc3000", "relay", i)
    relays.append(relay_node)

# Client Nodes (6 Clients)
clients = []
for i in range(1, 7):
    client_node = add_node("client%d" % i, "pc3000", "client", i)
    clients.append(client_node)

# Print the generated RSpec
portal.context.printRequestRSpec(request)

# Use a script to generate the config.yml file with the correct IP addresses post-deployment
generate_config_script = """
#!/bin/bash
BB_IP=$(hostname -I | awk '{print $1}')
RELAY_IPS=()
CLIENT_IPS=()

# Fetch IPs for relay and client nodes
for i in {1..6}; do
  RELAY_IP=$(ssh relay$i hostname -I | awk '{print $1}')
  CLIENT_IP=$(ssh client$i hostname -I | awk '{print $1}')
  RELAY_IPS+=($RELAY_IP)
  CLIENT_IPS+=($CLIENT_IP)
done

# Generate config.yml
cat <<EOT > /local/repository/config.yml
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
  host: '$BB_IP'
  port: 8080
clients:
EOT

for i in {1..6}; do
  echo "  - id: \$i" >> /local/repository/config.yml
  echo "    host: \${CLIENT_IPS[\$((i-1))]}" >> /local/repository/config.yml
  echo "    port: 810\$i" >> /local/repository/config.yml
  echo "    prometheus_port: 910\$i" >> /local/repository/config.yml
done

echo "relays:" >> /local/repository/config.yml
for i in {1..6}; do
  echo "  - id: \$i" >> /local/repository/config.yml
  echo "    host: \${RELAY_IPS[\$((i-1))]}" >> /local/repository/config.yml
  echo "    port: 808\$i" >> /local/repository/config.yml
  echo "    prometheus_port: 920\$i" >> /local/repository/config.yml
done
"""

# Add the script to generate config and run the services after deployment
bb_node.addService(RSpec.Execute(shell="bash", command=generate_config_script))

# Run the services only after the config.yml has been generated
def run_service_after_config(role, index):
    return """
    if [ -f /local/repository/config.yml ]; then
        cd pi_t-experiment
        sudo chmod +x bin/runNode.sh
        sudo ./bin/runNode.sh %s %d
    else
        echo "Config file not found, service will not start!"
    fi
    """ % (role, index)

# Bulletin Board service
bb_service_command = run_service_after_config("bulletin_board", 0)
bb_node.addService(RSpec.Execute(shell="bash", command=bb_service_command))

# Relay services
for i in range(1, 7):
    relay_service_command = run_service_after_config("relay", i)
    relays[i-1].addService(RSpec.Execute(shell="bash", command=relay_service_command))

# Client services
for i in range(1, 7):
    client_service_command = run_service_after_config("client", i)
    clients[i-1].addService(RSpec.Execute(shell="bash", command=client_service_command))
