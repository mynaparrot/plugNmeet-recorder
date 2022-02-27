# plugNmeet-recorder

The plugNmeet-recorder can be used to record sessions as well as RTMP broadcasts. This software consumes CPU power, so
it's best to run it on a different server from the one used for plugNmeet or livekit. However, if you are not recording
frequently, you can continue to use the same server. In this scenario,
the [plugnmeet-install](https://github.com/mynaparrot/plugNmeet-install) script is recommended for installation.

**Requirements**

1) Nodejs
2) Google Chrome
3) xvfb
4) ffmpeg

**Install dependencies (Ubuntu)**

```
## nodejs
curl -fsSL https://deb.nodesource.com/setup_16.x | sudo -E bash -
sudo apt install -y nodejs

## Google Chrome
curl -sS -o - https://dl-ssl.google.com/linux/linux_signing_key.pub | sudo apt-key add
sudo echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" > /etc/apt/sources.list.d/google-chrome.list
sudo apt -y update
sudo apt -y install google-chrome-stable

## xvfb & ffmpeg
sudo apt install -y xvfb ffmpeg

## optional
sudo apt install -y fonts-noto fonts-liberation
```

**Install recorder**

1) To download the latest version, check [release page.](https://github.com/mynaparrot/plugNmeet-recorder/releases)
2) Unzip the recorder.zip file and navigate to the directory from the terminal, then run.

```
cp config_sample.yaml config.yaml
```

4) Change the relevant information in the `config.yaml` file. Redis information should be the same
   as `plugnmeet-server's`. The `main_path` value should be same as `recording_files_path` value of `plugnmeet-server`.
   If you intend to use NFS, ensure that both the recorder and the plugnmeet-server can access this directory.
   Otherwise, the user will be unable to download recordings. Also, ensure that nodejs has write permissions on the
   path.

5) Change `join_host` with correct format. It should be https url where you've installed `plugNmeet-server`
   with `plugNmeet-client`.

6) It's possible to install `plugNmeet-recorder` in multiple server. `plugNmeet-server` will choose appropriate one
   based on availability. In that case change value of `id` inside `config.yaml` file. Make sure that value is unique,
   example: `node_01`, `node_02` ... You can also set the value of `max_limit` based on the server's capacity.

7) Start server `npm start`

**Development**

1) Clone the project & navigate to the directory. Make sure you've nodejs install in your
   PC. https://nodejs.org/en/download/
2) Copy to rename this file:

```
cp config_sample.yaml config.yaml
```

3) Now start server

```
npm install
npm run dev
// to build
npm run build
```
