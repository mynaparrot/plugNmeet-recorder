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
*   **`plugNmeet_info`**: Update this section with your plugNmeet server details.

**Important:** If you use NFS or other network-mounted storage for recordings, ensure both the recorder and the `plugnmeet-server` can access the storage path. Otherwise, users' won't be able to download recordings.

### Multi-server Deployment

You can run multiple `plugnmeet-recorder` instances for load balancing and high availability. The `plugnmeet-server` will automatically choose a recorder based on server load.

When deploying multiple recorders:

*   Assign a unique `id` in each `config.yaml` (e.g., `node_01`, `node_02`).
*   Adjust the `max_limit` value in `config.yaml` based on each server’s capacity.

## Operational Modes (Recorder & Transcoder Workers)

The `plugnmeet-recorder` application supports different operational modes, allowing for a highly scalable and resilient recording and transcoding pipeline.

Each instance of the `plugnmeet-recorder` can be configured to run in one of three modes via the `recorder.mode` setting in `config.yaml`:

*   **`both` (Default):** In this mode, a single `plugnmeet-recorder` instance performs both live session recording and post-processing (transcoding) of recorded files.
    *   **Workflow:** Records a session -> Publishes transcoding job -> Processes transcoding job.

*   **`recorderOnly`:** This instance will *only* handle live session recording. Once a raw recording file (e.g., `.mkv`) is captured, it will publish a transcoding job to a queue, and its responsibility for that recording ends.
    *   **Workflow:** Records a session -> Publishes transcoding job.

*   **`transcoderOnly`:** This instance will *only* process transcoding jobs. It subscribes to the transcoding job queue, fetches jobs one at a time, and executes the `ffmpeg` command to convert the raw recording file into a final, compressed MP4.
    *   **Workflow:** Subscribes to job queue -> Fetches transcoding job -> Processes transcoding job.

### Benefits of this Architecture:

*   **Decoupling:** Recording and transcoding are separate concerns, preventing CPU-intensive post-processing from impacting live sessions.
*   **Scalability:** You can scale recording instances (`recorderOnly`) and transcoding instances (`transcoderOnly`) independently based on your workload.
*   **Resilience:** The job queue ensures that transcoding jobs are persistent. If a `transcoderOnly` worker fails, the job remains in the queue and will be picked up by another available worker, guaranteeing that all recordings are eventually processed.
*   **Resource Management:** `transcoderOnly` workers process one `ffmpeg` job at a time, preventing CPU overload on individual machines.

## Running the Recorder

To start the recorder, run the binary for your system's architecture:

```bash
# For AMD64
./plugnmeet-recorder-linux-amd64

# For ARM64
./plugnmeet-recorder-linux-arm64
```

## Deployment Recommendations

For optimal performance and resource utilization, especially in production environments with frequent recordings, it is highly recommended to deploy `plugnmeet-recorder` instances on dedicated servers. Both live recording and transcoding can be CPU-intensive processes.

Review the "Operational Modes" section above. Based on your specific use case and desired load distribution, deploy `plugnmeet-recorder` instances in the appropriate modes across dedicated servers.

For a streamlined and automated installation on a single server, you can use the [plugnmeet-install](https://github.com/mynaparrot/plugNmeet-install) script.

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
