#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# This is an example post-processing script.
# It receives a JSON payload as the first command-line argument
# and can perform actions like calling an external API or logging.

# 1. Log the received data for debugging.
echo "Post-processing script triggered with data:"
echo "$1"

# 2. Example: Use jq to extract the recording ID and file path
# This requires jq to be installed.
# recording_id=$(echo "$1" | jq -r .recording_id)
# file_path=$(echo "$1" | jq -r .file_path)
# echo "Successfully processed recording $recording_id at path $file_path"

# 3. Example of calling an external API with the recording info
# curl -X POST -H "Content-Type: application/json" \
#   -d "$1" https://api.example.com/recording_processed
