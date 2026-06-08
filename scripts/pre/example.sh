#!/bin/sh

# This is an example pre-transcode script.
# It reads a JSON payload from stdin, can perform actions,
# modifies the JSON, and prints the new JSON to stdout.
#
# The primary use case is to handle file transfers and path modifications
# between a recorder and a transcoder that do not share the same filesystem.
#
# For this to work, you may need tools like 'jq' and 'rsync' installed.

# 1. Read the original JSON from stdin
input_json=$(cat)

# 2. (Optional) Log the input for debugging to stderr
# echo "Received input: $input_json" >&2

# 3. Use 'jq' to extract values
original_path=$(echo "$input_json" | jq -r .file_path)
original_filename=$(echo "$input_json" | jq -r .file_name)
recording_id=$(echo "$input_json" | jq -r .recording_id)
full_original_path="$original_path/$original_filename"

# 4. Perform your custom logic here.
# For example, moving the file from the recorder's local storage
# to a shared NFS mount that the transcoder can access.

# Let's assume the transcoder expects files in `/mnt/transcode_jobs`
transcoder_base_path="/mnt/transcode_jobs"
new_path_on_transcoder="$transcoder_base_path/$recording_id"
new_full_path="$new_path_on_transcoder/$original_filename"

# Create the directory on the destination and move the file
# The `rsync` command is a robust way to move files, even across mounts.
# The `--remove-source-files` flag makes it a "move" operation.
mkdir -p "$new_path_on_transcoder"
rsync -av --remove-source-files "$full_original_path" "$new_full_path"

# 5. Modify the JSON with the new path for the transcoder and print it to stdout.
# The transcoder will now look for the file at this new `file_path`.
output_json=$(echo "$input_json" | jq --arg new_path "$new_path_on_transcoder" '.file_path = $new_path')

echo "$output_json"
