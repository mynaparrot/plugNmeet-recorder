# plugNmeet-recorder

The plugNmeet-recorder can be used to record sessions as well as RTMP broadcasts. This software consumes CPU power, so
it's best to run it on a different server from the one used for plugNmeet or livekit. However, if you are not recording
frequently, you can continue to use the same server. In this scenario,
the [plugnmeet-install](https://github.com/mynaparrot/plugNmeet-install) script is recommended for installation.

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

1) To download the latest version based on the type of your OS architecture, check [release page](https://github.com/mynaparrot/plugNmeet-recorder/releases) or can use [plugnmeet-recorder](https://hub.docker.com/r/mynaparrot/plugnmeet-recorder) docker image.
2) Unzip the zip file and navigate to the directory from the terminal, then run.

```
cp config_sample.yaml config.yaml
```

3) Change the relevant information in the `config.yaml` file. Redis information should be the same as `plugnmeet-server`
   . The `main_path` value should be same as `recording_files_path` value of `plugnmeet-server`'s `config.yaml` file. If
   you intend to use NFS, ensure that both the recorder and the plugnmeet-server can access this directory. Otherwise,
   the user will be unable to download recordings.

4) Change `plugNmeet_info` & `nats_info` with correct information.

5) It's possible to install `plugNmeet-recorder` in multiple server. `plugNmeet-server` will choose the appropriate one
   based on availability. In that case change value of `id` inside `config.yaml` file. Make sure that value is unique,
   example: `node_01`, `node_02` ... You can also set the value of `max_limit` based on the server's capacity.

6) Start server `./plugnmeet-recorder-linux-[amd64|arm64]`

**Development**

1) Clone the project & navigate to the directory. 
2) Copy to rename the following files and update info:

```
cp config_sample.yaml config.yaml
cp docker-compose_sample.yaml docker-compose.yaml
```

3) Now start

```
docker compose build
docker compose up
```
