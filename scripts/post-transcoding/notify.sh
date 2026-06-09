#!/bin/bash

#
# STAGE 3: Post-Transcoding Script (runs on Transcoder)
#
# Exit immediately if a command exits with a non-zero status.
set -e

# This script's purpose is to perform final actions after the recording
# has been successfully transcoded. It's a "fire-and-forget" hook.

# 1. Log the received data for debugging.
# The JSON payload is passed as the first command-line argument.
echo "Post-Transcoding: Script triggered with data:"
echo "$1"

# 2. Example: Use jq to extract the recording ID and final file path
# This requires jq to be installed.
# recording_id=$(echo "$1" | jq -r .recording_id)
# file_path=$(echo "$1" | jq -r .file_path)
# echo "Successfully processed recording $recording_id at path $file_path"

# 3. Example of calling an external API to notify that the recording is ready.
# curl -X POST -H "Content-Type: application/json" \
#   -d "$1" https://api.example.com/recording_processed
