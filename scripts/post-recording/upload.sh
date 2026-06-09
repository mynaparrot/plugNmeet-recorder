#!/bin/bash

#
# STAGE 1: Post-Recording Script (runs on Recorder)
#
# Exit immediately if a command exits with a non-zero status.
set -e

# This script's purpose is to move the raw recording file from the recorder's
# local disk to a network-accessible location for the transcoder.

# For this to work, you may need tools like 'jq' and 'rsync' installed.

# 1. Read the original JSON from stdin
input_json=$(cat)

# 2. (Optional) Log the input for debugging to stderr
# echo "Post-Recording: Received input: $input_json" >&2

# 3. Use 'jq' to extract values
original_path=$(echo "$input_json" | jq -r .file_path)
original_filename=$(echo "$input_json" | jq -r .file_name)
recording_id=$(echo "$input_json" | jq -r .recording_id)
full_original_path="$original_path/$original_filename"

# 4. Perform the upload/move operation.
# In this example, we move the file to a shared NFS mount that the transcoder
# can access. For a cloud setup, this is where you would use 'aws s3 cp'.

# Let's assume the transcoder will access files from a shared mount at `/mnt/recordings`
shared_storage_path="/mnt/recordings"
new_path_on_shared_storage="$shared_storage_path/$recording_id"
new_full_path="$new_path_on_shared_storage/$original_filename"

# Create the directory on the destination and move the file.
mkdir -p "$new_path_on_shared_storage"
rsync -av --remove-source-files "$full_original_path" "$new_full_path"

# 5. Modify the JSON with the new path and print it to stdout.
# This new path is what the transcoder will receive in its job payload.
output_json=$(echo "$input_json" | jq --arg new_path "$new_path_on_shared_storage" '.file_path = $new_path')

echo "$output_json"
