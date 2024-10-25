"""
Pi_t experiment deploying 100 clients, 50 relays, and 1 bulletin board
across 151 nodes. Each client node runs 10 clients with distinct Prometheus
ports, and each relay node runs 5 relays. Each node installs dependencies
and clones the pi_t-experiment repository to run assigned tasks.
"""
import geni.portal as portal
import geni.rspec.pg as rspec

# Create a Request object to start building the RSpec.
request = portal.context.makeRequestRSpec()

# Function to create nodes with adequate resources based on their role
def add_node(node_id, role=None):
    node = request.RawPC(node_id)

    # Allocate resources based on the role
    if role == "relay":
        node.cores = 8          # Relays
        node.threads = 16
        node.ram = 16384         # 16GB RAM for handling heavy traffic

    elif role == "client":
        node.cores = 4           # Supports multiple clients per node
        node.threads = 8
        node.ram = 8192          # 8GB RAM

    elif role == "bulletin_board":
        node.cores = 8           # Moderate CPU for consistent availability
        node.threads = 16
        node.ram = 32768         # 32GB RAM

    # Add an interface for networking
    iface = node.addInterface("if1")
    iface.component_id = "eth1"

    # Install required packages
    node.addService(rspec.Execute(
        shell="bash",
        command="sudo apt update && sudo apt install -y git snapd wget tar"
    ))
    node.addService(rspec.Execute(shell="bash", command="sudo snap install go --classic"))
    node.addService(rspec.Execute(shell="bash", command="sudo snap install yq"))

    # Install Prometheus
    prometheus_version = "2.54.1"
    prometheus_install_command = """
        cd /tmp && \
        wget https://github.com/prometheus/prometheus/releases/download/v%s/prometheus-%s.linux-amd64.tar.gz && \
        tar -xvf prometheus-%s.linux-amd64.tar.gz && \
        sudo mv prometheus-%s.linux-amd64/prometheus /usr/bin/ && \
        sudo mv prometheus-%s.linux-amd64/promtool /usr/bin/ && \
        rm -rf prometheus-%s.linux-amd64*
    """ % (prometheus_version, prometheus_version, prometheus_version,
           prometheus_version, prometheus_version, prometheus_version)
    node.addService(rspec.Execute(shell="bash", command=prometheus_install_command))

    node.addService(rspec.Execute(shell="bash", command="which prometheus"))

    # Clone the repository and set up directory permissions
    clone_command = """
        sudo mkdir -p /home/Ismail/pi_t-experiment && sudo chown -R $USER:$USER /home/Ismail/pi_t-experiment && \
        cd /home/Ismail && git clone https://github.com/DrIsmailBadrez/pi_t-experiment.git && \
        cd /home/Ismail/pi_t-experiment && git config --global --add safe.directory /home/Ismail/pi_t-experiment
    """
    node.addService(rspec.Execute(shell="bash", command=clone_command))

    return node

# Function to run multiple clients or relays on a node
def run_service_instances(node, role, count, start_index, base_port=9000):
    for i in range(count):
        instance_index = start_index + i
        port = base_port + i
        command = f"""
            cd /home/ismail/pi_t-experiment && \
            sudo chmod +x bin/runNode.sh && \
            sudo ./bin/runNode.sh {role} {instance_index} {port}
        """
        node.addService(rspec.Execute(shell="bash", command=command))

# Create and configure relay nodes (5 relays per node)
for i in range(50):
    relay_node = add_node(f"relay{i + 1}", role="relay")
    run_service_instances(relay_node, "relay", 5, i * 5 + 1)

# Create and configure client nodes (10 clients per node)
for i in range(100):
    client_node = add_node(f"client{i + 1}", role="client")
    run_service_instances(client_node, "client", 10, i * 10 + 1)

# Print the generated RSpec
portal.context.printRequestRSpec()
