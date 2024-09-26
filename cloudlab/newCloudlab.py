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
    node.addService(RSpec.Execute(shell="bash", command="cd $HOME && git clone https://github.com/DrIsmailBadrez/pi_t-experiment.git && cd $HOME/pi_t-experiment && git config --global --add safe.directory $HOME/pi_t-experiment"))

    return node

# Create nodes for bulletin_board, relays, and clients
bulletin_board = add_node("bulletin_board", "pc3000", "bulletin_board", 0)

# Relay nodes (assuming you want 6 relays)
relays = [add_node("relay%d" % i, "pc3000", "relay", i) for i in range(1, 7)]

# Client nodes (assuming you want 6 clients)
clients = [add_node("client%d" % i, "pc3000", "client", i) for i in range(1, 7)]

# Function to dynamically create config file with the correct IP addresses
def create_config_file():
    # Get the IP addresses of the bulletin board, relays, and clients
    bulletin_board_ip = bulletin_board.getInterfaces()[0].getIPv4()
    relay_ips = [relay.getInterfaces()[0].getIPv4() for relay in relays]
    client_ips = [client.getInterfaces()[0].getIPv4() for client in clients]

    config_content = f"""
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
  host: '{bulletin_board_ip}'
  port: 8080
clients:
"""
    for i, client_ip in enumerate(client_ips):
        config_content += f"  - id: {i+1}\n    host: '{client_ip}'\n    port: {8100 + i + 1}\n    prometheus_port: {9100 + i + 1}\n"

    config_content += "\nrelays:\n"
    for i, relay_ip in enumerate(relay_ips):
        config_content += f"  - id: {i+1}\n    host: '{relay_ip}'\n    port: {8080 + i + 1}\n    prometheus_port: {9200 + i + 1}\n"

    return config_content

# Once the IP addresses are gathered, generate the config file and send it to all nodes
config_file_content = create_config_file()

# Write the config file to each node dynamically
for node in [bulletin_board] + relays + clients:
    node.addService(RSpec.Execute(shell="bash", command=f"echo '{config_file_content}' > $HOME/pi_t-experiment/config/config.yml"))
        # Command to run the services based on role and index
    command = "cd $HOME/pi_t-experiment && ls && sudo chmod +x bin/runNode.sh && pwd && ls $HOME/pi_t-experiment/config && sudo ./bin/runNode.sh %s %d" % (role, index)
    node.addService(RSpec.Execute(shell="bash", command=command))

# Print the generated RSpec
portal.context.printRequestRSpec(request)
