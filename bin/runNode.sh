#!/bin/bash

# Path to the config.yml file
CONFIG_FILE="/home/Ismail/pi_t-experiment/config/config.yml"

# Function to kill all processes started in the background
terminate_processes() {
    echo "Terminating process..."
    sudo kill -9 $SCRIPT_PID
    exit 0
}

# Set up a trap to catch the SIGINT (Ctrl+C)
trap "terminate_processes" SIGINT

# Check if the correct number of parameters are provided
if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <type> <id> <port>"
    echo "Type should be 'client', 'relay', or 'bulletin_board'"
    exit 1
fi

type=$1
id=$2
port=$3

# Print the type, ID, and port
echo "Starting $type with ID: $id on port: $port"

# Find the root directory of the project by locating a known file or directory
PROJECT_ROOT="$(sudo git rev-parse --show-toplevel 2>/dev/null)"

if [ -z "$PROJECT_ROOT" ]; then
    echo "Error: Unable to determine the project root directory. Are you inside a Git repository?"
    exit 1
fi

# Change directory to project root
cd "$PROJECT_ROOT" || exit 1

# Print the contents of the config directory for verification
echo "Printing content of ./config:"
ls ./config

echo "Trying to open config file at: $CONFIG_FILE"
ls -l $CONFIG_FILE

# Ensure correct permissions for the config file
sudo chown root:root "$CONFIG_FILE"

# Handle Bulletin Board
if [ "$type" = "bulletin_board" ]; then
    echo "Starting bulletin board..."

    # Start the bulletin board process in the background
    sudo go run cmd/bulletin-board/main.go &
    SCRIPT_PID=$!

elif [ "$type" = "client" ]; then
    echo "Starting client $id on port $port..."

    # Retrieve the bulletin board host from the config file
    BULLETIN_BOARD_HOST=$(sudo yq e ".bulletin_board.host" "$CONFIG_FILE")

    if [ -z "$BULLETIN_BOARD_HOST" ]; then
        echo "Bulletin board configuration not found."
        exit 1
    fi

    # Start the client process in the background with the bulletin board host and port
    sudo go run cmd/client/main.go -id "$id" -host="$BULLETIN_BOARD_HOST" -port="$port" &
    SCRIPT_PID=$!

elif [ "$type" = "relay" ]; then
    echo "Starting relay $id on port $port..."

    # Retrieve the bulletin board host from the config file
    BULLETIN_BOARD_HOST=$(sudo yq e ".bulletin_board.host" "$CONFIG_FILE")

    if [ -z "$BULLETIN_BOARD_HOST" ]; then
        echo "Bulletin board configuration not found."
        exit 1
    fi

    # Start the relay process in the background with the bulletin board host and port
    sudo go run cmd/relay/main.go -id "$id" -host="$BULLETIN_BOARD_HOST" -port="$port" &
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
