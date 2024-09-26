#!/bin/bash

# Path to the config.yml file
CONFIG_FILE="/home/Ismail/pi_t-experiment/config/config.yml"

# Function to kill all processes started in the background
terminate_processes() {
    echo "Terminating process..."
    sudo curl -X POST "$ADDRESS/shutdown" > /dev/null 2>&1
    sudo kill -9 $SCRIPT_PID
    exit 0
}

# Set up a trap to catch the SIGINT (Ctrl+C)
trap "terminate_processes" SIGINT

# Check if the correct number of parameters are provided
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <type> <id>"
    echo "Type should be 'client', 'relay', or 'bulletin_board'"
    exit 1
fi

type=$1
id=$2

# Print the type and ID
echo "Starting $type with ID: $id"

# Find the root directory of the project by locating a known file or directory
PROJECT_ROOT="$(sudo git rev-parse --show-toplevel 2>/dev/null)"

if [ -z "$HOME/pi_t-experiment" ]; then
    echo "Error: Unable to determine the project root directory. Are you inside a Git repository?"
    exit 1
fi

sudo ls

sudo pwd

echo "Printing content of ./config: "
sudo ls ./config

echo "Trying to open config file at: $CONFIG_FILE"
ls -l $CONFIG_FILE

sudo chown root:root /home/Ismail/pi_t-experiment/config/config.yml

# Handle Bulletin Board
if [ "$type" = "bulletin_board" ]; then
    HOST=$(sudo yq e ".bulletin_board | .host" $CONFIG_FILE)
    PORT=$(sudo yq e ".bulletin_board | .port" $CONFIG_FILE)

    if [ -z "$HOST" ] || [ -z "$PORT" ]; then
        echo "Bulletin board not found in the configuration."
        exit 1
    fi

    ADDRESS="http://$HOST:$PORT"

    echo "Bulletin board address: $ADDRESS"

    # Start the bulletin board process in the background
    sudo go run cmd/bulletin-board/main.go &
    SCRIPT_PID=$!

elif [ "$type" = "client" ]; then
    HOST=$(sudo yq e ".clients[] | select(.id == $id) | .host" $CONFIG_FILE)
    PORT=$(sudo yq e ".clients[] | select(.id == $id) | .port" $CONFIG_FILE)

    if [ -z "$HOST" ] || [ -z "$PORT" ]; then
        echo "Client with ID $id not found in the configuration."
        exit 1
    fi

    ADDRESS="http://$HOST:$PORT"
    echo "Client $id address: $ADDRESS"

    # Start the client process in the background
    sudo go run cmd/client/main.go -id "$id" &
    SCRIPT_PID=$!

elif [ "$type" = "relay" ]; then
    HOST=$(sudo yq e ".relays[] | select(.id == $id) | .host" $CONFIG_FILE)
    PORT=$(sudo yq e ".relays[] | select(.id == $id) | .port" $CONFIG_FILE)

    if [ -z "$HOST" ] || [ -z "$PORT" ]; then
        echo "Relay with ID $id not found in the configuration."
        exit 1
    fi

    ADDRESS="http://$HOST:$PORT"
    echo "Relay $id address: $ADDRESS"

    # Start the relay process in the background
    sudo go run cmd/relay/main.go -id "$id" &
    SCRIPT_PID=$!
else
    echo "Invalid type: $type. Must be 'client', 'relay', or 'bulletin_board'."
    exit 1
fi

# Wait for the user to send SIGINT (Ctrl+C)
while true; do
    sleep 1
done

# Exit the script
exit 0
