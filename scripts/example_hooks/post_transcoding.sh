#!/bin/bash

#
# STAGE 3: Post-Transcoding Script (runs on Transcoder)
# This script runs in a continuous loop, reading newline-delimited JSON from stdin.
# For each request, it can perform final actions like notifying an external API.
#
# Exit immediately if a command exits with a non-zero status.
set -e

# Add a log function for easier debugging. Output is sent to stderr.
log() {
  echo "Post-Transcoding Hook: $1" >&2
}

log "Starting long-lived post-transcoding notify script..."

while read -r line; do
  log "Received request: $line"

  # The 'data' field contains the original JSON payload from the transcoder job.
  DATA_JSON=$(echo "$line" | jq -c '.data')

  if [ -z "$DATA_JSON" ] || [ "$DATA_JSON" = "null" ]; then
    log "Error: 'data' field is missing or empty in request."
    jq -n '{"error": "data field is missing or empty"}'
    continue
  fi

  # --- Your Custom Logic Goes Here ---
  #
  # This is where you would perform final actions like notifying an external API,
  # moving the final file to permanent storage, or cleaning up temporary files.

  recording_id=$(echo "$DATA_JSON" | jq -r '.recording_id')
  final_file_path=$(echo "$DATA_JSON" | jq -r '.output_path') # Path of the transcoded file

  log "Successfully processed recording $recording_id at path $final_file_path"

  # Example: Notify an external API that the recording is ready.
  # log "Sending notification to external API..."
  # curl -X POST -H "Content-Type: application/json" \
  #   -d "$DATA_JSON" https://api.example.com/recording_processed || log "Warning: Failed to notify external API."

  # Example: Upload the final transcoded file to a permanent cloud storage.
  # log "Uploading final file to permanent storage..."
  # final_cloud_path="s3://my-final-bucket/recordings/$recording_id.mp4"
  # aws s3 cp "$final_file_path" "$final_cloud_path" || {
  #   log "Error: Failed to upload final file to S3."
  #   jq -n --arg err "Failed to upload final file" '{"error": $err}'
  #   continue
  # }
  #
  # # If you upload, you might want to update the output_path for the next script in the chain.
  # DATA_JSON=$(echo "$DATA_JSON" | jq --arg new_path "$final_cloud_path" '.output_path = $new_path')

  # --- End of Custom Custom Logic ---

  # The final step is to output a JSON object to stdout.
  # This can be the original data, or modified data if you changed something.
  # This output will be passed to the next script in the post_transcoding hook chain, if any.
  echo "$DATA_JSON"

done
