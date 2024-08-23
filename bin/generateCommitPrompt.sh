#!/bin/bash

# Constants
MAX_DIFF_LENGTH=10000  # Adjust this value based on your needs

# Function to check if the current directory is a Git repository
check_git_repository() {
    git rev-parse --is-inside-work-tree > /dev/null 2>&1
}

# Function to load ignore patterns from .ai-commit.json
load_ignore_patterns() {
    local config_path=".ai-commit.json"  # Define the path to the configuration file
    local ignore_patterns=()  # Initialize an empty array to hold ignore patterns

    # Check if the configuration file exists
    if [ -f "$config_path" ]; then
        # Use jq to parse the ignore patterns from the JSON file
        ignore_patterns=($(jq -r '.ignore[]' "$config_path"))
    fi

    # Return the ignore patterns as a space-separated string
    echo "${ignore_patterns[@]}"
}

# Function to filter the diff based on ignore patterns
filter_diff() {
    local diff="$1"
    local ignore_patterns=("${!2}")

    local filtered_lines=()
    local skip_block=false
    IFS=$'\n' read -r -a lines <<< "$diff"

    for line in "${lines[@]}"; do
        skip_block=false
        for pattern in "${ignore_patterns[@]}"; do
            if [[ "$line" =~ ${pattern//\*/.*} ]]; then
                skip_block=true
                break
            fi
        done

        if ! $skip_block; then
            filtered_lines+=("$line")
        fi

        if [[ "$line" =~ ^diff\ --git ]]; then
            skip_block=false
        fi
    done

    printf "%s\n" "${filtered_lines[@]}"
}

# Check if the directory is a Git repository
if ! check_git_repository; then
    echo "This is not a git repository 🙅‍♂️"
    exit 1
fi

# Get the staged diff, ignoring space changes
diff=$(git diff --staged --ignore-space-change)

# Remove empty lines and stars
diff=$(echo "$diff" | sed '/^\+[\s]*$/d' | sed '/^[[:space:]]*$/d' | sed 's/\*//g')

# Truncate the diff if it's too large
if [ ${#diff} -gt $MAX_DIFF_LENGTH ]; then
    diff="${diff:0:$MAX_DIFF_LENGTH}\n... [diff truncated]"
fi

# Exit if there's no diff
if [ -z "$diff" ]; then
    echo "No changes to commit 🙅"
    echo "Maybe you forgot to add the files? Try 'git add .' and then run this script again."
    exit 1
fi

# Load ignore patterns from .ai-commit.json if it exists
ignore_patterns=($(load_ignore_patterns))

# Filter the diff based on ignore patterns
filtered_diff=$(filter_diff "$diff" ignore_patterns[@])

# Exit if there's no relevant diff after filtering
if [ -z "$filtered_diff" ]; then
    echo "No relevant changes to commit after applying ignore patterns 🙅"
    exit 0
fi

# Prepare the prompt for generating a commit message
prompt=$(cat <<EOF
Please act as the author of a git commit message. I will provide you with a git diff, and your task is to convert it into a detailed, informative commit message.
To help you understand the git diff output:
    1. File Comparison Line: Shows the files being compared.
    2. Index Line: Indicates the blob hashes before and after the change and the file mode.
    3. File Change Markers: --- shows the file before the change and +++ shows the file after the change.
    4. Hunk Header: Indicates the location and number of lines affected in the files.
       Example: @@ -1,5 +1,7 @@ means the changes start at line 1 and cover 5 lines in the original file and start at line 1 and cover 7 lines in the new file.

    5. Changes: Lines starting with - are removed lines. Lines starting with + are added lines. Some unchanged lines may be shown for context.
    Example:
    diff
    diff --git file1.txt file1.txt
    index e69de29..d95f3ad 100644
    --- file1.txt
    +++ file1.txt
    @@ -0,0 +1,2 @@
    -This line was removed.
    +This is a new line.
    +Another new line.

Here\'s how you can structure your commit message:
    Summary: <A concise, one-line sentence in the present tense that summarizes all changes (50 characters or less)>.
    Description: <A detailed explanation of all changes in the past tense.>

Important:
    1. The summary must be in the present tense, e.g., Fix login issue, edit variables,....
    2. The description must be in the past tense, e.g., This change fixed a bug by....
    3. Avoid prefacing your response with any additional text.
    4. The summary and description should cover ALL changes and focus on the most important ones.

Here is the git diff, which you are to convert into a commit message as described:

$filtered_diff
EOF
)

# Print the prompt
echo "$prompt"

# Copy to clipboard (macOS)
echo -e "$prompt" | pbcopy