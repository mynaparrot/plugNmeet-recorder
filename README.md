# plugNmeet-recorder

The plugnmeet-recorder handles session recording and RTMP streaming for plugnmeet. It is designed for scalability and can record multiple sessions simultaneously. You can deploy multiple recorder instances to create a horizontally scalable, highly available recording infrastructure. The plugnmeet-server will automatically balance the load across the available recorders.

## Requirements

*   A Linux system (Ubuntu is recommended)
*   Google Chrome
*   PulseAudio
*   Xvfb (X virtual framebuffer)
*   FFmpeg

## Installation

These instructions are for Ubuntu.

### 1. Install Dependencies

```bash
# Install PulseAudio, Xvfb, and FFmpeg
sudo apt install -y pulseaudio xvfb ffmpeg

# Start PulseAudio and enable it to start on boot
pulseaudio -D --verbose --exit-idle-time=-1 --disallow-exit
systemctl --user enable pulseaudio

# Install Google Chrome
curl -fsSL https://dl-ssl.google.com/linux/linux_signing_key.pub | sudo gpg --dearmor -o /usr/share/keyrings/googlechrome-linux-keyring.gpg
echo "deb [arch=amd64 signed-by=/usr/share/keyrings/googlechrome-linux-keyring.gpg] http://dl.google.com/linux/chrome/deb/ stable main" | sudo tee /etc/apt/sources.list.d/google-chrome.list > /dev/null
sudo apt -y update && sudo apt -y install google-chrome-stable

# Optional: Install additional fonts
sudo apt install -y fonts-noto fonts-liberation
```

### 2. Install the Recorder

1.  Download the latest release for your architecture from the [releases page](https://github.com/mynaparrot/plugNmeet-recorder/releases) or use the [Docker image](https://hub.docker.com/r/mynaparrot/plugnmeet-recorder).
2.  Extract the downloaded archive and navigate into the new directory.
3.  Create a configuration file from the sample:

    ```bash
    cp config_sample.yaml config.yaml
    ```

## Configuration

Edit `config.yaml` to configure the recorder.

### Basic Configuration

*   **`nats_info`**: This section must match the NATS configuration in your `plugnmeet-server`'s `config.yaml`.
*   **`main_path`**: This should be the same as `recording_files_path` in your `plugnmeet-server`'s `config.yaml`.
*   **`plugNmeet_info`**: Update this section with your plugnmeet server details.

**Important:** If you use NFS or other network-mounted storage for `main_path`, ensure both the recorder and the `plugnmeet-server` can access it. Otherwise, users won't be able to download recordings. To prevent I/O errors and dropped frames caused by this network latency, it is **highly recommended** to set a local `temporary_dir` in `config.yaml`. The recorder will write to this local directory first and then automatically move the file to the final network path after the recording is complete.

### Multi-server Deployment

You can run multiple `plugnmeet-recorder` instances for load balancing and high availability. The `plugnmeet-server` will automatically choose a recorder based on server load.

When deploying multiple recorders:

*   Assign a unique `id` in each `config.yaml` (e.g., `node_01`, `node_02`).
*   Adjust the `max_limit` value in `config.yaml` based on each server’s capacity.

#### Operational Modes (Recorder & Transcoder Workers)

The `plugnmeet-recorder` application supports different operational modes, allowing for a highly scalable and resilient recording and transcoding pipeline.

Each instance of the `plugnmeet-recorder` can be configured to run in one of three modes via the `mode` setting in `config.yaml`:

*   **`both` (Default):** In this mode, a single `plugnmeet-recorder` instance performs both live session recording and post-processing (transcoding) of recorded files.
    *   **Workflow:** Records a session -> Publishes transcoding job -> Processes transcoding job.

*   **`recorderOnly`:** This instance will *only* handle live session recording. Once a raw recording file (e.g., `.mkv`) is captured, it will publish a transcoding job to a queue, and its responsibility for that recording ends.
    *   **Workflow:** Records a session -> Publishes transcoding job.

*   **`transcoderOnly`:** This instance will *only* process transcoding jobs. It subscribes to the transcoding job queue, fetches jobs one at a time, and executes the `ffmpeg` command to convert the raw recording file into a final, compressed MP4.
    *   **Workflow:** Subscribes to job queue -> Fetches transcoding job -> Processes transcoding job.

##### Benefits of this Architecture:

*   **Decoupling:** Recording and transcoding are separate concerns, preventing CPU-intensive post-processing from impacting live sessions.
*   **Scalability:** You can scale recording instances (`recorderOnly`) and transcoding instances (`transcoderOnly`) independently based on your workload.
*   **Resilience:** The job queue ensures that transcoding jobs are persistent. If a `transcoderOnly` worker fails, the job remains in the queue and will be picked up by another available worker, guaranteeing that all recordings are eventually processed.
*   **Resource Management:** `transcoderOnly` workers process one `ffmpeg` job at a time, preventing CPU overload on individual machines.

## Scripting Hooks

Scripting hooks allow you to automate tasks at different stages of the recording and transcoding process. This is especially powerful in a multi-server setup where recording and transcoding happen on different machines. For example, you can use hooks to automatically upload a raw recording to cloud storage from a `recorderOnly` instance, and then download it on a `transcoderOnly` instance for processing. This decouples your workflow and makes your storage management flexible.

### The Long-Lived & One-shot Process Model

For maximum performance, all hook scripts can be **long-lived processes** or **one-shot** commands. When the `plugnmeet-recorder` starts, it launches each configured script once. The script then runs continuously, waiting for requests. This eliminates the overhead of starting a new process for every hook event.

All communication happens over `stdin` and `stdout` using **newline-delimited JSON**.

*   **`stdin`**: Your script must read from `stdin` in a loop. Each line will be a complete JSON object representing a single request.
*   **`stdout`**: For each request it receives, your script must print a single line of JSON to `stdout`. This line is the response.
*   **`stderr`**: You can use `stderr` for logging. This output will be ignored by the recorder but is useful for debugging your script.

If multiple scripts are defined for a single hook, they form a pipeline: the **stdout** of the first script becomes the **stdin** for the second, and so on.

The application will validate that all configured scripts exist and have executable permissions on startup.

### Built-in `http-request` command

A special command `http-request` is available to send POST requests to an HTTP/HTTPS endpoint. This is useful for notifying external services without writing a full script.

**Example:**
`http-request http://localhost:8080/your/endpoint`

This will send the current JSON payload to the specified URL.

### Hook Stages

Scripts are executed in three stages:
1. **`post_recording`**: Runs on the RECORDER after the raw file is saved.
   - **Purpose**: Upload the raw file to shared storage (NFS, S3, etc.).
   - **Action**: Should return JSON with the `output_path` updated to the new network-accessible location for the transcoder.

2. **`pre_transcoding`**: Runs on the TRANSCODER before ffmpeg starts.
   - **Purpose**: Download the file from shared storage to a local path.
   - **Action**: Should return JSON with the `output_path` updated to the final local path for ffmpeg to use.

3. **`post_transcoding`**: Runs on the TRANSCODER after ffmpeg finishes.
   - **Purpose**: Final cleanup, notification, or upload of the processed file.
   - **Action**: Can optionally return JSON with the `output_path` updated (e.g., to an S3 URL) to be sent to the main plugNmeet server.

### How to Use

1.  **Create a Long-Lived Script:** A "script" can be any executable file (a shell script, a compiled Go program, etc.) that runs in a loop, reading from `stdin` and writing to `stdout`.
2.  **Enable in Config:** Add the path to your executable (or multiple executables) in the `hooks` section of your `config.yaml`. The order of scripts in the list defines the execution order of the chain.

    ```yaml
    # config.yaml
    hooks:
      post_recording:
        pool_size: 2
        hook_timeout: 1h
        scripts:
          - script: "/path/to/your/script.sh"
            is_one_shot: false
          - script: "http-request http://localhost:8080/your/endpoint"
            is_one_shot: true
      pre_transcoding:
        pool_size: 2
        hook_timeout: 1h
        scripts:
          - script: "/path/to/your/script.sh"
            is_one_shot: false
      post_transcoding:
        pool_size: 2
        hook_timeout: 1h
        scripts:
          - script: "/path/to/your/script.sh"
            is_one_shot: false
    ```

3.  **Make it Executable:** Ensure your script or program has execute permissions:

    ```bash
    chmod +x ./scripts/post-recording/upload.sh
    ```

### Script Data

When a script is executed, it receives a JSON payload via **stdin**. Your script can parse this JSON to get the information it needs. The `plugnmeet-recorder` sends the raw data payload directly.

**Example JSON Data (received by your script on `stdin`):**

```json
{
  "task": "single",
  "recording_id": "REC_ax9s3djn2s",
  "room_table_id": 123,
  "room_id": "room01",
  "room_sid": "SID_d82k3s9d2l",
  "file_name": "REC_ax9s3djn2s.mp4",
  "input_path": "/path/to/recording/files/node_01/room01/REC_ax9s3djn2s.mp4",
  "file_size": 123.45,
  "recorder_id": "node_01"
}
```

**Example JSON Data (returned by your script on `stdout`):**

Your script should return the same JSON structure, but with the `output_path` field added or modified to reflect the result of its operation.

```json
{
  "task": "single",
  "recording_id": "REC_ax9s3djn2s",
  "room_table_id": 123,
  "room_id": "room01",
  "room_sid": "SID_d82k3s9d2l",
  "file_name": "REC_ax9s3djn2s.mp4",
  "input_path": "/path/to/recording/files/node_01/room01/REC_ax9s3djn2s.mp4",
  "output_path": "s3://my-bucket/recordings/REC_ax9s3djn2s.mp4",
  "file_size": 123.45,
  "recorder_id": "node_01"
}
```

Example scripts are provided in the `./scripts` directory to demonstrate how to build a long-lived script that can parse this JSON using tools like `jq` and log the output. You can use these as a template for your own custom workflows.

## Development

1.  Clone this repository and navigate into the project directory.
2.  Create configuration files from the samples:

    ```bash
    cp config_sample.yaml config.yaml
    cp docker-compose_sample.yaml docker-compose.yaml
    ```
3.  Update `config.yaml` with your development settings.
4.  Build and start the development environment using Docker Compose:

    ```bash
    docker compose build
    docker compose up
    ```
