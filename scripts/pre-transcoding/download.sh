#!/bin/bash

#
# STAGE 2: Pre-Transcoding Script (runs on Transcoder)
# This script runs in a continuous loop, reading newline-delimited JSON from stdin.
# For each request, it must generate a JSON response to stdout.
#
# Exit immediately if a command exits with a non-zero status.
set -e

# Add a log function for easier debugging. Output is sent to stderr.
log() {
  echo "Pre-Transcoding Hook: $1" >&2
}

log "Starting long-lived pre-transcoding download script..."

while read -r line; do
  log "Received request: $line"

  # The 'data' field contains the original JSON payload from the transcoder job.
  DATA_JSON=$(echo "$line" | jq -c '.data')

  if [ -z "$DATA_JSON" ] || [ "$DATA_JSON" = "null" ]; then
    log "Error: 'data' field is missing or empty in request."
    jq -n '{"error": "data field is missing or empty"}'
    continue
  fi

  task_type=$(echo "$DATA_JSON" | jq -r '.task')
  recording_id=$(echo "$DATA_JSON" | jq -r '.recording_id')

  if [ -z "$task_type" ] || [ "$task_type" = "null" ]; then
    log "Error: task is missing from data."
    jq -n '{"error": "task is missing"}'
    continue
  fi
  if [ -z "$recording_id" ] || [ "$recording_id" = "null" ]; then
    log "Error: recording_id is missing from data."
    jq -n '{"error": "recording_id is missing"}'
    continue
  fi

  # Define a local staging directory for this job
  local_staging_dir="/tmp/transcode-staging/$recording_id"
  mkdir -p "$local_staging_dir" || {
    log "Error: Failed to create local staging directory $local_staging_dir"
    jq -n --arg err "Failed to create directory $local_staging_dir" '{"error": $err}'
    continue
  }

  # --- Your Custom Logic Goes Here ---
  #
  # This is where you would perform the actual download/copy operation from network storage
  # onto the transcoder's local disk.

  if [ "$task_type" == "merge" ]; then
    log "Handling 'merge' task for recording_id: $recording_id"

    # Read the file paths into a bash array
    mapfile -t file_paths < <(echo "$DATA_JSON" | jq -r '.input_paths[]')

    for file_path in "${file_paths[@]}"; do
      log "Copying file: $file_path to $local_staging_dir/"
      rsync -av "$file_path" "$local_staging_dir/" || {
        log "Error: Failed to rsync file $file_path"
        jq -n --arg err "Failed to rsync file $file_path" '{"error": $err}'
        continue 2 # Continue outer loop
      }
    done

  else # Assuming "single" task type
    log "Handling 'single' task for recording_id: $recording_id"
    network_path=$(echo "$DATA_JSON" | jq -r '.input_path')
    original_filename=$(echo "$DATA_JSON" | jq -r '.file_name')

    if [ -z "$network_path" ] || [ "$network_path" = "null" ]; then
      log "Error: input_path is missing from data for single task."
      jq -n '{"error": "input_path is missing"}'
      continue
    fi
    if [ -z "$original_filename" ] || [ "$original_filename" = "null" ]; then
      log "Error: file_name is missing from data for single task."
      jq -n '{"error": "file_name is missing"}'
      continue
    fi

    full_network_path="$network_path/$original_filename"

    log "Copying file: $full_network_path to $local_staging_dir/"
    rsync -av "$full_network_path" "$local_staging_dir/" || {
      log "Error: Failed to rsync file $full_network_path"
      jq -n --arg err "Failed to rsync file $full_network_path" '{"error": $err}'
      continue
    }
  fi
  # --- End of Custom Custom Logic ---

  # Modify the JSON with the new *local* path and print it to stdout.
  # The Go application will now use this path as the base for ffmpeg operations.
  echo "$DATA_JSON" | jq --arg new_path "$local_staging_dir" '.output_path = $new_path'

done