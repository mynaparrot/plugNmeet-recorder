#!/bin/bash

#
# STAGE 1: Post-Recording Script (runs on Recorder)
# This script runs in a continuous loop, reading newline-delimited JSON from stdin.
# For each request, it must generate a JSON response to stdout.
#
# Exit immediately if a command exits with a non-zero status.
set -e

# Add a log function for easier debugging. Output is sent to stderr.
log() {
  echo "Post-Recording Hook: $1" >&2
}

log "Starting long-lived post-recording upload script..."

while read -r line; do
  log "Received request: $line"

  # The 'data' field contains the original JSON payload from the recorder.
  # We extract the necessary fields from the 'data' object.
  # Use a temporary variable to hold the 'data' part for easier parsing.
  DATA_JSON=$(echo "$line" | jq -c '.data')

  if [ -z "$DATA_JSON" ] || [ "$DATA_JSON" = "null" ]; then
    log "Error: 'data' field is missing or empty in request."
    jq -n '{"error": "data field is missing or empty"}'
    continue
  fi

  original_path=$(echo "$DATA_JSON" | jq -r '.input_path')
  original_filename=$(echo "$DATA_JSON" | jq -r '.file_name')
  recording_id=$(echo "$DATA_JSON" | jq -r '.recording_id')

  if [ -z "$original_path" ] || [ "$original_path" = "null" ]; then
    log "Error: input_path is missing from data."
    jq -n '{"error": "input_path is missing"}'
    continue
  fi
  if [ -z "$original_filename" ] || [ "$original_filename" = "null" ]; then
    log "Error: file_name is missing from data."
    jq -n '{"error": "file_name is missing"}'
    continue
  fi
  if [ -z "$recording_id" ] || [ "$recording_id" = "null" ]; then
    log "Error: recording_id is missing from data."
    jq -n '{"error": "recording_id is missing"}'
    continue
  fi

  full_original_path="$original_path/$original_filename"

  # --- Your Custom Logic Goes Here ---
  #
  # This is where you would perform the actual upload/move operation.
  # For example, moving the file to a network-accessible location for the transcoder.
  #
  # For a cloud setup, this is where you would use 'aws s3 cp' or similar.
  # Example for S3:
  #   aws s3 cp "$full_original_path" "s3://your-bucket/recordings/$recording_id/$original_filename"
  #   NEW_PATH_FOR_TRANSCODER="s3://your-bucket/recordings/$recording_id"
  #
  # For rclone:
  #   rclone copyto "$full_original_path" "my-remote:recordings/$recording_id/$original_filename"
  #   NEW_PATH_FOR_TRANSCODER="my-remote:recordings/$recording_id"

  # In this example, we simulate moving the file to a shared NFS mount.
  shared_storage_base_path="/mnt/recordings"
  new_path_on_shared_storage="$shared_storage_base_path/$recording_id"
  new_full_path="$new_path_on_shared_storage/$original_filename"

  log "Attempting to move $full_original_path to $new_full_path"

  # Ensure the destination directory exists
  mkdir -p "$new_path_on_shared_storage" || {
    log "Error: Failed to create directory $new_path_on_shared_storage"
    jq -n --arg err "Failed to create directory $new_path_on_shared_storage" '{"error": $err}'
    continue
  }

  # Use rsync to move the file and remove the source
  rsync -av --remove-source-files "$full_original_path" "$new_full_path" || {
    log "Error: Failed to rsync file $full_original_path to $new_full_path"
    jq -n --arg err "Failed to rsync file $full_original_path to $new_full_path" '{"error": $err}'
    continue
  }

  log "Successfully moved $full_original_path to $new_full_path"
  NEW_PATH_FOR_TRANSCODER="$new_path_on_shared_storage"

  # --- End of Custom Custom Logic ---

  # The final step is to output a JSON object to stdout.
  # This new path is what the transcoder will receive in its job payload.
  # We return the original data, but with the output_path updated.
  echo "$DATA_JSON" | jq --arg new_path "$NEW_PATH_FOR_TRANSCODER" '.output_path = $new_path'

done