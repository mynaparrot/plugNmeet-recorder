#!/bin/bash

#
# STAGE 2: Pre-Transcoding Script (runs on Transcoder)
#
# Exit immediately if a command exits with a non-zero status.
set -e

# This script's purpose is to get the raw file(s) from network storage
# onto the transcoder's local disk, ready for ffmpeg.

# For this to work, you may need tools like 'jq' and 'rsync'.

# 1. Read the JSON job payload from stdin
input_json=$(cat)

# 2. (Optional) Log the input for debugging to stderr
# echo "Pre-Transcoding: Received input: $input_json" >&2

# 3. Use 'jq' to extract the task type
task_type=$(echo "$input_json" | jq -r .task)
recording_id=$(echo "$input_json" | jq -r .recording_id)

# 4. Define a local staging directory for this job
local_staging_dir="/tmp/transcode-staging/$recording_id"
mkdir -p "$local_staging_dir"

# 5. Perform the download/copy operation based on the task type
if [ "$task_type" == "merge" ]; then
  # For merge tasks, 'input_paths' is an array.
  # We iterate through it and copy each file to our local staging directory.
  echo "Pre-Transcoding: Handling 'merge' task." >&2

  # Read the file paths into a bash array
  # Using 'input_paths' as per the new naming convention.
  mapfile -t file_paths < <(echo "$input_json" | jq -r '.input_paths[]')

  for file_path in "${file_paths[@]}"; do
    echo "Copying file: $file_path" >&2
    rsync -av "$file_path" "$local_staging_dir/"
  done

else
  # For "single" tasks, we just copy the one file.
  echo "Pre-Transcoding: Handling 'single' task." >&2
  # Using 'input_path' as per the new naming convention.
  network_path=$(echo "$input_json" | jq -r .input_path)
  original_filename=$(echo "$input_json" | jq -r .file_name)
  full_network_path="$network_path/$original_filename"

  echo "Copying file: $full_network_path" >&2
  rsync -av "$full_network_path" "$local_staging_dir/"
fi

# 6. Modify the JSON with the new *local* path and print it to stdout.
# The Go application will now use this path as the base for ffmpeg operations.
# Using 'output_path' as per the new naming convention for the result path.
output_json=$(echo "$input_json" | jq --arg new_path "$local_staging_dir" '.output_path = $new_path')

echo "$output_json"
