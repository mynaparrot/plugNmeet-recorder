#!/bin/bash

#
# STAGE 3: Post-Transcoding Script (runs on Transcoder)
#
# Exit immediately if a command exits with a non-zero status.
set -e

# This script's purpose is to perform final actions after the recording
# has been successfully transcoded.

# 1. Read the JSON payload from stdin.
input_data=$(cat)

# 2. Log the received data for debugging.
echo "Post-Transcoding: Script triggered with data:"
echo "$input_data"

# 3. Example: Use jq to extract the recording ID and final file path.
# This requires jq to be installed.
# recording_id=$(echo "$input_data" | jq -r .recording_id)
# file_path=$(echo "$input_data" | jq -r .file_path)
# echo "Successfully processed recording $recording_id at path $file_path"

# 4. Example of calling an external API to notify that the recording is ready.
# curl -X POST -H "Content-Type: application/json" \
#   -d "$input_data" https://api.example.com/recording_processed

# 5. Example of modifying the data and passing it to the next script in the chain.
# If this script is part of a chain in the `post_transcoding_hooks`, its stdout
# will be the stdin for the next script.
#
# Here, we could change the file_path if we uploaded it to a cloud service.
# new_path="s3://my-bucket/recordings/$recording_id.mp4"
# updated_data=$(echo "$input_data" | jq --arg new_path "$new_path" '.file_path = $new_path')
# echo "$updated_data"
