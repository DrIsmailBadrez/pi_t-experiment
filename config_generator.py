import os

# Function to generate config.yml with the fetched IP addresses
def generate_config_file(bulletin_board_ip, relay_ips, client_ips):
    config_content = f"""
l1: 3         # Number of planned mixers in a routing path
l2: 2         # Number of planned gatekeepers in a routing path
x: 25          # Server load (x = Ω(polylog λ)) i.e. the expected number of onions per intermediary hop
tau: 0.8      # (τ < (1 − γ)(1 − X)) Fraction of checkpoints needed to progress local clock
d: 2          # Threshold for number of bruises before an onion is discarded by a gatekeeper
delta: 1e-5   # The probability of differential privacy violation due to the adversary's actions.
chi: 1.0      # Fraction of corrupted relays (which perform no mixing)
vis: true     # Visualize the network
scrapeInterval: 1000 # Prometheus scrape interval in milliseconds
dropAllOnionsFromClient: 1 # Client ID to drop all onions from
prometheusPath: '/opt/homebrew/bin/prometheus'

metrics:
  host: 'localhost'
  port: 8200
bulletin_board:
  host: '{bulletin_board_ip}'
  port: 8080
clients:
"""

    for i, client_ip in enumerate(client_ips, start=1):
        config_content += f"""
  - id: {i}
    host: '{client_ip}'
    port: 810{i}
    prometheus_port: 910{i}
"""

    config_content += """
relays:
"""
    for i, relay_ip in enumerate(relay_ips, start=1):
        config_content += f"""
  - id: {i}
    host: '{relay_ip}'
    port: 808{i}
    prometheus_port: 920{i}
"""

    # Write the config.yml file to the correct location
    with open('/local/repository/pi_t-experiment/config/config.yml', 'w') as config_file:
        config_file.write(config_content)

# Replace these IPs with the IPs dynamically retrieved from the nodes during deployment
bulletin_board_ip = os.getenv("BB_IP")
relay_ips = os.getenv("RELAY_IPS").split(',')
client_ips = os.getenv("CLIENT_IPS").split(',')

# Generate the config file
generate_config_file(bulletin_board_ip, relay_ips, client_ips)
