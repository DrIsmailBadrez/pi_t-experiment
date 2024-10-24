"""
Pi_t experiment deploying 100 clients, 50 relays, and 1 bulletin board across
151 nodes. Each node runs a specific service, either a client, relay,
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
    node.addService(rspec.Execute(shell="bash", command="sudo apt update && sudo apt install -y git snapd wget tar"))

    # Install the latest version of Go via Snap
    node.addService(rspec.Execute(shell="bash", command="sudo snap install go --classic"))

    # Install yq via Snap
    node.addService(rspec.Execute(shell="bash", command="sudo snap install yq"))

    # Download and install Prometheus version 2.54.1
    prometheus_version = "2.54.1"
    prometheus_install_command = """
        cd /tmp && \
        wget https://github.com/prometheus/prometheus/releases/download/v%s/prometheus-%s.linux-amd64.tar.gz && \
        tar -xvf prometheus-%s.linux-amd64.tar.gz && \
        sudo mv prometheus-%s.linux-amd64/prometheus /usr/bin/ && \
        sudo mv prometheus-%s.linux-amd64/promtool /usr/bin/ && \
        rm -rf prometheus-%s.linux-amd64*
    """ % (prometheus_version, prometheus_version, prometheus_version, prometheus_version, prometheus_version, prometheus_version)

    node.addService(rspec.Execute(shell="bash", command=prometheus_install_command))

    # Verify Prometheus installation and version

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

# Create relays
relays = []
relay_ifaces = []
for i, relay_ip in enumerate(7, 25):
    relay, relay_iface = add_node_with_ip("relay%d" % (i + 1), relay_ip, role="relay", index=i + 1)
    relays.append(relay)
    relay_ifaces.append(relay_iface)

# Create clients
clients = []
client_ifaces = []
for i, client_ip in enumerate(7, 37):
    client, client_iface = add_node_with_ip("client%d" % (i + 1), client_ip, role="client", index=i + 1)
    clients.append(client)
    client_ifaces.append(client_iface)

# Print the generated RSpec
portal.context.printRequestRSpec()
