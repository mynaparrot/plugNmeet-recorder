# plugNmeet-recorder

The plugNmeet-recorder is capable of handling both session recording and RTMP streaming functionalities. Due to its high CPU utilization, it is strongly recommended to deploy the recorder on a server separate from the one running plugNmeet or LiveKit. However, in environments where recording is performed infrequently, co-locating the recorder on the same server is fine. In such cases, it is advisable to use the [plugnmeet-install](https://github.com/mynaparrot/plugNmeet-install) script to facilitate a streamlined and automated installation process.

**Requirements**

1) Linux system (**Recommend: Ubuntu**)
2) Google Chrome
3) pulseaudio
4) xvfb
5) ffmpeg

**Install dependencies (Ubuntu)**

```
## To insall pulseaudio, xvfb & ffmpeg
sudo apt install -y pulseaudio xvfb ffmpeg

## To start pulseaudio
pulseaudio -D --verbose --exit-idle-time=-1 --disallow-exit
# to start at boot
systemctl --user enable pulseaudio

## Google Chrome

curl -fsSL https://dl-ssl.google.com/linux/linux_signing_key.pub | sudo gpg --dearmor -o /usr/share/keyrings/googlechrome-linux-keyring.gpg
sudo echo "deb [arch=amd64 signed-by=/usr/share/keyrings/googlechrome-linux-keyring.gpg] http://dl.google.com/linux/chrome/deb/ stable main" >/etc/apt/sources.list.d/google-chrome.list

sudo apt -y update && apt -y install google-chrome-stable

## optional
sudo apt install -y fonts-noto fonts-liberation
```

**Install recorder**

1) To download the latest version suitable for your operating system architecture, visit the [release page](https://github.com/mynaparrot/plugNmeet-recorder/releases) or use the [plugnmeet-recorder](https://hub.docker.com/r/mynaparrot/plugnmeet-recorder) Docker image.
2) Extract the downloaded ZIP file, navigate to the extracted directory using the terminal, and run:

```
cp config_sample.yaml config.yaml
```

3) Edit the `config.yaml` file with the appropriate settings. The `nats_info` section must match the configuration used in the `plugnmeet-server`. The `main_path` value should be the same as the `recording_files_path` specified in the `plugnmeet-server's config.yaml`. If you're using NFS or another type of network-mounted storage, ensure both the `recorder` and the `plugnmeet-server` have access to it. Otherwise, users will not be able to download recordings. 

4) Update the `plugNmeet_info` and `nats_info` sections with the correct values.

5) You can deploy `plugnmeet-recorder` on multiple servers. The `plugnmeet-server` will automatically select an available recorder based on load and availability. In such cases, make sure to assign a unique id in each `config.yaml` file (e.g., `node_01`, `node_02`, etc.). You can also adjust the `max_limit` value according to the server’s capacity.

6) Start the recorder using the appropriate binary: `./plugnmeet-recorder-linux-[amd64|arm64]`

**Development**

1) Clone the project & navigate to the directory. 
2) Copy to rename the following files and update info:

```
cp config_sample.yaml config.yaml
cp docker-compose_sample.yaml docker-compose.yaml
```

3) Start the development environment

```
docker compose build
docker compose up
```
